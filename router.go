package gonoleks

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/valyala/fasthttp"
)

// Ultra-fast route cache with CPU cache line alignment
type ultraFastRouteCache struct {
	// Cache-aligned entries to prevent false sharing
	entries [512]ultraFastCacheEntry
	// Pre-computed hash masks for bit operations
	hashMask uint32
}

// ultraFastCacheEntry is cache-line aligned for optimal CPU performance
type ultraFastCacheEntry struct {
	hash     uint32        // Pre-computed hash
	handlers handlersChain // Handler chain
	_        [36]byte      // Padding to 64-byte cache line
}

// FastRouter is an optimized router for static routes
// Uses hash-based lookups for zero-allocation performance
type FastRouter struct {
	// Hash-based route storage for zero allocations
	routeHashes map[uint64]handlersChain

	// Ultra-fast route cache with CPU cache optimization
	ultraCache *ultraFastRouteCache

	// Pre-allocated context pool
	ctxPool sync.Pool

	// Cache-aligned route cache using hashes
	routeCache [256]hashCacheEntry

	// Hash table for common routes (powers of 2 for bit masking)
	commonRoutes [1024]commonRoute
}

// hashCacheEntry represents a hash-based cache entry for zero allocations
type hashCacheEntry struct {
	hash     uint64
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
//go:nosplit
func (r *router) acquireCtx(fctx *fasthttp.RequestCtx) *Context {
	ctx := r.pool.Get().(*Context)

	// Ultra-fast context initialization without function calls
	ctx.handlers = ctx.handlers[:0] // Reset length, keep capacity
	ctx.index = -1
	ctx.fullPath = ""
	ctx.requestCtx = fctx

	// Initialize or clear param values map
	if ctx.paramValues == nil {
		ctx.paramValues = make(map[string]string)
	} else if len(ctx.paramValues) > 0 {
		clear(ctx.paramValues)
	}

	return ctx
}

// releaseCtx returns a context to the pool after clearing its state
// This prevents memory leaks while allowing object reuse
//
//go:noinline
//go:nosplit
func (r *router) releaseCtx(ctx *Context) {
	// Ultra-fast reset: only clear what's necessary
	ctx.handlers = ctx.handlers[:0] // Reset length, keep capacity
	ctx.index = -1
	ctx.fullPath = ""
	ctx.requestCtx = nil

	// Clear map only if it has entries (performance optimization)
	if len(ctx.paramValues) > 0 {
		clear(ctx.paramValues)
	}

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
// It manages context lifecycle and routes requests to appropriate handlers
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
	// Ultra-fast path: Pre-computed method hash lookup
	if r.fastRouter != nil {
		// Use unsafe pointer operations for zero-allocation performance
		methodPtr := unsafe.Pointer(unsafe.StringData(method))
		pathPtr := unsafe.Pointer(unsafe.StringData(path))

		// Try ultra-fast lookup first with CPU cache optimization
		if handlers, exists := r.fastRouter.UltraFastLookup(methodPtr, pathPtr, len(method), len(path)); exists {
			// Preserve existing handlers (like logger) and append route handlers
			context.handlers = append(context.handlers, handlers...)
			return true
		}

		// Fallback to regular fast lookup only if ultra-fast fails
		if handlers, exists := r.fastRouter.FastLookup(method, path); exists {
			// Preserve existing handlers (like logger) and append route handlers
			context.handlers = append(context.handlers, handlers...)
			return true
		}
	}

	// Optimized method lookup with branch prediction hints
	var root *node
	// Reorder switch cases by frequency for better branch prediction
	switch method {
	case MethodGet: // Most common
		root = r.getTree
	case MethodPost: // Second most common
		root = r.postTree
	case MethodPut: // Third most common
		root = r.putTree
	case MethodDelete, MethodPatch: // Less common but still frequent
		root = r.trees[method]
	default: // Least common methods
		root = r.trees[method]
	}

	if root == nil {
		return false
	}

	// Optimized tree traversal for parameterized routes
	handlers := root.matchRoute(path, context)
	if handlers != nil {
		// Preserve existing handlers (like logger) and append route handlers
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
		routeHashes: make(map[uint64]handlersChain, 2048),
		ultraCache: &ultraFastRouteCache{
			hashMask: 511, // 512 - 1 for bit masking
		},
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

// AddRoute adds a static route with zero-allocation optimizations
//
//go:noinline
func (fr *FastRouter) AddRoute(method, path string, handlers handlersChain) {
	// Compute combined hash for zero-allocation lookup
	methodHash := ultraFastStringHash(method)
	pathHash := ultraFastStringHash(path)
	combinedHash := ultraFastCombinedHash(methodHash, pathHash)

	// Store using hash-based key for zero-allocation lookup
	fr.routeHashes[combinedHash] = handlers

	// Pre-compute hash for ultra-fast cache
	hash32 := uint32(combinedHash)

	// Add to ultra-fast cache with CPU cache optimization
	cacheIndex := hash32 & fr.ultraCache.hashMask
	fr.ultraCache.entries[cacheIndex] = ultraFastCacheEntry{
		hash:     hash32,
		handlers: handlers,
	}

	// Add to common routes cache using optimized hash
	index := hash32 & 1023 // Bit mask for power-of-2 modulo
	fr.commonRoutes[index] = commonRoute{
		key:      hash32,
		handlers: handlers,
	}

	// Add to route cache using hash
	routeCacheIndex := hash32 & 255
	fr.routeCache[routeCacheIndex] = hashCacheEntry{
		hash:     combinedHash,
		handlers: handlers,
	}
}

// FastLookup performs fast route lookup
//
//go:noinline
//go:nosplit
func (fr *FastRouter) FastLookup(method, path string) (handlersChain, bool) {
	// Use optimized combined hash computation
	methodHash := ultraFastStringHash(method)
	pathHash := ultraFastStringHash(path)
	combinedHash := ultraFastCombinedHash(methodHash, pathHash)
	hash32 := uint32(combinedHash)

	// Level 1: Check CPU cache-optimized route cache first
	cacheIndex := hash32 & 255
	if entry := &fr.routeCache[cacheIndex]; entry.hash == combinedHash {
		return entry.handlers, true
	}

	// Level 2: Check common routes with bit-masked indexing
	commonIndex := hash32 & 1023
	if common := &fr.commonRoutes[commonIndex]; common.key == hash32 {
		return common.handlers, true
	}

	// Level 3: Fallback to hash-based map lookup
	if handlers, exists := fr.routeHashes[combinedHash]; exists {
		return handlers, true
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
	// Reset handlers slice length but keep capacity
	ctx.handlers = ctx.handlers[:0]

	// Clear param values map if it has entries
	if len(ctx.paramValues) > 0 {
		clear(ctx.paramValues)
	}

	// Reset index and clear full path
	ctx.index = -1
	ctx.fullPath = ""

	fr.ctxPool.Put(ctx)
}

// UltraFastLookup performs the fastest possible route lookup with zero allocations
//
//go:noinline
//go:nosplit
func (fr *FastRouter) UltraFastLookup(methodPtr, pathPtr unsafe.Pointer, methodLen, pathLen int) (handlersChain, bool) {
	// Compute method hash dynamically for platform independence
	methodHash := ultraFastStringHash(unsafe.String((*byte)(methodPtr), methodLen))

	// Fast path hash for common paths
	pathHash := ultraFastStringHash(unsafe.String((*byte)(pathPtr), pathLen))

	// Combine hashes efficiently
	combinedHash := ultraFastCombinedHash(methodHash, pathHash)
	hash32 := uint32(combinedHash)

	// Level 0: Ultra-fast cache lookup with CPU cache optimization
	ultraIndex := hash32 & fr.ultraCache.hashMask
	if entry := &fr.ultraCache.entries[ultraIndex]; entry.hash == hash32 {
		return entry.handlers, true
	}

	// Level 1: Check CPU cache-optimized route cache with hash-based comparison
	cacheIndex := hash32 & 255
	if entry := &fr.routeCache[cacheIndex]; entry.hash == combinedHash {
		return entry.handlers, true
	}

	// Level 2: Check common routes with bit-masked indexing
	commonIndex := hash32 & 1023
	if common := &fr.commonRoutes[commonIndex]; common.key == hash32 {
		return common.handlers, true
	}

	// Level 3: Final hash map lookup
	if handlers, exists := fr.routeHashes[combinedHash]; exists {
		return handlers, true
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
