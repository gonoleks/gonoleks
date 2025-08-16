package gonoleks

import (
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func createTestRouter() *router {
	return &router{
		trees: make(map[string]*node),
		pool: sync.Pool{
			New: func() any { return new(Context) },
		},
		app: &Gonoleks{
			enableLogging: false,
			Options: Options{
				MaxRequestURLLength: 2048,
			},
		},
		globalMiddleware: make(handlersChain, 0),
	}
}

func createTestRequestCtx(method, path string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(path)
	return ctx
}

func TestRouterBasics(t *testing.T) {
	r := createTestRouter()
	handler := func(c *Context) {}

	// Test registering valid routes
	r.handle(MethodGet, "/test", handlersChain{handler})
	assert.NotNil(t, r.trees[MethodGet], "GET tree should be created")

	// Test route handling
	ctx := &Context{paramValues: make(map[string]string)}
	assert.True(t, r.handleRoute(MethodGet, "/test", ctx), "Registered route should be handled")
	assert.False(t, r.handleRoute(MethodGet, "/nonexistent", ctx), "Non-existing route should not be handled")
	assert.False(t, r.handleRoute(MethodPost, "/test", ctx), "Route with different method should not be handled")

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

	// Test allowed methods
	r.handle(MethodPost, "/test", handlersChain{handler})
	r.handle(MethodPut, "/test", handlersChain{handler})
	allowed := r.allowed(MethodDelete, "/test", ctx)
	assert.Contains(t, allowed, MethodGet)
	assert.Contains(t, allowed, MethodPost)
	assert.Contains(t, allowed, MethodPut)
	assert.NotContains(t, allowed, MethodDelete)

	// Test SetNoRoute
	r.SetNoRoute(handlersChain{handler})
	assert.Equal(t, 1, len(r.noRoute), "No route handlers should be set")
}

func TestRouterContextManagement(t *testing.T) {
	r := createTestRouter()
	fctx := createTestRequestCtx(MethodGet, "/test")

	// Test context acquisition and release
	ctx := r.acquireCtx(fctx)
	assert.NotNil(t, ctx, "Acquired context should not be nil")
	assert.Equal(t, fctx, ctx.requestCtx, "Context should reference the request context")
	assert.Equal(t, -1, ctx.index, "Context index should be initialized to -1")
	assert.NotNil(t, ctx.paramValues, "Context paramValues should be initialized")

	// Add data and release
	ctx.paramValues["id"] = "123"
	ctx.handlers = handlersChain{func(c *Context) {}}
	ctx.index = 0
	r.releaseCtx(ctx)

	// Verify reset
	ctx2 := r.acquireCtx(fctx)
	assert.Empty(t, ctx2.paramValues, "Context paramValues should be cleared")
	assert.Empty(t, ctx2.handlers, "Context handlers should be empty")
	assert.Equal(t, -1, ctx2.index, "Context index should be reset to -1")
}

func TestRouterRequestHandling(t *testing.T) {
	r := createTestRouter()
	r.app.HandleMethodNotAllowed = true
	r.app.Use(Recovery())

	// Test basic request handling
	handlerCalled := false
	r.handle(MethodGet, "/test", handlersChain{func(c *Context) {
		handlerCalled = true
		c.Status(StatusOK)
	}})

	fctx := createTestRequestCtx(MethodGet, "/test")
	r.Handler(fctx)
	assert.True(t, handlerCalled, "Handler should be called")
	assert.Equal(t, StatusOK, fctx.Response.StatusCode(), "Status code should be 200")

	// Test non-existent route
	fctx = createTestRequestCtx(MethodGet, "/nonexistent")
	r.Handler(fctx)
	assert.Equal(t, StatusNotFound, fctx.Response.StatusCode(), "Status code should be 404")

	// Test method not allowed
	fctx = createTestRequestCtx(MethodPost, "/test")
	r.Handler(fctx)
	assert.Equal(t, StatusMethodNotAllowed, fctx.Response.StatusCode(), "Status code should be 405")

	// Test OPTIONS request
	fctx = createTestRequestCtx(MethodOptions, "/test")
	r.Handler(fctx)
	allowHeader := string(fctx.Response.Header.Peek(HeaderAllow))
	assert.Contains(t, allowHeader, MethodGet)

	// Test case insensitive routing
	r.app.CaseInSensitive = true
	r.handle(MethodGet, "/case", handlersChain{func(c *Context) {
		c.Status(StatusOK)
	}})
	fctx = createTestRequestCtx(MethodGet, "/CASE")
	r.Handler(fctx)
	assert.Equal(t, StatusOK, fctx.Response.StatusCode(), "Case insensitive routing should work")
}

func TestRouterParameters(t *testing.T) {
	r := createTestRouter()

	// Test simple parameters
	paramValue := ""
	r.handle(MethodGet, "/users/:id", handlersChain{func(c *Context) {
		paramValue = c.Param("id")
		c.Status(StatusOK)
	}})

	fctx := createTestRequestCtx(MethodGet, "/users/123")
	r.Handler(fctx)
	assert.Equal(t, "123", paramValue, "Parameter value should be extracted")

	// Test compound parameters
	var fileParam, extParam string
	r.handle(MethodGet, "/download/:file.:ext", handlersChain{func(c *Context) {
		fileParam = c.Param("file")
		extParam = c.Param("ext")
		c.Status(StatusOK)
	}})

	fctx = createTestRequestCtx(MethodGet, "/download/report.pdf")
	r.Handler(fctx)
	assert.Equal(t, "report", fileParam, "File parameter should be extracted")
	assert.Equal(t, "pdf", extParam, "Extension parameter should be extracted")

	// Test dash-separated parameters
	var fromParam, toParam string
	r.handle(MethodGet, "/range/:from-:to", handlersChain{func(c *Context) {
		fromParam = c.Param("from")
		toParam = c.Param("to")
		c.Status(StatusOK)
	}})

	fctx = createTestRequestCtx(MethodGet, "/range/100-200")
	r.Handler(fctx)
	assert.Equal(t, "100", fromParam, "From parameter should be extracted")
	assert.Equal(t, "200", toParam, "To parameter should be extracted")
}

func TestFastRouter(t *testing.T) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Test FastRouter initialization
	assert.NotNil(t, fr, "FastRouter should not be nil")
	assert.NotNil(t, fr.routeHashes, "routeHashes should be initialized")
	assert.NotNil(t, fr.ultraCache, "ultraCache should be initialized")

	// Test context pool
	ctx := fr.GetContext()
	assert.NotNil(t, ctx, "Context from pool should not be nil")
	assert.Equal(t, -1, ctx.index, "Context index should be -1")
	assert.Equal(t, 16, cap(ctx.handlers), "handlers should have capacity of 16")

	// Test adding and looking up routes
	fr.AddRoute(MethodGet, "/test", handlers)
	result, found := fr.FastLookup(MethodGet, "/test")
	assert.True(t, found, "Existing route should be found")
	assert.Equal(t, handlers, result, "Handlers should match")

	// Test non-existent route
	result, found = fr.FastLookup(MethodGet, "/nonexistent")
	assert.False(t, found, "Non-existent route should not be found")
	assert.Nil(t, result, "Result should be nil for non-existent route")

	// Test UltraFastLookup
	method := MethodGet
	path := "/test"
	methodPtr := unsafe.Pointer(unsafe.StringData(method))
	pathPtr := unsafe.Pointer(unsafe.StringData(path))
	result, found = fr.UltraFastLookup(methodPtr, pathPtr, len(method), len(path))
	assert.True(t, found, "UltraFastLookup should find the route")
	assert.Equal(t, handlers, result, "Handlers should match")

	// Test context pool reset
	ctx.index = 5
	ctx.fullPath = "/test/path"
	ctx.paramValues["id"] = "123"
	fr.PutContext(ctx)
	assert.Equal(t, -1, ctx.index, "Index should be reset")
	assert.Empty(t, ctx.fullPath, "fullPath should be reset")
	assert.Empty(t, ctx.paramValues, "paramValues should be cleared")
}

