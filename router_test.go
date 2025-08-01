package gonoleks

import (
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func createTestRouter() *router {
	return &router{
		trees: make(map[string]*node),
		pool: sync.Pool{
			New: func() any { return new(Context) },
		},
		// Add missing app reference to prevent nil pointer dereference
		app: &Gonoleks{
			enableLogging: false, // Set to false for tests
			Options: Options{
				MaxRequestURLLength: 2048,
			},
		},
		globalMiddleware: make(handlersChain, 0), // Initialize to prevent nil slice issues
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
	// Execute the handlers to actually call them
	ctx.Next()
	assert.True(t, handlerCalled, "Handler should be called")

	// Test handling a non-existent route
	handlerCalled = false
	ctx = r.acquireCtx(fctx) // Reset context
	assert.False(t, r.handleRoute(MethodGet, "/nonexistent", ctx), "Non-existent route should not be handled")
	assert.False(t, handlerCalled, "Handler should not be called")

	// Test handling a route with a different method
	handlerCalled = false
	ctx = r.acquireCtx(fctx) // Reset context
	assert.False(t, r.handleRoute(MethodPost, "/test", ctx), "Route with different method should not be handled")
	assert.False(t, handlerCalled, "Handler should not be called")
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
	allowHeader := string(fctx.Response.Header.Peek(HeaderAllow))
	assert.Contains(t, allowHeader, MethodGet)
	assert.Contains(t, allowHeader, MethodOptions)

	// Test handling method not allowed for a non-existent path
	fctx = createTestRequestCtx(MethodPost, "/nonexistent")
	assert.False(t, r.handleMethodNotAllowed(fctx, MethodPost, "/nonexistent", ctx), "Method not allowed for non-existent path should not be handled")
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
	r.app.HandleMethodNotAllowed = true
	r.app.Use(Recovery())

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
	allowHeader := string(fctx.Response.Header.Peek(HeaderAllow))
	assert.Contains(t, allowHeader, MethodGet)

	// Test handling a method not allowed request
	fctx = createTestRequestCtx(MethodPost, "/test")
	r.Handler(fctx)
	assert.Equal(t, StatusMethodNotAllowed, fctx.Response.StatusCode(), "Status code should be 405")

	// Test case insensitive routing
	r.app.CaseInSensitive = true
	r.handle(MethodGet, "/case", handlersChain{func(c *Context) {
		c.Status(StatusOK)
	}})

	fctx = createTestRequestCtx(MethodGet, "/CASE")
	r.Handler(fctx)
	assert.Equal(t, StatusOK, fctx.Response.StatusCode(), "Status code should be 200 with case insensitive routing")
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

	// Test handling a request with logging disabled
	fctx = createTestRequestCtx(MethodGet, "/log")
	assert.NotPanics(t, func() {
		r.Handler(fctx)
	}, "Handler without logging should not panic")
}

func TestNewFastRouter(t *testing.T) {
	fr := NewFastRouter()

	// Test that FastRouter is properly initialized
	assert.NotNil(t, fr, "FastRouter should not be nil")
	assert.NotNil(t, fr.routeMap, "routeMap should be initialized")
	assert.NotNil(t, fr.routeMap, "routeMap should be initialized and not nil")

	// Test that context pool is working
	ctx := fr.GetContext()
	assert.NotNil(t, ctx, "Context from pool should not be nil")
	assert.NotNil(t, ctx.paramValues, "Context paramValues should be initialized")
	assert.Equal(t, 0, len(ctx.paramValues), "paramValues should have initial length of 0")
	assert.Equal(t, 16, cap(ctx.handlers), "handlers should have capacity of 16")
	assert.Equal(t, -1, ctx.index, "Context index should be -1")

	// Test that pool is pre-warmed
	fr.PutContext(ctx)
	ctx2 := fr.GetContext()
	assert.NotNil(t, ctx2, "Second context from pool should not be nil")
}

func TestRouter_AddRoute(t *testing.T) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Test adding a route
	fr.AddRoute(MethodGet, "/test", handlers)

	// Verify route was added to all caches
	routeKey := "GET/test"
	hash := fastHash([]byte(routeKey))

	// Check common routes cache
	commonIndex := hash & 1023
	assert.Equal(t, hash, fr.commonRoutes[commonIndex].key, "Route should be in common routes cache")
	assert.Equal(t, handlers, fr.commonRoutes[commonIndex].handlers, "Handlers should match in common routes")

	// Check route cache
	cacheIndex := hash & 255
	assert.NotEqual(t, uintptr(0), fr.routeCache[cacheIndex].key, "Route should be in route cache")
	assert.Equal(t, handlers, fr.routeCache[cacheIndex].handlers, "Handlers should match in route cache")

	// Test string interning
	fr.AddRoute(MethodGet, "/test", handlers) // Add same route again
	interned, ok := fr.stringPool.Load(routeKey)
	assert.True(t, ok, "Route key should be interned")
	assert.Equal(t, routeKey, interned.(string), "Interned string should match")
}

func TestRouter_FastLookup(t *testing.T) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Test lookup for non-existent route
	result, found := fr.FastLookup(MethodGet, "/nonexistent")
	assert.False(t, found, "Non-existent route should not be found")
	assert.Nil(t, result, "Result should be nil for non-existent route")

	// Add a route and test lookup
	fr.AddRoute(MethodGet, "/test", handlers)
	result, found = fr.FastLookup(MethodGet, "/test")
	assert.True(t, found, "Existing route should be found")
	assert.Equal(t, handlers, result, "Handlers should match")

	// Test different methods
	fr.AddRoute(MethodPost, "/test", handlers)
	result, found = fr.FastLookup(MethodPost, "/test")
	assert.True(t, found, "POST route should be found")
	assert.Equal(t, handlers, result, "POST handlers should match")

	// Test case sensitivity
	_, found = fr.FastLookup(strings.ToLower(MethodGet), "/test")
	assert.False(t, found, "Lowercase method should not match")

	_, found = fr.FastLookup(MethodGet, "/Test")
	assert.False(t, found, "Different case path should not match")
}

