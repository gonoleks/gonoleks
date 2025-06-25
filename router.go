package gonoleks

import (
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/valyala/fasthttp"
)

// router handles HTTP request routing
type router struct {
	trees    map[string]*node        // Route trees by HTTP method
	cache    *lru.Cache[string, any] // LRU cache for optimizing route lookups
	noRoute  handlersChain           // Handlers for 404 Not Found responses
	noMethod handlersChain           // Handlers for 405 Method Not Allowed responses
	settings *Settings               // Server settings
	pool     sync.Pool               // Reused context objects
	app      *gonoleks               // Reference to the gonoleks app instance
}

// matchResult holds route match data
type matchResult struct {
	handlers handlersChain     // Request handlers
	params   map[string]string // URL parameters
}

// acquireCtx retrieves a context instance from the pool and initializes it
func (r *router) acquireCtx(fctx *fasthttp.RequestCtx) *Context {
	ctx := r.pool.Get().(*Context)

	// Pre-allocate with expected capacity to reduce resizing
	expectedParams := 16
	if ctx.paramValues == nil {
		ctx.paramValues = make(map[string]string, expectedParams)
	} else {
		clear(ctx.paramValues)
	}

	// Pre-allocate handlers slice to avoid growth
	if cap(ctx.handlers) < 8 {
		ctx.handlers = make(handlersChain, 0, 8)
	} else {
		ctx.handlers = ctx.handlers[:0]
	}

	ctx.requestCtx = fctx
	ctx.index = -1
	ctx.fullPath = ""

	return ctx
}

