package gonoleks

import (
	"io/fs"
	"strings"

	"github.com/valyala/fasthttp"
)

// IRoutes defines all common routing methods that both Gonoleks and RouterGroup implement
type IRoutes interface {
 	Use(...handlerFunc) IRoutes
	Group(string, ...handlerFunc) *RouterGroup
	Handle(string, string, ...handlerFunc) *Route
	Any(string, ...handlerFunc) []*Route
	Match([]string, string, ...handlerFunc) []*Route
	GET(string, ...handlerFunc) *Route
	HEAD(string, ...handlerFunc) *Route
	POST(string, ...handlerFunc) *Route
	PUT(string, ...handlerFunc) *Route
	PATCH(string, ...handlerFunc) *Route
	DELETE(string, ...handlerFunc) *Route
	CONNECT(string, ...handlerFunc) *Route
	OPTIONS(string, ...handlerFunc) *Route
	TRACE(string, ...handlerFunc) *Route
	StaticFile(string, string)
	StaticFileFS(string, string, fs.FS)
	Static(string, string)
	StaticFS(string, fs.FS)
}

// RouterGroup represents a group of routes with a common prefix
// It embeds RouteHandler to inherit all routing methods
type RouterGroup struct {
	RouteHandler
}

// RouteHandler provides the core routing implementation that both Gonoleks and RouterGroup embed
type RouteHandler struct {
	app         *Gonoleks
	prefix      string
	middlewares handlersChain
}

// Use registers middleware functions to be executed for all routes of the specified group
func (rh *RouteHandler) Use(middleware ...handlerFunc) IRoutes {
	rh.middlewares = append(rh.middlewares, middleware...)
	return rh
}

// Group creates a new router group with the specified relativePath prefix
func (rh *RouteHandler) Group(relativePath string, handlers ...handlerFunc) *RouterGroup {
	if rh.app.CaseInSensitive {
		relativePath = strings.ToLower(relativePath)
	}

	// Create new middleware slice inheriting from parent
	newMiddlewares := make(handlersChain, len(rh.middlewares))
	copy(newMiddlewares, rh.middlewares)
	
	// Append any additional handlers passed to Group
	if len(handlers) > 0 {
		newMiddlewares = append(newMiddlewares, handlers...)
	}

	rg := &RouterGroup{}
	rg.RouteHandler = RouteHandler{
		app:         rh.app,
		prefix:      rh.prefix + relativePath,
		middlewares: newMiddlewares,
	}

	return rg
}

// BasePath returns the base path of router group
// For example, if group := app.Group("/rest/n/v1/api"), group.BasePath() is "/rest/n/v1/api"
func (rg *RouterGroup) BasePath() string {
	return rg.prefix
}

// Handle implements the core routing logic
func (rh *RouteHandler) Handle(httpMethod, relativePath string, handlers ...handlerFunc) *Route {
	if rh.app.CaseInSensitive {
		relativePath = strings.ToLower(relativePath)
	}

	fullPath := rh.prefix + relativePath

	// Combine middlewares: global + group + route handlers
	totalHandlers := len(rh.app.middlewares) + len(rh.middlewares) + len(handlers)
	finalHandlers := make(handlersChain, totalHandlers)

	// Copy global middleware first
	copy(finalHandlers[:len(rh.app.middlewares)], rh.app.middlewares)
	// Then group middleware
	copy(finalHandlers[len(rh.app.middlewares):len(rh.app.middlewares)+len(rh.middlewares)], rh.middlewares)
	// Finally route handlers
	copy(finalHandlers[len(rh.app.middlewares)+len(rh.middlewares):], handlers)

	// Register the main route
	route := rh.app.registerRoute(httpMethod, fullPath, finalHandlers)

	// Handle trailing slash normalization
	if len(fullPath) > 1 && fullPath[len(fullPath)-1] == '/' {
		pathWithoutSlash := fullPath[:len(fullPath)-1]
		rh.app.registerRoute(httpMethod, pathWithoutSlash, finalHandlers)
	}

	return route
}

// Any registers a route that matches all the HTTP methods
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE
func (rh *RouteHandler) Any(relativePath string, handlers ...handlerFunc) []*Route {
	methods := []string{
		MethodGet,
		MethodPost,
		MethodPut,
		MethodPatch,
		MethodHead,
		MethodOptions,
		MethodDelete,
		MethodConnect,
		MethodTrace,
	}

	return rh.Match(methods, relativePath, handlers...)
}

// Match registers a route that matches the specified methods that you declared
func (rh *RouteHandler) Match(methods []string, relativePath string, handlers ...handlerFunc) []*Route {
	routes := make([]*Route, 0, len(methods))
	for _, method := range methods {
		routes = append(routes, rh.Handle(method, relativePath, handlers...))
	}
	return routes
}

