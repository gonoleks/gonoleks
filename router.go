package gonoleks

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/valyala/fasthttp"
)

// FNV-1a hash constants
const (
	fnvOffsetBasis = uint64(14695981039346656037)
	fnvPrime       = uint64(1099511628211)
)

// FastRouter is an optimized router for static routes
// Uses pointer arithmetic and unsafe operations for performance
type FastRouter struct {
	// Route lookup using pointer-based keys for O(1) access
	routeMap map[uintptr]handlersChain

	// String interning pool for string reuse
	stringPool sync.Map

	// Pre-allocated context pool
	ctxPool sync.Pool

	// Cache-aligned route cache
	routeCache [256]routeCacheEntry

	// Hash table for common routes (powers of 2 for bit masking)
	commonRoutes [1024]commonRoute
}

// routeCacheEntry represents a cache-aligned route entry
type routeCacheEntry struct {
	key      uintptr
	handlers handlersChain
	_        [40]byte // Cache line padding to prevent false sharing
}

// commonRoute represents frequently accessed routes
type commonRoute struct {
	key      uint32
	handlers handlersChain
}

// router handles HTTP request routing
type router struct {
	trees            map[string]*node         // Route trees by HTTP method
	noRoute          handlersChain            // Handlers for 404 Not Found responses
	noMethod         handlersChain            // Handlers for 405 Method Not Allowed responses
	pool             sync.Pool                // Reused context objects
	app              *Gonoleks                // Reference to the gonoleks app instance
	getTree          *node                    // Lookup for GET HTTP method
	postTree         *node                    // Lookup for POST HTTP method
	putTree          *node                    // Lookup for PUT HTTP method
	staticRoutes     map[string]handlersChain // Static route cache for O(1) lookup
	fastRouter       *FastRouter              // Router for static routes
	globalMiddleware handlersChain            // Global middleware for all requests including errors
}

// acquireCtx gets a context from the pool and initializes it
// This reduces allocations by reusing context objects
//
//go:noinline
func (r *router) acquireCtx(fctx *fasthttp.RequestCtx) *Context {
	ctx := r.pool.Get().(*Context)

	// Reuse existing allocations without clearing
	if ctx.paramValues == nil {
		ctx.paramValues = make(map[string]string, 4)
	}
	// Clear map if it has entries
	if len(ctx.paramValues) > 0 {
		clear(ctx.paramValues)
	}

	// Reset slice without reallocation
	ctx.handlers = ctx.handlers[:0]
	ctx.requestCtx = fctx
	ctx.index = -1
	ctx.fullPath = ""

	return ctx
}

// releaseCtx returns a context to the pool after clearing its state
// This prevents memory leaks while allowing object reuse
//
//go:noinline
//go:nosplit
func (r *router) releaseCtx(ctx *Context) {
	// Reset: only clear handlers slice length, keep capacity
	ctx.handlers = ctx.handlers[:0]

	// Clear map if it has entries
	if len(ctx.paramValues) > 0 {
		clear(ctx.paramValues)
	}

	// Clear request context reference
	ctx.requestCtx = nil
	r.pool.Put(ctx)
}

// handle registers handler functions for a specific HTTP method and path
// It validates inputs and adds the route to the appropriate routing tree
func (r *router) handle(method, path string, handlers handlersChain) {
	if path == "" {
		panic("router.handle: path cannot be empty")
	} else if method == "" {
		panic("router.handle: HTTP method cannot be empty")
	} else if path[0] != '/' {
		panic("router.handle: path must begin with '/' character, got '" + path + "'")
	} else if len(handlers) == 0 {
		panic("router.handle: no handler functions provided for route '" + method + " " + path + "'")
	}

	// Initialize tree if it's empty
	if r.trees == nil {
		r.trees = make(map[string]*node)
	}
	if r.staticRoutes == nil {
		r.staticRoutes = make(map[string]handlersChain, 256)
	}
	if r.fastRouter == nil {
		r.fastRouter = NewFastRouter()
	}

	// Check if this is a static route (no parameters)
	if !strings.Contains(path, ":") && !strings.Contains(path, "*") {
		// Cache static routes for O(1) lookup
		routeKey := method + path
		r.staticRoutes[routeKey] = handlers
		r.fastRouter.AddRoute(method, path, handlers)
	}

	// Get root of method if it exists, otherwise create it
	root := r.trees[method]
	if root == nil {
		root = createRootNode()
		r.trees[method] = root

		// Update lookup trees for common methods
		switch method {
		case MethodGet:
			r.getTree = root
		case MethodPost:
			r.postTree = root
		case MethodPut:
			r.putTree = root
		}
	}

	// Check if route already exists
	if r.routeExists(method, path) {
		return
	}

	root.addRoute(path, handlers)
}