// releaseCtx returns a context to the pool after clearing its state
// This prevents memory leaks while allowing object reuse
func (r *router) releaseCtx(ctx *Context) {
	ctx.handlers = nil

	// Clear the map instead of setting to nil to reuse allocated memory
	if ctx.paramValues != nil {
		for k := range ctx.paramValues {
			delete(ctx.paramValues, k)
		}
	}

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

	// Get root of method if it exists, otherwise create it
	root := r.trees[method]
	if root == nil {
		root = createRootNode()
		r.trees[method] = root
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

// Handler processes all incoming HTTP requests
// It manages the request lifecycle, including routing, execution, and response generation
func (r *router) Handler(fctx *fasthttp.RequestCtx) {
	var startTime time.Time
	if !r.settings.DisableLogging {
		startTime = time.Now()
	}

	context := r.acquireCtx(fctx)
	defer r.releaseCtx(context)
	fctx.SetUserValue("gonoleksApp", r.app)

	if r.settings.AutoRecover {
		defer r.recoverFromPanic(fctx)
	}

	// Use byte slices directly where possible to avoid string allocations
	method := fctx.Method()
	path := fctx.URI().PathOriginal()

	// Validate path length to prevent potential issues
	if len(path) > r.settings.MaxRequestURLLength {
		fctx.Error("Request URL too long", StatusRequestURITooLong)
		return
	}

	// Only convert to string if case insensitivity is needed
	methodStr := getString(method)
	var pathStr string
	if r.settings.CaseInSensitive {
		pathStr = strings.ToLower(getString(path))
	} else {
		pathStr = getString(path)
	}

	// Streamlined handling with early returns
	if r.handleCache(methodStr, pathStr, context) ||
		r.handleRoute(methodStr, pathStr, context) ||
		(methodStr == MethodOptions && r.settings.HandleOPTIONS && r.handleOptions(fctx, methodStr, pathStr, context)) ||
		(r.settings.HandleMethodNotAllowed && r.handleMethodNotAllowed(fctx, methodStr, pathStr, context)) ||
		r.handleNoRoute(context) {
		// Request handled successfully
	} else {
		fctx.Error(fasthttp.StatusMessage(StatusNotFound), StatusNotFound)
	}

	// Conditional latency calculation
	if !r.settings.DisableLogging {
		latency := time.Since(startTime)
		logHTTPTransaction(fctx, latency)
	}
}

// recoverFromPanic catches any panics that occur during request processing
// It logs the error and returns a 500 Internal Server Error response
func (r *router) recoverFromPanic(fctx *fasthttp.RequestCtx) {
	if rcv := recover(); rcv != nil {
		log.Error(ErrRecoveredFromError, "error", rcv)
		fctx.Error(fasthttp.StatusMessage(StatusInternalServerError), StatusInternalServerError)
	}
}

// handleCache attempts to serve a request from the route cache
// Returns true if the request was handled from cache, false otherwise
func (r *router) handleCache(method, path string, context *Context) bool {
	if r.settings.DisableCaching || r.cache == nil {
		return false
	}

	// Use byte buffer for cache key to avoid allocations
	var keyBuf [256]byte // Stack-allocated buffer
	keyLen := len(method) + 1 + len(path)
	if keyLen > 255 {
		return false // Skip caching for very long paths
	}

	copy(keyBuf[:len(method)], method)
	keyBuf[len(method)] = ':'
	copy(keyBuf[len(method)+1:], path)
	cacheKey := string(keyBuf[:keyLen])

	if value, ok := r.cache.Get(cacheKey); ok {
		cacheResult := value.(*matchResult)
		context.handlers = cacheResult.handlers

		if len(cacheResult.params) > 0 {
			for k, v := range cacheResult.params {
				context.paramValues[k] = v
			}
		}

		context.Next()
		return true
	}

	return false
}

// handleRoute processes a request by matching it against the routing tree
// It also caches successful matches for future requests
func (r *router) handleRoute(method, path string, context *Context) bool {
	// Find the tree for this HTTP method
	root := r.trees[method]
	if root == nil {
		return false
	}

	// Try to find route in the tree
	handlers := root.matchRoute(path, context)
	if handlers != nil {
		// Cache the successful match if caching is enabled
		if !r.settings.DisableCaching && r.cache != nil &&
			(method == MethodGet || method == MethodPost || method == MethodHead || method == MethodOptions) {
			// Create a copy of the current parameter values to store in cache
			params := make(map[string]string, len(context.paramValues))
			for k, v := range context.paramValues {
				params[k] = v
			}

			// Store both handlers and params in cache
			cacheKey := method + ":" + path
			r.cache.Add(cacheKey, &matchResult{
				handlers: handlers,
				params:   params,
			})
		}

		context.handlers = handlers
		context.Next()
		return true
	}

	return false
}

// handleOptions processes OPTIONS requests when automatic handling is enabled
// Returns true if the request was handled, false otherwise
func (r *router) handleOptions(fctx *fasthttp.RequestCtx, method, path string, context *Context) bool {
	if allow := r.allowed(method, path, context); len(allow) > 0 {
		fctx.Response.Header.Set("Allow", allow)
		return true
	}
	return false
}

// handleMethodNotAllowed generates a 405 Method Not Allowed response
// Returns true if the request was handled, false otherwise
func (r *router) handleMethodNotAllowed(fctx *fasthttp.RequestCtx, method, path string, context *Context) bool {
	if allow := r.allowed(method, path, context); len(allow) > 0 {
		fctx.Response.Header.Set("Allow", allow)

		// Use custom handlers if available
		if r.noMethod != nil {
			fctx.SetStatusCode(StatusMethodNotAllowed)
			context.handlers = r.noMethod
			context.Next()
			return true
		}

		// Default Method Not Allowed response
		fctx.SetStatusCode(StatusMethodNotAllowed)
		fctx.SetContentTypeBytes([]byte(MIMETextPlainCharsetUTF8))
		fctx.SetBodyString(fasthttp.StatusMessage(StatusMethodNotAllowed))
		return true
	}
	return false
}

// handleNoRoute executes custom 404 Not Found handlers if configured
// Returns true if custom handlers were executed, false otherwise
func (r *router) handleNoRoute(context *Context) bool {
	if r.noRoute != nil {
		r.noRoute[0](context)
		return true
	}
	return false
}

// SetNoRoute registers custom handler functions for 404 Not Found responses
// These handlers will be executed when no matching route is found
func (r *router) SetNoRoute(handlers handlersChain) {
	r.noRoute = append(r.noRoute, handlers...)
}