// GET registers a route for the HTTP GET method
func (rh *RouteHandler) GET(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodGet, relativePath, handlers...)
}

// HEAD registers a route for the HTTP HEAD method
func (rh *RouteHandler) HEAD(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodHead, relativePath, handlers...)
}

// POST registers a route for the HTTP POST method
func (rh *RouteHandler) POST(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodPost, relativePath, handlers...)
}

// PUT registers a route for the HTTP PUT method
func (rh *RouteHandler) PUT(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodPut, relativePath, handlers...)
}

// PATCH registers a route for the HTTP PATCH method
func (rh *RouteHandler) PATCH(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodPatch, relativePath, handlers...)
}

// DELETE registers a route for the HTTP DELETE method
func (rh *RouteHandler) DELETE(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodDelete, relativePath, handlers...)
}

// CONNECT registers a route for the HTTP CONNECT method
func (rh *RouteHandler) CONNECT(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodConnect, relativePath, handlers...)
}

// OPTIONS registers a route for the HTTP OPTIONS method
func (rh *RouteHandler) OPTIONS(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodOptions, relativePath, handlers...)
}

// TRACE registers a route for the HTTP TRACE method
func (rh *RouteHandler) TRACE(relativePath string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodTrace, relativePath, handlers...)
}

// StaticFile registers a single route in order to serve a single file of the local filesystem
//
//	app.StaticFile("favicon.ico", "./assets/favicon.ico")
func (rh *RouteHandler) StaticFile(relativePath, filePath string) {
	rh.staticFileHandler(relativePath, func(c *Context) {
		c.File(filePath)
	})
}

// StaticFileFS registers a single route to serve a single file from the given file system
//
//	app.StaticFileFS("favicon.ico", "favicon.ico", os.DirFS("./assets"))
func (rh *RouteHandler) StaticFileFS(relativePath, filePath string, fs fs.FS) {
	rh.staticFileHandler(relativePath, func(c *Context) {
		fasthttp.ServeFS(c.requestCtx, fs, filePath)
	})
}

// staticFileHandler is a helper function for single file serving
func (rh *RouteHandler) staticFileHandler(relativePath string, handler handlerFunc) {
	if rh.app.CaseInSensitive {
		relativePath = strings.ToLower(relativePath)
	}
	rh.GET(relativePath, handler)
}

// Static serves static files from the specified root directory under the given URL prefix
//
//	app.Static("/static", "./assets")
func (rh *RouteHandler) Static(relativePath, root string) {
	rh.createStaticHandler(relativePath, &fasthttp.FS{
		Root:       root,
		IndexNames: []string{"index.html"},
	})
}

// StaticFS serves static files from the given file system under the specified URL prefix
//
//	app.StaticFS("/static", os.DirFS("./assets"))
//	app.StaticFS("/static", embed.FS)
func (rh *RouteHandler) StaticFS(relativePath string, fs fs.FS) {
	rh.createStaticHandler(relativePath, &fasthttp.FS{
		FS:                 fs,
		Root:               "",
		AllowEmptyRoot:     true,
		IndexNames:         []string{"index.html"},
		GenerateIndexPages: false,
		Compress:           true,
		CompressBrotli:     true,
		AcceptByteRange:    true,
	})
}

// createStaticHandler is a helper function for directory serving with common logic
func (rh *RouteHandler) createStaticHandler(relativePath string, fs *fasthttp.FS) {
	if rh.app.CaseInSensitive {
		relativePath = strings.ToLower(relativePath)
	}
	fullPath := strings.TrimSuffix(rh.prefix+relativePath, "/")

	// Configure relativePath rewrite for the file system
	fs.PathRewrite = func(ctx *fasthttp.RequestCtx) []byte {
		requestPath := ctx.Path()
		if len(requestPath) >= len(fullPath) {
			// Remove the route prefix from the request relativePath
			requestPath = requestPath[len(fullPath):]
		}
		if len(requestPath) == 0 {
			return []byte("/")
		}
		if requestPath[0] != '/' {
			return append([]byte("/"), requestPath...)
		}
		return requestPath
	}

	fileHandler := fs.NewRequestHandler()
	handler := func(c *Context) {
		fctx := c.Context()
		fileHandler(fctx)

		// Handle not found cases
		status := fctx.Response.StatusCode()
		if status == StatusNotFound || status == StatusForbidden {
			// Pass to custom not found handlers if available
			if len(rh.app.router.noRoute) > 0 {
				rh.app.router.noRoute[0](c)
				return
			}

			// Default Not Found response
			c.requestCtx.Error(fasthttp.StatusMessage(StatusNotFound), StatusNotFound)
		}
	}

	rh.GET(relativePath, handler)

	// Register wildcard route if needed
	if len(relativePath) > 0 && relativePath[len(relativePath)-1] != '*' {
		rh.GET(relativePath+"/*", handler)
	}
}
