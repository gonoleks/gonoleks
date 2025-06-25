package gonoleks

import (
	"sync"
	"testing"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func createTestRouter() *router {
	cache, _ := lru.New[string, any](10)
	return &router{
		trees:   make(map[string]*node),
		cache:   cache,
		options: &Options{MaxRequestURLLength: 2048},
		pool: sync.Pool{
			New: func() any { return new(Context) },
		},
	}
}

func createTestRequestCtx(method, path string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(path)
	return ctx
}

func TestRouterHandle(t *testing.T) {
	r := createTestRouter()

	// Test registering a valid route
	handler := func(c *Context) {}
	r.handle(MethodGet, "/test", handlersChain{handler})

	// Verify the route was registered
	assert.NotNil(t, r.trees[MethodGet], "GET tree should be created")

	// Test registering a route with the same path (should not panic)
	assert.NotPanics(t, func() {
		r.handle(MethodGet, "/test", handlersChain{handler})
	})

	// Test invalid inputs
	assert.Panics(t, func() {
		r.handle("", "/test", handlersChain{handler})
	}, "Empty method should panic")

	assert.Panics(t, func() {
		r.handle(MethodGet, "", handlersChain{handler})
	}, "Empty path should panic")

	assert.Panics(t, func() {
		r.handle(MethodGet, "test", handlersChain{handler})
	}, "Path without leading slash should panic")

	assert.Panics(t, func() {
		r.handle(MethodGet, "/test", handlersChain{})
	}, "Empty handlers should panic")
}

func TestRouterRouteExists(t *testing.T) {
	r := createTestRouter()
	handler := func(c *Context) {}

	// Register a route
	r.handle(MethodGet, "/test", handlersChain{handler})

	// Test existing route
	assert.True(t, r.routeExists(MethodGet, "/test"), "Route should exist")

	// Test non-existing route
	assert.False(t, r.routeExists(MethodGet, "/nonexistent"), "Route should not exist")

	// Test non-existing method
	assert.False(t, r.routeExists(MethodPost, "/test"), "Route with different method should not exist")
}

func TestRouterAllowed(t *testing.T) {
	r := createTestRouter()
	handler := func(c *Context) {}

	// Register routes with different methods
	r.handle(MethodGet, "/test", handlersChain{handler})
	r.handle(MethodPost, "/test", handlersChain{handler})
	r.handle(MethodPut, "/test", handlersChain{handler})

	ctx := &Context{
		paramValues: make(map[string]string),
	}

	// Test allowed methods for a path
	allowed := r.allowed(MethodDelete, "/test", ctx)
	assert.Contains(t, allowed, MethodGet)
	assert.Contains(t, allowed, MethodPost)
	assert.Contains(t, allowed, MethodPut)
	assert.Contains(t, allowed, MethodOptions)
	assert.NotContains(t, allowed, MethodDelete)

	// Test wildcard path
	allowed = r.allowed(MethodDelete, "*", ctx)
	assert.Contains(t, allowed, MethodGet)
	assert.Contains(t, allowed, MethodPost)
	assert.Contains(t, allowed, MethodPut)
}

func TestRouterAcquireReleaseCtx(t *testing.T) {
	r := createTestRouter()
	fctx := createTestRequestCtx(MethodGet, "/test")

	// Acquire context
	ctx := r.acquireCtx(fctx)
	assert.NotNil(t, ctx, "Acquired context should not be nil")
	assert.Equal(t, fctx, ctx.requestCtx, "Context should reference the request context")
	assert.Equal(t, -1, ctx.index, "Context index should be initialized to -1")
	assert.Empty(t, ctx.fullPath, "Context fullPath should be empty")
	assert.NotNil(t, ctx.paramValues, "Context paramValues should be initialized")

	// Add some data to the context
	ctx.paramValues["id"] = "123"
	ctx.handlers = handlersChain{func(c *Context) {}}
	ctx.index = 0

	// Release context
	r.releaseCtx(ctx)

	// Acquire again to verify it was reset
	ctx2 := r.acquireCtx(fctx)
	assert.Empty(t, ctx2.paramValues, "Context paramValues should be cleared")
	assert.Empty(t, ctx2.handlers, "Context handlers should be empty")
	assert.Equal(t, -1, ctx2.index, "Context index should be reset to -1")
}

func TestRouterHandleCache(t *testing.T) {
	r := createTestRouter()
	handler := func(c *Context) {
		c.Status(StatusOK)
	}

	// Register a route
	r.handle(MethodGet, "/test", handlersChain{handler})

	// Create a request context
	fctx := createTestRequestCtx(MethodGet, "/test")
	ctx := r.acquireCtx(fctx)

	// First request should not be cached
	assert.False(t, r.handleCache(MethodGet, "/test", ctx), "First request should not be from cache")

	// Process the route to populate the cache
	assert.True(t, r.handleRoute(MethodGet, "/test", ctx), "Route should be handled")

	// Reset context
	r.releaseCtx(ctx)
	ctx = r.acquireCtx(fctx)

	// Second request should be cached
	assert.True(t, r.handleCache(MethodGet, "/test", ctx), "Second request should be from cache")

	// Test with disabled caching
	r.options.DisableCaching = true
	r.releaseCtx(ctx)
	ctx = r.acquireCtx(fctx)
	assert.False(t, r.handleCache(MethodGet, "/test", ctx), "Request should not be cached when caching is disabled")

	// Test with non-cacheable method
	r.options.DisableCaching = false
	r.releaseCtx(ctx)
	ctx = r.acquireCtx(fctx)
	assert.False(t, r.handleCache(MethodTrace, "/test", ctx), "TRACE method should not be cached")
}

func TestRouterHandleRoute(t *testing.T) {
	r := createTestRouter()
	handlerCalled := false

	handler := func(c *Context) {
		handlerCalled = true
		c.Status(StatusOK)
	}

	// Register a route
	r.handle(MethodGet, "/test", handlersChain{handler})

	// Create a request context
	fctx := createTestRequestCtx(MethodGet, "/test")
	ctx := r.acquireCtx(fctx)

	// Test handling a route
	assert.True(t, r.handleRoute(MethodGet, "/test", ctx), "Route should be handled")
	assert.True(t, handlerCalled, "Handler should be called")

	// Test handling a non-existent route
	handlerCalled = false
	assert.False(t, r.handleRoute(MethodGet, "/nonexistent", ctx), "Non-existent route should not be handled")
	assert.False(t, handlerCalled, "Handler should not be called")

	// Test handling a route with a different method
	handlerCalled = false
	assert.False(t, r.handleRoute(MethodPost, "/test", ctx), "Route with different method should not be handled")
	assert.False(t, handlerCalled, "Handler should not be called")
}

func TestRouterHandleOptions(t *testing.T) {
	r := createTestRouter()
	handler := func(c *Context) {}

	// Register routes with different methods
	r.handle(MethodGet, "/test", handlersChain{handler})
	r.handle(MethodPost, "/test", handlersChain{handler})

	// Create a request context
	fctx := createTestRequestCtx(MethodOptions, "/test")
	ctx := r.acquireCtx(fctx)

	// Test handling OPTIONS request
	assert.True(t, r.handleOptions(fctx, MethodOptions, "/test", ctx), "OPTIONS request should be handled")

	// Verify the Allow header was set
	allowHeader := string(fctx.Response.Header.Peek("Allow"))
	assert.Contains(t, allowHeader, MethodGet)
	assert.Contains(t, allowHeader, MethodPost)
	assert.Contains(t, allowHeader, MethodOptions)

	// Test handling OPTIONS request for a non-existent path
	fctx = createTestRequestCtx(MethodOptions, "/nonexistent")
	assert.False(t, r.handleOptions(fctx, MethodOptions, "/nonexistent", ctx), "OPTIONS request for non-existent path should not be handled")
}

func TestRouterHandleMethodNotAllowed(t *testing.T) {
	r := createTestRouter()
	handler := func(c *Context) {}

	// Register a route
	r.handle(MethodGet, "/test", handlersChain{handler})

	// Create a request context
	fctx := createTestRequestCtx(MethodPost, "/test")
	ctx := r.acquireCtx(fctx)

	// Test handling method not allowed
	assert.True(t, r.handleMethodNotAllowed(fctx, MethodPost, "/test", ctx), "Method not allowed should be handled")

	// Verify the response
	assert.Equal(t, StatusMethodNotAllowed, fctx.Response.StatusCode(), "Status code should be 405")
	assert.Equal(t, MIMETextPlainCharsetUTF8, string(fctx.Response.Header.ContentType()), "Content type should be text/plain")
	assert.Equal(t, fasthttp.StatusMessage(StatusMethodNotAllowed), string(fctx.Response.Body()), "Body should contain error message")

	// Verify the Allow header was set
	allowHeader := string(fctx.Response.Header.Peek("Allow"))
	assert.Contains(t, allowHeader, MethodGet)
	assert.Contains(t, allowHeader, MethodOptions)

	// Test handling method not allowed for a non-existent path
	fctx = createTestRequestCtx(MethodPost, "/nonexistent")
	assert.False(t, r.handleMethodNotAllowed(fctx, MethodPost, "/nonexistent", ctx), "Method not allowed for non-existent path should not be handled")
}

func TestRouterHandleNoRoute(t *testing.T) {
	r := createTestRouter()

	// Create a request context
	fctx := createTestRequestCtx(MethodGet, "/nonexistent")
	ctx := r.acquireCtx(fctx)

	// Test handling no route without custom handlers
	assert.False(t, r.handleNoRoute(ctx), "No route without custom handlers should not be handled")

	// Set custom no route handlers
	handlerCalled := false
	r.SetNoRoute(handlersChain{func(c *Context) {
		handlerCalled = true
		c.Status(StatusNotFound)
	}})

	// Test handling no route with custom handlers
	assert.True(t, r.handleNoRoute(ctx), "No route with custom handlers should be handled")
	assert.True(t, handlerCalled, "No route handler should be called")
}

func TestRouterSetNoRoute(t *testing.T) {
	r := createTestRouter()
	handler := func(c *Context) {}

	// Set no route handlers
	r.SetNoRoute(handlersChain{handler})

	// Verify handlers were set
	assert.Equal(t, 1, len(r.noRoute), "No route handlers should be set")
}

func TestRouterHandler(t *testing.T) {
	r := createTestRouter()

	// Enable all handler options
	r.options.HandleOPTIONS = true
	r.options.HandleMethodNotAllowed = true
	r.options.AutoRecover = true

	// Register routes
	handlerCalled := false
	r.handle(MethodGet, "/test", handlersChain{func(c *Context) {
		handlerCalled = true
		c.Status(StatusOK)
	}})

	// Test handling a valid request
	fctx := createTestRequestCtx(MethodGet, "/test")
	r.Handler(fctx)
	assert.True(t, handlerCalled, "Handler should be called")
	assert.Equal(t, StatusOK, fctx.Response.StatusCode(), "Status code should be 200")

	// Test handling a non-existent route
	fctx = createTestRequestCtx(MethodGet, "/nonexistent")
	r.Handler(fctx)
	assert.Equal(t, StatusNotFound, fctx.Response.StatusCode(), "Status code should be 404")

	// Test handling an OPTIONS request
	fctx = createTestRequestCtx(MethodOptions, "/test")
	r.Handler(fctx)
	allowHeader := string(fctx.Response.Header.Peek("Allow"))
	assert.Contains(t, allowHeader, MethodGet)

	// Test handling a method not allowed request
	fctx = createTestRequestCtx(MethodPost, "/test")
	r.Handler(fctx)
	assert.Equal(t, StatusMethodNotAllowed, fctx.Response.StatusCode(), "Status code should be 405")

	// Test case insensitive routing
	r.options.CaseInSensitive = true
	r.handle(MethodGet, "/case", handlersChain{func(c *Context) {
		c.Status(StatusOK)
	}})

	fctx = createTestRequestCtx(MethodGet, "/CASE")
	r.Handler(fctx)
	assert.Equal(t, StatusOK, fctx.Response.StatusCode(), "Status code should be 200 with case insensitive routing")
}

func TestRouterRecoverFromPanic(t *testing.T) {
	r := createTestRouter()

	// Register a route that panics
	r.handle(MethodGet, "/panic", handlersChain{func(c *Context) {
		panic("test panic")
	}})

	// Enable auto recovery
	r.options.AutoRecover = true

	// Test handling a request that causes a panic
	fctx := createTestRequestCtx(MethodGet, "/panic")
	assert.NotPanics(t, func() {
		r.Handler(fctx)
	}, "Handler should recover from panic")

	assert.Equal(t, StatusInternalServerError, fctx.Response.StatusCode(), "Status code should be 500 after panic")
}

func TestRouterWithCaching(t *testing.T) {
	cache, _ := lru.New[string, any](10)
	r := &router{
		trees:   make(map[string]*node),
		cache:   cache,
		options: &Options{MaxRequestURLLength: 2048},
		pool: sync.Pool{
			New: func() any { return new(Context) },
		},
	}

	// Register a route
	handlerCallCount := 0
	r.handle(MethodGet, "/cached", handlersChain{func(c *Context) {
		handlerCallCount++
		c.Status(StatusOK)
	}})

	// First request should not be cached
	fctx1 := createTestRequestCtx(MethodGet, "/cached")
	r.Handler(fctx1)
	assert.Equal(t, 1, handlerCallCount, "Handler should be called once")

	// Second request should be served from cache
	fctx2 := createTestRequestCtx(MethodGet, "/cached")
	r.Handler(fctx2)
	assert.Equal(t, 2, handlerCallCount, "Handler should be called twice")

	// Disable caching
	r.options.DisableCaching = true

	// Third request should not be served from cache
	fctx3 := createTestRequestCtx(MethodGet, "/cached")
	r.Handler(fctx3)
	assert.Equal(t, 3, handlerCallCount, "Handler should be called three times")
}

func TestRouterWithParameters(t *testing.T) {
	r := createTestRouter()

	// Register routes with parameters
	paramValue := ""
	r.handle(MethodGet, "/users/:id", handlersChain{func(c *Context) {
		paramValue = c.Param("id")
		c.Status(StatusOK)
	}})

	// Test handling a request with a parameter
	fctx := createTestRequestCtx(MethodGet, "/users/123")
	r.Handler(fctx)
	assert.Equal(t, "123", paramValue, "Parameter value should be extracted")
	assert.Equal(t, StatusOK, fctx.Response.StatusCode(), "Status code should be 200")

	// Test handling a request with a different parameter value
	fctx = createTestRequestCtx(MethodGet, "/users/456")
	r.Handler(fctx)
	assert.Equal(t, "456", paramValue, "Parameter value should be updated")
}

func TestRouterWithCompoundParameters(t *testing.T) {
	r := createTestRouter()

	// Register a route with compound parameters
	var fileParam, extParam string
	r.handle(MethodGet, "/download/:file.:ext", handlersChain{func(c *Context) {
		fileParam = c.Param("file")
		extParam = c.Param("ext")
		c.Status(StatusOK)
	}})

	// Test handling a request with compound parameters
	fctx := createTestRequestCtx(MethodGet, "/download/report.pdf")
	r.Handler(fctx)
	assert.Equal(t, "report", fileParam, "File parameter should be extracted")
	assert.Equal(t, "pdf", extParam, "Extension parameter should be extracted")

	// Register a route with dash-separated parameters
	var fromParam, toParam string
	r.handle(MethodGet, "/range/:from-:to", handlersChain{func(c *Context) {
		fromParam = c.Param("from")
		toParam = c.Param("to")
		c.Status(StatusOK)
	}})

	// Test handling a request with dash-separated parameters
	fctx = createTestRequestCtx(MethodGet, "/range/100-200")
	r.Handler(fctx)
	assert.Equal(t, "100", fromParam, "From parameter should be extracted")
	assert.Equal(t, "200", toParam, "To parameter should be extracted")
}

func TestRouterWithLogging(t *testing.T) {
	r := createTestRouter()

	// Enable logging
	r.options.DisableLogging = false

	// Register a route
	r.handle(MethodGet, "/log", handlersChain{func(c *Context) {
		c.Status(StatusOK)
	}})

	// Test handling a request with logging enabled
	fctx := createTestRequestCtx(MethodGet, "/log")

	// This is a bit tricky to test since we can't easily capture log output
	// We're just ensuring it doesn't panic
	assert.NotPanics(t, func() {
		r.Handler(fctx)
	}, "Handler with logging should not panic")

	// Disable logging
	r.options.DisableLogging = true

	// Test handling a request with logging disabled
	fctx = createTestRequestCtx(MethodGet, "/log")
	assert.NotPanics(t, func() {
		r.Handler(fctx)
	}, "Handler without logging should not panic")
}