func TestRouter_UltraFastLookup(t *testing.T) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Add a route
	fr.AddRoute(MethodGet, "/ultra", handlers)

	// Test UltraFastLookup with unsafe pointers
	method := MethodGet
	path := "/ultra"
	methodPtr := unsafe.Pointer(unsafe.StringData(method))
	pathPtr := unsafe.Pointer(unsafe.StringData(path))

	result, found := fr.UltraFastLookup(methodPtr, pathPtr, len(method), len(path))
	assert.True(t, found, "UltraFastLookup should find the route")
	assert.Equal(t, handlers, result, "Handlers should match")

	// Test with non-existent route
	method2 := MethodPost
	path2 := "/nonexistent"
	methodPtr2 := unsafe.Pointer(unsafe.StringData(method2))
	pathPtr2 := unsafe.Pointer(unsafe.StringData(path2))

	result, found = fr.UltraFastLookup(methodPtr2, pathPtr2, len(method2), len(path2))
	assert.False(t, found, "UltraFastLookup should not find non-existent route")
	assert.Nil(t, result, "Result should be nil for non-existent route")
}

func TestRouter_ContextPool(t *testing.T) {
	fr := NewFastRouter()

	// Test getting context from pool
	ctx := fr.GetContext()
	require.NotNil(t, ctx, "Context should not be nil")
	assert.Equal(t, -1, ctx.index, "Initial index should be -1")
	assert.Empty(t, ctx.fullPath, "Initial fullPath should be empty")
	assert.Empty(t, ctx.handlers, "Initial handlers should be empty")
	assert.NotNil(t, ctx.paramValues, "paramValues should be initialized")

	// Modify context
	ctx.index = 5
	ctx.fullPath = "/test/path"
	ctx.handlers = handlersChain{func(c *Context) {}}
	ctx.paramValues["id"] = "123"
	ctx.paramValues["name"] = "test"

	// Put context back to pool
	fr.PutContext(ctx)

	// Verify context was reset
	assert.Equal(t, -1, ctx.index, "Index should be reset to -1")
	assert.Empty(t, ctx.fullPath, "fullPath should be reset")
	assert.Empty(t, ctx.handlers, "handlers should be reset")
	assert.Empty(t, ctx.paramValues, "paramValues should be cleared")

	// Get another context and verify it's clean
	ctx2 := fr.GetContext()
	assert.Equal(t, -1, ctx2.index, "New context index should be -1")
	assert.Empty(t, ctx2.fullPath, "New context fullPath should be empty")
	assert.Empty(t, ctx2.handlers, "New context handlers should be empty")
	assert.Empty(t, ctx2.paramValues, "New context paramValues should be empty")
}

func TestRouter_WarmupCache(t *testing.T) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Add some routes
	fr.AddRoute(MethodGet, "/api/v1/users", handlers)
	fr.AddRoute(MethodPost, "/api/v1/users", handlers)
	fr.AddRoute(MethodGet, "/api/v1/posts", handlers)

	// Test warmup cache
	routes := []string{"/api/v1/users", "/api/v1/posts", "/api/v1/comments"}
	assert.NotPanics(t, func() {
		fr.WarmupCache(routes)
	}, "WarmupCache should not panic")

	// Verify routes are still accessible after warmup
	result, found := fr.FastLookup(MethodGet, "/api/v1/users")
	assert.True(t, found, "Route should still be found after warmup")
	assert.Equal(t, handlers, result, "Handlers should still match after warmup")
}