func TestRouterPerformance(t *testing.T) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Test hash function consistency
	str1 := "GET/test"
	str2 := "GET/test"
	str3 := "POST/test"
	hash1 := ultraFastStringHash(str1)
	hash2 := ultraFastStringHash(str2)
	hash3 := ultraFastStringHash(str3)
	assert.Equal(t, hash1, hash2, "Same data should produce same hash")
	assert.NotEqual(t, hash1, hash3, "Different data should produce different hash")

	// Test cache collision handling
	for i := range 100 {
		path := "/route" + string(rune(i))
		fr.AddRoute(MethodGet, path, handlers)
	}

	// Verify all routes can be found
	for i := range 100 {
		path := "/route" + string(rune(i))
		result, found := fr.FastLookup(MethodGet, path)
		assert.True(t, found, "Route should be found")
		assert.NotNil(t, result, "Handlers should not be nil")
	}

	// Test warmup cache
	routes := []string{"/api/v1/users", "/api/v1/posts"}
	assert.NotPanics(t, func() {
		fr.WarmupCache(routes)
	}, "WarmupCache should not panic")

	// Test concurrent access
	done := make(chan bool, 5)
	for i := range 5 {
		go func(routeNum int) {
			defer func() { done <- true }()
			path := "/route" + string(rune(routeNum))
			for range 50 {
				result, found := fr.FastLookup(MethodGet, path)
				assert.True(t, found, "Route should be found in concurrent access")
				assert.Equal(t, handlers, result, "Handlers should match")
			}
		}(i)
	}

	// Wait for completion
	for range 5 {
		<-done
	}
}