// routeExists checks if a route with the given method and path already exists
// Returns true if the route is found, false otherwise
func (r *router) routeExists(method, path string) bool {
	if root := r.trees[method]; root != nil {
		// Create a temporary context to check if the route exists
		tempCtx := &Context{
			paramValues: make(map[string]string),
		}
		if handlers := root.matchRoute(path, tempCtx); handlers != nil {
			return true
		}
	}
	return false
}

// allowed determines which HTTP methods are supported for a given path
// Returns a comma-separated list of allowed methods for the path
func (r *router) allowed(reqMethod, path string, ctx *Context) string {
	var allow string
	pathLen := len(path)

	// Handle * and /* requests
	if (pathLen == 1 && path[0] == '*') || (pathLen > 1 && path[1] == '*') {
		for method := range r.trees {
			if method == MethodOptions {
				continue
			}
			if allow != "" {
				allow += ", " + method
			} else {
				allow = method
			}
		}
		return allow
	}

	for method, tree := range r.trees {
		if method == reqMethod || method == MethodOptions {
			continue
		}
		if handlers := tree.matchRoute(path, ctx); handlers != nil {
			if allow != "" {
				allow += ", " + method
			} else {
				allow = method
			}
		}
	}

	if len(allow) > 0 {
		allow += ", " + MethodOptions
	}
	return allow
}

// Handler is the main request handler that processes incoming HTTP requests
// It manages context lifecycle and handles routes requests to appropriate handlers in router.Handler method
func (r *router) Handler(fctx *fasthttp.RequestCtx) {
	// Set gonoleks app instance for template engine access
	fctx.SetUserValue("gonoleksApp", r.app)

	// Acquire context from pool
	ctx := r.acquireCtx(fctx)
	defer r.releaseCtx(ctx)

	// Apply logging middleware for Default() mode (all requests)
	if r.app != nil && r.app.enableLogging {
		ctx.handlers = append(ctx.handlers, LoggerWithFormatter(DefaultLogFormatter))
	}

	// Extract method and path with zero-copy optimization
	methodBytes := fctx.Method()
	pathBytes := fctx.Path()

	var method, path string
	if r.app.CaseInSensitive {
		method = strings.ToUpper(getString(methodBytes))
		path = strings.ToLower(getString(pathBytes))
	} else {
		method = getString(methodBytes)
		path = getString(pathBytes)
	}

	// Try to handle the route
	if r.handleRoute(method, path, ctx) {
		// Route was handled successfully, execute middleware chain
		ctx.Next()
		return
	}

	// Route not found, handle special cases but ensure logging still happens
	handled := false

	// Handle method not allowed
	if !handled && r.app.HandleMethodNotAllowed {
		if r.handleMethodNotAllowed(fctx, method, path, ctx) {
			handled = true
		}
	}

	// Handle not found
	if !handled {
		// Apply global middleware for error responses in production mode
		if r.app != nil && !r.app.enableLogging && len(r.globalMiddleware) > 0 {
			ctx.handlers = append(ctx.handlers, r.globalMiddleware...)
		}
		if r.noRoute != nil {
			ctx.handlers = append(ctx.handlers, r.noRoute...)
		} else {
			fctx.Error(fasthttp.StatusMessage(StatusNotFound), StatusNotFound)
		}
	}

	// Always execute middleware chain to ensure logging happens
	ctx.Next()
}

// handleRoute processes a request by matching it against the routing tree
//
//go:noinline
//go:nosplit
func (r *router) handleRoute(method, path string, context *Context) bool {
	// Fast path: optimized FastRouter with pointer arithmetic
	if r.fastRouter != nil {
		// Use unsafe pointer operations for performance
		methodPtr := unsafe.Pointer(unsafe.StringData(method))
		pathPtr := unsafe.Pointer(unsafe.StringData(path))

		// Try fast lookup first
		if handlers, exists := r.fastRouter.UltraFastLookup(methodPtr, pathPtr, len(method), len(path)); exists {
			context.handlers = append(context.handlers, handlers...)
			return true
		}

		// Fallback to regular fast lookup
		if handlers, exists := r.fastRouter.FastLookup(method, path); exists {
			context.handlers = append(context.handlers, handlers...)
			return true
		}
	}

	// Fast path: optimized method lookup with better branch prediction
	var root *node
	// Switch for most common methods first (better branch prediction)
	switch method {
	case MethodGet:
		root = r.getTree
	case MethodPost:
		root = r.postTree
	case MethodPut:
		root = r.putTree
	case MethodDelete, MethodPatch, MethodHead, MethodOptions, MethodConnect, MethodTrace:
		// Fallback to map lookup for less common methods
		root = r.trees[method]
	default:
		// Fallback to map lookup for any other methods
		root = r.trees[method]
	}

	if root == nil {
		return false
	}

	// Fast path: optimized tree traversal for parameterized routes
	handlers := root.matchRoute(path, context)
	if handlers != nil {
		context.handlers = append(context.handlers, handlers...)
		return true
	}

	return false
}

