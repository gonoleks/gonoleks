package gonoleks

import "strings"

// RouterGroup represents a group of routes with a common prefix
type RouterGroup struct {
	prefix      string
	app         *Gonoleks
	middlewares handlersChain
}

// Group creates a new sub-group with an additional prefix
func (rg *RouterGroup) Group(path string) *RouterGroup {
	if rg.app.CaseInSensitive {
		path = strings.ToLower(path)
	}
	return &RouterGroup{
		prefix:      rg.prefix + path,
		app:         rg.app,
		middlewares: make(handlersChain, len(rg.middlewares)), // Inherit parent group middleware
	}
}

// BasePath returns the base path of router group
// For example, if group := app.Group("/rest/n/v1/api"), group.BasePath() is "/rest/n/v1/api"
func (rg *RouterGroup) BasePath() string {
	return rg.prefix
}

// Use registers middleware for the group
func (rg *RouterGroup) Use(middleware ...handlerFunc) *RouterGroup {
	rg.middlewares = append(rg.middlewares, middleware...)
	return rg
}

// Handle registers a new request handle and middleware with the given path and custom HTTP method
func (rg *RouterGroup) Handle(httpMethod, path string, handlers ...handlerFunc) *Route {
	if rg.app.CaseInSensitive {
		path = strings.ToLower(path)
	}
	// Only register a default handler for the group root if there are handlers to register
	defaultHandlers := append(rg.app.middlewares, rg.middlewares...)
	if path != "/" && path != "" && !rg.app.router.routeExists(httpMethod, rg.prefix) && len(defaultHandlers) > 0 {
		rg.app.router.handle(httpMethod, rg.prefix, defaultHandlers)
	}
	// Combine global middleware + group middleware + route handlers
	totalHandlers := len(rg.app.middlewares) + len(rg.middlewares) + len(handlers)
	finalHandlers := make(handlersChain, totalHandlers)

	// Copy global middleware first
	copy(finalHandlers[:len(rg.app.middlewares)], rg.app.middlewares)
	// Then group middleware
	copy(finalHandlers[len(rg.app.middlewares):len(rg.app.middlewares)+len(rg.middlewares)], rg.middlewares)
	// Finally route handlers
	copy(finalHandlers[len(rg.app.middlewares)+len(rg.middlewares):], handlers)

	// Register the route in the router
	rg.app.router.handle(httpMethod, rg.prefix+path, finalHandlers)

	// Create and store route information
	route := &Route{
		Method:   httpMethod,
		Path:     rg.prefix + path,
		Handlers: handlers,
	}
	rg.app.registeredRoutes = append(rg.app.registeredRoutes, route)

	return route
}

// Any registers a route that matches all the HTTP methods
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE
func (rg *RouterGroup) Any(path string, handlers ...handlerFunc) []*Route {
	return rg.Match(AllHTTPMethods, path, handlers...)
}

// Match registers a route that matches the specified methods that you declared
func (rg *RouterGroup) Match(methods []string, path string, handlers ...handlerFunc) []*Route {
	routes := make([]*Route, 0, len(methods))

	for _, method := range methods {
		routes = append(routes, rg.Handle(method, path, handlers...))
	}

	return routes
}

// GET registers a GET route with the group prefix
func (rg *RouterGroup) GET(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodGet, path, handlers...)
}

// HEAD registers a HEAD route with the group prefix
func (rg *RouterGroup) HEAD(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodHead, path, handlers...)
}

// POST registers a POST route with the group prefix
func (rg *RouterGroup) POST(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodPost, path, handlers...)
}

// PUT registers a PUT route with the group prefix
func (rg *RouterGroup) PUT(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodPut, path, handlers...)
}

// PATCH registers a PATCH route with the group prefix
func (rg *RouterGroup) PATCH(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodPatch, path, handlers...)
}

// DELETE registers a DELETE route with the group prefix
func (rg *RouterGroup) DELETE(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodDelete, path, handlers...)
}

// CONNECT registers a CONNECT route with the group prefix
func (rg *RouterGroup) CONNECT(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodConnect, path, handlers...)
}

// OPTIONS registers an OPTIONS route with the group prefix
func (rg *RouterGroup) OPTIONS(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodOptions, path, handlers...)
}

// TRACE registers a TRACE route with the group prefix
func (rg *RouterGroup) TRACE(path string, handlers ...handlerFunc) *Route {
	return rg.Handle(MethodTrace, path, handlers...)
}
