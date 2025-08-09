package gonoleks

import (
	"io/fs"
	"strings"

	"github.com/valyala/fasthttp"
)

// IRoutes defines all common routing methods that both Gonoleks and RouterGroup implement
type IRoutes interface {
	Use(middleware ...handlerFunc) IRoutes
	Group(prefix string) *RouterGroup
	Handle(httpMethod, path string, handlers ...handlerFunc) *Route
	Any(path string, handlers ...handlerFunc) []*Route
	Match(methods []string, path string, handlers ...handlerFunc) []*Route
	GET(path string, handlers ...handlerFunc) *Route
	HEAD(path string, handlers ...handlerFunc) *Route
	POST(path string, handlers ...handlerFunc) *Route
	PUT(path string, handlers ...handlerFunc) *Route
	PATCH(path string, handlers ...handlerFunc) *Route
	DELETE(path string, handlers ...handlerFunc) *Route
	CONNECT(path string, handlers ...handlerFunc) *Route
	OPTIONS(path string, handlers ...handlerFunc) *Route
	TRACE(path string, handlers ...handlerFunc) *Route
	StaticFile(path, filepath string)
	StaticFileFS(path, filepath string, filesystem fs.FS)
	Static(path, root string)
	StaticFS(path string, filesystem fs.FS)
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

// Group creates a new router group with the specified path prefix
func (rh *RouteHandler) Group(prefix string) *RouterGroup {
	if rh.app.CaseInSensitive {
		prefix = strings.ToLower(prefix)
	}

	// Create new middleware slice inheriting from parent
	newMiddlewares := make(handlersChain, len(rh.middlewares))
	copy(newMiddlewares, rh.middlewares)

	rg := &RouterGroup{}
	rg.RouteHandler = RouteHandler{
		app:         rh.app,
		prefix:      rh.prefix + prefix,
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
func (rh *RouteHandler) Handle(httpMethod, path string, handlers ...handlerFunc) *Route {
	if rh.app.CaseInSensitive {
		path = strings.ToLower(path)
	}

	fullPath := rh.prefix + path

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
func (rh *RouteHandler) Any(path string, handlers ...handlerFunc) []*Route {
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

	return rh.Match(methods, path, handlers...)
}

// Match registers a route that matches the specified methods that you declared
func (rh *RouteHandler) Match(methods []string, path string, handlers ...handlerFunc) []*Route {
	routes := make([]*Route, 0, len(methods))
	for _, method := range methods {
		routes = append(routes, rh.Handle(method, path, handlers...))
	}
	return routes
}

// GET registers a route for the HTTP GET method
func (rh *RouteHandler) GET(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodGet, path, handlers...)
}

// HEAD registers a route for the HTTP HEAD method
func (rh *RouteHandler) HEAD(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodHead, path, handlers...)
}

// POST registers a route for the HTTP POST method
func (rh *RouteHandler) POST(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodPost, path, handlers...)
}

// PUT registers a route for the HTTP PUT method
func (rh *RouteHandler) PUT(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodPut, path, handlers...)
}

// PATCH registers a route for the HTTP PATCH method
func (rh *RouteHandler) PATCH(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodPatch, path, handlers...)
}

// DELETE registers a route for the HTTP DELETE method
func (rh *RouteHandler) DELETE(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodDelete, path, handlers...)
}

// CONNECT registers a route for the HTTP CONNECT method
func (rh *RouteHandler) CONNECT(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodConnect, path, handlers...)
}

// OPTIONS registers a route for the HTTP OPTIONS method
func (rh *RouteHandler) OPTIONS(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodOptions, path, handlers...)
}

// TRACE registers a route for the HTTP TRACE method
func (rh *RouteHandler) TRACE(path string, handlers ...handlerFunc) *Route {
	return rh.Handle(MethodTrace, path, handlers...)
}

// StaticFile registers a single route in order to serve a single file of the local filesystem
//
//	app.StaticFile("favicon.ico", "./assets/favicon.ico")
func (rh *RouteHandler) StaticFile(path, filepath string) {
	rh.staticFileHandler(path, func(c *Context) {
		c.File(filepath)
	})
}

// StaticFileFS registers a single route to serve a single file from the given file system
//
//	app.StaticFileFS("favicon.ico", "favicon.ico", os.DirFS("./assets"))
func (rh *RouteHandler) StaticFileFS(path, filepath string, filesystem fs.FS) {
	rh.staticFileHandler(path, func(c *Context) {
		fasthttp.ServeFS(c.requestCtx, filesystem, filepath)
	})
}

// staticFileHandler is a helper function for single file serving
func (rh *RouteHandler) staticFileHandler(path string, handler handlerFunc) {
	if rh.app.CaseInSensitive {
		path = strings.ToLower(path)
	}
	rh.GET(path, handler)
}

// Static serves static files from the specified root directory under the given URL prefix
//
//	app.Static("/static", "./assets")
func (rh *RouteHandler) Static(path, root string) {
	rh.createStaticHandler(path, &fasthttp.FS{
		Root:       root,
		IndexNames: []string{"index.html"},
	})
}

// StaticFS serves static files from the given file system under the specified URL prefix
//
//	app.StaticFS("/static", os.DirFS("./assets"))
//	app.StaticFS("/static", embed.FS)
func (rh *RouteHandler) StaticFS(path string, filesystem fs.FS) {
	rh.createStaticHandler(path, &fasthttp.FS{
		FS:                 filesystem,
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
func (rh *RouteHandler) createStaticHandler(path string, fs *fasthttp.FS) {
	if rh.app.CaseInSensitive {
		path = strings.ToLower(path)
	}
	fullPath := strings.TrimSuffix(rh.prefix+path, "/")

	// Configure path rewrite for the file system
	fs.PathRewrite = func(ctx *fasthttp.RequestCtx) []byte {
		requestPath := ctx.Path()
		if len(requestPath) >= len(fullPath) {
			// Remove the route prefix from the request path
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

	rh.GET(path, handler)

	// Register wildcard route if needed
	if len(path) > 0 && path[len(path)-1] != '*' {
		rh.GET(path+"/*", handler)
	}
}