// handleMethodNotAllowed generates a 405 Method Not Allowed response
// Returns true if the request was handled, false otherwise
func (r *router) handleMethodNotAllowed(fctx *fasthttp.RequestCtx, method, path string, context *Context) bool {
	if allow := r.allowed(method, path, context); len(allow) > 0 {
		fctx.Response.Header.Set(HeaderAllow, allow)

		// Use custom handlers if available
		if r.noMethod != nil {
			// Apply global middleware for error responses in production mode
			if r.app != nil && !r.app.enableLogging && len(r.globalMiddleware) > 0 {
				context.handlers = append(context.handlers, r.globalMiddleware...)
			}
			fctx.SetStatusCode(StatusMethodNotAllowed)
			context.handlers = append(context.handlers, r.noMethod...)
			return true
		}

		// Apply global middleware for error responses in production mode
		if r.app != nil && !r.app.enableLogging && len(r.globalMiddleware) > 0 {
			context.handlers = append(context.handlers, r.globalMiddleware...)
		}
		// Default Method Not Allowed response
		fctx.SetStatusCode(StatusMethodNotAllowed)
		fctx.SetContentTypeBytes([]byte(MIMETextPlainCharsetUTF8))
		fctx.SetBodyString(fasthttp.StatusMessage(StatusMethodNotAllowed))
		return true
	}
	return false
}

// SetNoRoute registers custom handler functions for 404 Not Found responses
// These handlers will be executed when no matching route is found
func (r *router) SetNoRoute(handlers handlersChain) {
	r.noRoute = append(r.noRoute, handlers...)
}

// NewFastRouter creates a new fast router with optimizations
func NewFastRouter() *FastRouter {
	fr := &FastRouter{
		routeMap: make(map[uintptr]handlersChain, 2048),
		ctxPool: sync.Pool{
			New: func() any {
				// Pre-allocate with optimal sizes for zero-reallocation
				return &Context{
					paramValues: make(map[string]string, 8),
					handlers:    make(handlersChain, 0, 16),
					index:       -1,
				}
			},
		},
	}

	// Pre-warm the context pool
	for i := 0; i < 32; i++ {
		ctx := fr.ctxPool.Get().(*Context)
		fr.ctxPool.Put(ctx)
	}

	return fr
}

// AddRoute adds a static route
//
//go:noinline
func (fr *FastRouter) AddRoute(method, path string, handlers handlersChain) {
	// Use string interning for lookups
	routeKey := method + path
	if interned, ok := fr.stringPool.Load(routeKey); ok {
		routeKey = interned.(string)
	} else {
		fr.stringPool.Store(routeKey, routeKey)
	}

	// Store using pointer-based key for fast lookup
	ptrKey := stringToPointer(routeKey)
	fr.routeMap[ptrKey] = handlers

	// Add to common routes cache
	hash := fastHash([]byte(routeKey))
	index := hash & 1023 // Bit mask for power-of-2 modulo
	fr.commonRoutes[index] = commonRoute{
		key:      hash,
		handlers: handlers,
	}

	// Add to route cache
	cacheIndex := hash & 255
	fr.routeCache[cacheIndex] = routeCacheEntry{
		key:      ptrKey,
		handlers: handlers,
	}
}

// fastHash is an ultra-fast hash function for route keys
//
//go:noinline
//go:nosplit
func fastHash(data []byte) uint32 {
	hash := fnvOffsetBasis
	for _, b := range data {
		hash *= fnvPrime
		hash ^= uint64(b)
	}
	return uint32(hash)
}

// stringToPointer converts a string to a uintptr
//
//go:noinline
//go:nosplit
func stringToPointer(s string) uintptr {
	return uintptr(unsafe.Pointer(unsafe.StringData(s)))
}

