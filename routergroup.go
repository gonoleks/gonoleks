package gonoleks

// RouterGroup represents a group of routes with a common prefix
type RouterGroup struct {
	prefix      string
	app         *gonoleks
	middlewares handlersChain
}

// Group creates a new sub-group with an additional prefix
func (r *RouterGroup) Group(path string) *RouterGroup {
	return &RouterGroup{
		prefix:      r.prefix + path,
		app:         r.app,
		middlewares: make(handlersChain, len(r.middlewares)), // Inherit parent group middleware
	}
}

// BasePath returns the base path of router group
// For example, if group := app.Group("/rest/n/v1/api"), group.BasePath() is "/rest/n/v1/api"
func (r *RouterGroup) BasePath() string {
	return r.prefix
}

// Use registers middleware for the group
func (r *RouterGroup) Use(middleware ...handlerFunc) *RouterGroup {
	r.middlewares = append(r.middlewares, middleware...)
	return r
}

// Handle registers a new request handle and middleware with the given path and custom HTTP method
func (r *RouterGroup) Handle(httpMethod, path string, handlers ...handlerFunc) *Route {
	// Combine global middleware + group middleware + route handlers
	totalHandlers := len(r.app.middlewares) + len(r.middlewares) + len(handlers)
	finalHandlers := make(handlersChain, totalHandlers)

	// Copy global middleware first
	copy(finalHandlers[:len(r.app.middlewares)], r.app.middlewares)
	// Then group middleware
	copy(finalHandlers[len(r.app.middlewares):len(r.app.middlewares)+len(r.middlewares)], r.middlewares)
	// Finally route handlers
	copy(finalHandlers[len(r.app.middlewares)+len(r.middlewares):], handlers)

	// Register the route in the router
	r.app.router.handle(httpMethod, r.prefix+path, finalHandlers)

	// Create and store route information
	route := &Route{
		Method:   httpMethod,
		Path:     r.prefix + path,
		Handlers: handlers,
	}
	r.app.registeredRoutes = append(r.app.registeredRoutes, route)

	return route
}

// Any registers a route that matches all the HTTP methods
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE
func (r *RouterGroup) Any(path string, handlers ...handlerFunc) []*Route {
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

	routes := make([]*Route, 0, len(methods))
	for _, method := range methods {
		routes = append(routes, r.Handle(method, path, handlers...))
	}
	return routes
}

// Match registers a route that matches the specified methods that you declared
func (r *RouterGroup) Match(methods []string, path string, handlers ...handlerFunc) []*Route {
	routes := make([]*Route, 0, len(methods))
	for _, method := range methods {
		routes = append(routes, r.Handle(method, path, handlers...))
	}
	return routes
}

// GET registers a GET route with the group prefix
func (r *RouterGroup) GET(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodGet, path, handlers...)
}

// HEAD registers a HEAD route with the group prefix
func (r *RouterGroup) HEAD(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodHead, path, handlers...)
}

// POST registers a POST route with the group prefix
func (r *RouterGroup) POST(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodPost, path, handlers...)
}

// PUT registers a PUT route with the group prefix
func (r *RouterGroup) PUT(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodPut, path, handlers...)
}

// PATCH registers a PATCH route with the group prefix
func (r *RouterGroup) PATCH(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodPatch, path, handlers...)
}

// DELETE registers a DELETE route with the group prefix
func (r *RouterGroup) DELETE(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodDelete, path, handlers...)
}

// CONNECT registers a CONNECT route with the group prefix
func (r *RouterGroup) CONNECT(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodConnect, path, handlers...)
}

// OPTIONS registers an OPTIONS route with the group prefix
func (r *RouterGroup) OPTIONS(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodOptions, path, handlers...)
}

// TRACE registers a TRACE route with the group prefix
func (r *RouterGroup) TRACE(path string, handlers ...handlerFunc) *Route {
	return r.Handle(MethodTrace, path, handlers...)
}