func TestFastHash(t *testing.T) {
	// Test hash function consistency
	data1 := []byte("GET/test")
	data2 := []byte("GET/test")
	data3 := []byte("POST/test")

	hash1 := fastHash(data1)
	hash2 := fastHash(data2)
	hash3 := fastHash(data3)

	assert.Equal(t, hash1, hash2, "Same data should produce same hash")
	assert.NotEqual(t, hash1, hash3, "Different data should produce different hash")

	// Test with empty data
	emptyHash := fastHash([]byte{})
	assert.NotEqual(t, uint32(0), emptyHash, "Empty data should still produce a hash")

	// Test hash distribution (basic check)
	hashes := make(map[uint32]bool)
	for i := 0; i < 1000; i++ {
		data := []byte("route" + string(rune(i)))
		hash := fastHash(data)
		hashes[hash] = true
	}
	// Should have good distribution (at least 90% unique hashes)
	assert.Greater(t, len(hashes), 900, "Hash function should have good distribution")
}

func TestStringToPointer(t *testing.T) {
	// Test pointer conversion consistency
	str1 := "test string"
	str2 := "test string"
	str3 := "different string"

	ptr1 := stringToPointer(str1)
	ptr2 := stringToPointer(str2)
	ptr3 := stringToPointer(str3)

	// Same string content should produce same pointer
	assert.Equal(t, ptr1, ptr2, "Same string content should produce same pointer")
	assert.NotEqual(t, ptr1, ptr3, "Different strings should produce different pointers")

	// Test with empty string - should return null pointer (0x0)
	emptyPtr := stringToPointer("")
	assert.Equal(t, uintptr(0), emptyPtr, "Empty string should return null pointer")

	// Test with non-empty strings should return valid pointers
	assert.NotEqual(t, uintptr(0), ptr1, "Non-empty string should return valid pointer")
	assert.NotEqual(t, uintptr(0), ptr2, "Non-empty string should return valid pointer")
	assert.NotEqual(t, uintptr(0), ptr3, "Non-empty string should return valid pointer")
}

func TestRouter_CacheCollisions(t *testing.T) {
	fr := NewFastRouter()
	handler1 := func(c *Context) { c.Status(StatusOK) }
	handler2 := func(c *Context) { c.Status(StatusCreated) }
	handlers1 := handlersChain{handler1}
	handlers2 := handlersChain{handler2}

	// Add many routes to test cache collision handling
	for i := 0; i < 300; i++ {
		method := MethodGet
		path := "/route" + string(rune(i))
		var handlers handlersChain
		if i%2 == 0 {
			handlers = handlers1
		} else {
			handlers = handlers2
		}
		fr.AddRoute(method, path, handlers)
	}

	// Verify all routes can still be found
	for i := 0; i < 300; i++ {
		path := "/route" + string(rune(i))
		result, found := fr.FastLookup(MethodGet, path)
		assert.True(t, found, "Route %s should be found", path)
		assert.NotNil(t, result, "Handlers should not be nil for route %s", path)
	}
}

func TestRouter_ConcurrentAccess(t *testing.T) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Add some initial routes
	for i := 0; i < 10; i++ {
		path := "/concurrent" + string(rune(i))
		fr.AddRoute(MethodGet, path, handlers)
	}

	// Test concurrent lookups
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(routeNum int) {
			defer func() { done <- true }()
			path := "/concurrent" + string(rune(routeNum))
			for j := 0; j < 100; j++ {
				result, found := fr.FastLookup(MethodGet, path)
				assert.True(t, found, "Route should be found in concurrent access")
				assert.Equal(t, handlers, result, "Handlers should match in concurrent access")
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkRouter_AddRoute(b *testing.B) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		path := "/bench" + string(rune(i%1000)) // Cycle through 1000 different paths
		fr.AddRoute(MethodGet, path, handlers)
	}
}

func BenchmarkRouter_FastLookup(b *testing.B) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Pre-populate with routes
	for i := 0; i < 1000; i++ {
		path := "/bench" + string(rune(i))
		fr.AddRoute(MethodGet, path, handlers)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		path := "/bench" + string(rune(i%1000))
		fr.FastLookup(MethodGet, path)
	}
}

func BenchmarkRouter_UltraFastLookup(b *testing.B) {
	fr := NewFastRouter()
	handler := func(c *Context) { c.Status(StatusOK) }
	handlers := handlersChain{handler}

	// Pre-populate with routes
	for i := 0; i < 1000; i++ {
		path := "/bench" + string(rune(i))
		fr.AddRoute(MethodGet, path, handlers)
	}

	method := MethodGet
	methodPtr := unsafe.Pointer(unsafe.StringData(method))
	methodLen := len(method)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		path := "/bench" + string(rune(i%1000))
		pathPtr := unsafe.Pointer(unsafe.StringData(path))
		pathLen := len(path)
		fr.UltraFastLookup(methodPtr, pathPtr, methodLen, pathLen)
	}
}

func BenchmarkRouter_ContextPool(b *testing.B) {
	fr := NewFastRouter()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := fr.GetContext()
		// Simulate some work
		ctx.index = i
		ctx.paramValues["test"] = "value"
		fr.PutContext(ctx)
	}
}