// FastLookup performs fast route lookup
//
//go:noinline
//go:nosplit
func (fr *FastRouter) FastLookup(method, path string) (handlersChain, bool) {
	// Optimized hash computation without string concatenation
	hash := fnvOffsetBasis
	// Hash method string directly
	for i := 0; i < len(method); i++ {
		hash *= fnvPrime
		hash ^= uint64(method[i])
	}
	// Hash path string directly
	for i := 0; i < len(path); i++ {
		hash *= fnvPrime
		hash ^= uint64(path[i])
	}
	hash32 := uint32(hash)

	// Level 1: Check CPU cache-optimized route cache first
	cacheIndex := hash32 & 255
	if entry := &fr.routeCache[cacheIndex]; entry.key != 0 {
		// Only create route key when cache hit is possible
		routeKey := method + path
		if stringToPointer(routeKey) == entry.key {
			return entry.handlers, true
		}
	}

	// Level 2: Check common routes with bit-masked indexing
	commonIndex := hash32 & 1023
	if common := &fr.commonRoutes[commonIndex]; common.key == hash32 {
		return common.handlers, true
	}

	// Level 3: Fallback to pointer-based map lookup
	routeKey := method + path
	if interned, ok := fr.stringPool.Load(routeKey); ok {
		ptrKey := stringToPointer(interned.(string))
		if handlers, exists := fr.routeMap[ptrKey]; exists {
			return handlers, true
		}
	}

	return nil, false
}

// GetContext gets a context from the pool
//
//go:noinline
func (fr *FastRouter) GetContext() *Context {
	return fr.ctxPool.Get().(*Context)
}

// PutContext returns a context to the pool with reset
//
//go:noinline
//go:nosplit
func (fr *FastRouter) PutContext(ctx *Context) {
	// Fast reset without memory allocations
	ctx.index = -1
	ctx.fullPath = ""
	ctx.handlers = ctx.handlers[:0]

	// Optimized map clearing - reuse allocated memory
	if len(ctx.paramValues) > 0 {
		for k := range ctx.paramValues {
			delete(ctx.paramValues, k)
		}
	}

	fr.ctxPool.Put(ctx)
}

// UltraFastLookup performs fast route lookup
//
//go:noinline
//go:nosplit
func (fr *FastRouter) UltraFastLookup(methodPtr, pathPtr unsafe.Pointer, methodLen, pathLen int) (handlersChain, bool) {
	// Direct memory access for performance
	methodBytes := unsafe.Slice((*byte)(methodPtr), methodLen)
	pathBytes := unsafe.Slice((*byte)(pathPtr), pathLen)

	// Hash computation using direct memory access
	hash := fnvOffsetBasis
	// Hash method bytes directly
	for i := 0; i < methodLen; i++ {
		hash *= fnvPrime
		hash ^= uint64(methodBytes[i])
	}
	// Hash path bytes directly
	for i := 0; i < pathLen; i++ {
		hash *= fnvPrime
		hash ^= uint64(pathBytes[i])
	}
	hash32 := uint32(hash)

	// Direct cache lookup with prefetch hint
	cacheIndex := hash32 & 255
	entry := &fr.routeCache[cacheIndex]

	if entry.key != 0 {
		// Create route key string only when needed for comparison
		combinedLen := methodLen + pathLen
		combined := make([]byte, combinedLen)
		copy(combined[:methodLen], methodBytes)
		copy(combined[methodLen:], pathBytes)
		routeKey := unsafe.String(&combined[0], combinedLen)

		// Check if we have the interned version of this string
		if interned, ok := fr.stringPool.Load(routeKey); ok {
			if stringToPointer(interned.(string)) == entry.key {
				return entry.handlers, true
			}
		}
	}

	// Fallback: create route key for string pool lookup
	combinedLen := methodLen + pathLen
	combined := make([]byte, combinedLen)
	copy(combined[:methodLen], methodBytes)
	copy(combined[methodLen:], pathBytes)
	routeKey := unsafe.String(&combined[0], combinedLen)

	if interned, ok := fr.stringPool.Load(routeKey); ok {
		ptrKey := stringToPointer(interned.(string))
		if handlers, exists := fr.routeMap[ptrKey]; exists {
			return handlers, true
		}
	}

	return nil, false
}

// WarmupCache pre-loads frequently used routes into cache
func (fr *FastRouter) WarmupCache(routes []string) {
	for _, route := range routes {
		// Trigger cache loading
		fr.FastLookup(MethodGet, route)
		fr.FastLookup(MethodPost, route)
	}
}
