package gonoleks

import (
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/prefork"
)

// Options holds all configuration options
type Options struct {
	// ServerName to send in response headers
	ServerName string

	// GETOnly rejects all non-GET requests if set to true
	GETOnly bool

	// CaseInSensitive enables case-insensitive routing
	CaseInSensitive bool

	// ReadBufferSize is the per-connection buffer size for request reading
	// This also limits the maximum header size
	// Increase this buffer for clients sending multi-KB RequestURIs
	// and/or multi-KB headers (e.g., large cookies)
	ReadBufferSize int

	// WriteBufferSize is the per-connection buffer size for response writing
	WriteBufferSize int

	// HandleMethodNotAllowed enables HTTP 405 Method Not Allowed responses when a route exists
	// but the requested method is not supported, otherwise returns 404
	HandleMethodNotAllowed bool

	// MaxRequestBodySize is the maximum request body size
	MaxRequestBodySize int

	// MaxRouteParams is the maximum number of route parameters count
	MaxRouteParams int

	// MaxRequestURLLength is the maximum request URL length
	MaxRequestURLLength int

	// Concurrency is the maximum number of concurrent connections
	Concurrency int

	// Prefork spawns multiple Go processes listening on the same port when enabled
	Prefork bool

	// DisableKeepalive disables keep-alive connections, causing the server to close connections
	// after sending the first response to client
	DisableKeepalive bool

	// DisableDefaultDate excludes the default Date header from responses when enabled
	DisableDefaultDate bool

	// DisableDefaultContentType excludes the default Content-Type header from responses when enabled
	DisableDefaultContentType bool

	// DisableDefaultServerHeader excludes the default Server header from responses when enabled
	DisableDefaultServerHeader bool

	// DisableHeaderNamesNormalizing prevents header name normalization when enabled
	DisableHeaderNamesNormalizing bool

	// ReadTimeout is the maximum time allowed to read the full request including body
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration

	// IdleTimeout is the maximum time to wait for the next request when keep-alive is enabled
	IdleTimeout time.Duration
}

// Gonoleks is the main struct for the application
type Gonoleks struct {
	httpServer           *fasthttp.Server
	router               *router
	registeredRoutes     []*Route
	address              string
	middlewares          handlersChain
	htmlRender           HTMLRender
	secureJsonPrefix     string
	enableStartupMessage bool
	enableLogging        bool
	Options
}

// Route struct stores information about a registered HTTP route
type Route struct {
	Method   string
	Path     string
	Handlers handlersChain
}

// tlsConfig holds TLS configuration for HTTPS servers
type tlsConfig struct {
	certFile string
	keyFile  string
}

// New returns a new blank Gonoleks instance without any middleware attached
func New() *Gonoleks {
	return createInstance(false)
}

// Default returns an Gonoleks instance with logging and startup message
func Default() *Gonoleks {
	return createInstance(true)
}

func createInstance(debugMode bool) *Gonoleks {
	g := &Gonoleks{
		registeredRoutes:     make([]*Route, 0),
		middlewares:          make(handlersChain, 0),
		enableStartupMessage: debugMode,
		enableLogging:        debugMode,
		secureJsonPrefix:     "while(1);",
		Options:              defaultOptions(),
	}

	g.router = &router{
		pool: sync.Pool{
			New: func() any {
				return &Context{
					paramValues: make(map[string]string, 4),
					handlers:    make(handlersChain, 0, 6),
					index:       -1,
				}
			},
		},
		app: g,
	}

	// Pre-warm the pool with more contexts to reduce allocation overhead
	for i := 0; i < 16; i++ {
		ctx := g.router.pool.Get().(*Context)
		g.router.pool.Put(ctx)
	}

	g.httpServer = g.newHTTPServer()
	return g
}

// defaultOptions returns the default values for options
func defaultOptions() Options {
	return Options{
		ServerName: "Gonoleks",
	}
}

// Run starts the server and begins serving HTTP requests
func (g *Gonoleks) Run(addr ...string) error {
	var portStr string
	if len(addr) > 0 {
		portStr = addr[0]
	}
	address, networkProtocol := g.prepareServer(portStr)

	if g.Prefork {
		return g.runWithPrefork(address, networkProtocol, nil)
	}

	return g.runServer(address, networkProtocol, nil)
}

// RunTLS starts the server and begins serving HTTPS (secure) requests
func (g *Gonoleks) RunTLS(addr, certFile, keyFile string) error {
	tlsConf := &tlsConfig{
		certFile: certFile,
		keyFile:  keyFile,
	}
	address, networkProtocol := g.prepareServer(addr)

	if g.Prefork {
		return g.runWithPrefork(address, networkProtocol, tlsConf)
	}

	return g.runServer(address, networkProtocol, tlsConf)
}

// prepareServer prepares the server for running by setting up router and recreating HTTP server
func (g *Gonoleks) prepareServer(addr string) (string, string) {
	address := resolveAddress(addr)
	networkProtocol := detectNetworkProtocol(address)
	g.setupRouter()
	// Recreate httpServer with current configuration values
	g.httpServer = g.newHTTPServer()
	return address, networkProtocol
}

// runServer runs the server in standard mode
func (g *Gonoleks) runServer(address, networkProtocol string, tlsConfig *tlsConfig) error {
	listener, err := net.Listen(networkProtocol, address)
	if err != nil {
		return err
	}
	g.address = address
	if g.enableStartupMessage {
		printStartupMessage(address)
	}

	if tlsConfig != nil {
		return g.httpServer.ServeTLS(listener, tlsConfig.certFile, tlsConfig.keyFile)
	}
	return g.httpServer.Serve(listener)
}

// runWithPrefork runs the server in prefork mode
func (g *Gonoleks) runWithPrefork(address, networkProtocol string, tlsConfig *tlsConfig) error {
	if g.enableStartupMessage {
		printStartupMessage(address)
	}
	pf := prefork.New(g.httpServer)
	pf.Reuseport = true
	pf.Network = networkProtocol

	if tlsConfig != nil {
		return pf.ListenAndServeTLS(address, tlsConfig.certFile, tlsConfig.keyFile)
	}
	return pf.ListenAndServe(address)
}

// newHTTPServer creates and configures a new fasthttp server instance
func (g *Gonoleks) newHTTPServer() *fasthttp.Server {
	return &fasthttp.Server{
		Name:                          g.ServerName,
		Handler:                       g.router.Handler,
		ReduceMemoryUsage:             true,
		Concurrency:                   g.Concurrency,
		DisableKeepalive:              g.DisableKeepalive,
		ReadBufferSize:                g.ReadBufferSize,
		WriteBufferSize:               g.WriteBufferSize,
		ReadTimeout:                   g.ReadTimeout,
		WriteTimeout:                  g.WriteTimeout,
		IdleTimeout:                   g.IdleTimeout,
		MaxRequestBodySize:            g.MaxRequestBodySize,
		DisableHeaderNamesNormalizing: g.DisableHeaderNamesNormalizing,
		GetOnly:                       g.GETOnly,
		NoDefaultServerHeader:         g.DisableDefaultServerHeader,
		NoDefaultDate:                 g.DisableDefaultDate,
		NoDefaultContentType:          g.DisableDefaultContentType,
	}
}

// registerRoute adds a new route with the specified method, path, and handlers
func (g *Gonoleks) registerRoute(method, path string, handlers handlersChain) *Route {
	if g.CaseInSensitive {
		path = strings.ToLower(path)
	}

	route := &Route{
		Path:     path,
		Method:   method,
		Handlers: handlers,
	}

	// Add route to registered routes
	g.registeredRoutes = append(g.registeredRoutes, route)
	return route
}

// setupRouter initializes the router with all registered routes
func (g *Gonoleks) setupRouter() {
	// Store global middlewares in router before clearing them
	g.router.globalMiddleware = make(handlersChain, len(g.middlewares))
	copy(g.router.globalMiddleware, g.middlewares)

	for _, route := range g.registeredRoutes {
		g.router.handle(route.Method, route.Path, append(g.middlewares, route.Handlers...))
	}
	g.registeredRoutes = nil
	g.middlewares = nil
}

// Shutdown gracefully shuts down the server
func (g *Gonoleks) Shutdown() error {
	err := g.httpServer.Shutdown()
	if err == nil && g.address != "" {
		log.Infof("Gonoleks stopped listening on %s", g.address)
		return nil
	}
	return err
}

// Group creates a new router group with the specified path prefix
func (g *Gonoleks) Group(prefix string) *RouterGroup {
	if g.CaseInSensitive {
		prefix = strings.ToLower(prefix)
	}

	// Return a new RouterGroup for chaining
	return &RouterGroup{
		prefix:      prefix,
		app:         g,
		middlewares: make(handlersChain, 0),
	}
}

// Static serves static files from the specified root directory under the given URL prefix
func (g *Gonoleks) Static(path, root string) {
	if g.CaseInSensitive {
		path = strings.ToLower(path)
	}

	// Remove trailing slashes
	root = strings.TrimSuffix(root, "/")
	path = strings.TrimSuffix(path, "/")

	fs := &fasthttp.FS{
		Root:       root,
		IndexNames: []string{"index.html"},
		PathRewrite: func(c *fasthttp.RequestCtx) []byte {
			requestPath := c.Path()
			if len(requestPath) >= len(path) {
				// Remove the route prefix from the request path
				requestPath = requestPath[len(path):]
			}
			if len(requestPath) == 0 {
				return []byte("/")
			}
			if requestPath[0] != '/' {
				return append([]byte("/"), requestPath...)
			}
			return requestPath
		},
	}

	fileHandler := fs.NewRequestHandler()
	handler := func(c *Context) {
		fctx := c.Context()
		fileHandler(fctx)
		status := fctx.Response.StatusCode()
		if status != StatusNotFound && status != StatusForbidden {
			return
		}

		// Pass to custom not found handlers if available
		if len(g.router.noRoute) > 0 {
			g.router.noRoute[0](c)
			return
		}

		// Default Not Found response
		fctx.Error(fasthttp.StatusMessage(StatusNotFound), StatusNotFound)
	}

	g.GET(path, handler)

	if len(path) > 0 && path[len(path)-1] != '*' {
		g.GET(path+"/*", handler)
	}
}

// StaticFile registers a single route in order to serve a single file of the local filesystem
// Example: app.StaticFile("favicon.ico", "./resources/favicon.ico")
func (g *Gonoleks) StaticFile(path, filepath string) {
	if g.CaseInSensitive {
		path = strings.ToLower(path)
	}

	handler := func(c *Context) {
		c.File(filepath)
	}

	g.GET(path, handler)
}

// Use registers global middleware functions to be executed for all routes
func (g *Gonoleks) Use(middlewares ...handlerFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// Recovery catches any panics that occur during request processing
// It logs the error and returns a 500 Internal Server Error response
func Recovery() handlerFunc {
	return func(c *Context) {
		defer func() {
			if rcv := recover(); rcv != nil {
				log.Error(ErrRecoveredFromError, "error", rcv)
				c.requestCtx.Error(fasthttp.StatusMessage(StatusInternalServerError), StatusInternalServerError)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// NoRoute registers custom handlers for 404 Not Found responses
func (g *Gonoleks) NoRoute(handlers ...handlerFunc) {
	g.router.noRoute = handlers
}

// NoMethod registers custom handlers for 405 Method Not Allowed responses
// Note: Only works when HandleMethodNotAllowed: true
func (g *Gonoleks) NoMethod(handlers ...handlerFunc) {
	g.router.noMethod = handlers
}

// SecureJsonPrefix sets the secureJSONPrefix used in Context.SecureJSON
func (g *Gonoleks) SecureJsonPrefix(prefix string) {
	g.secureJsonPrefix = prefix
}

// HandleContext re-enters a context that has been rewritten
// This can be done by setting c.Context.URI.SetPath to your new target
func (g *Gonoleks) HandleContext(c *Context) {
	g.router.Handler(c.requestCtx)
}

// HTMLGlob loads HTML templates with the given pattern
// and associates the result with the HTML renderer
func (g *Gonoleks) LoadHTMLGlob(pattern string) error {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	return g.htmlRender.(*TemplateEngine).LoadGlob(pattern)
}

// LoadHTMLFiles loads HTML templates from the given files
func (g *Gonoleks) LoadHTMLFiles(files ...string) error {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	return g.htmlRender.(*TemplateEngine).LoadFiles(files...)
}

// SetFuncMap sets template function map
func (g *Gonoleks) SetFuncMap(funcMap map[string]any) {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	g.htmlRender.(*TemplateEngine).SetFuncMap(funcMap)
}

// Delims sets template delimiters
func (g *Gonoleks) Delims(left, right string) {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	g.htmlRender.(*TemplateEngine).SetDelims(left, right)
}

// Handle registers a new request handle and middleware with the given path and custom HTTP method
// The last handler should be the real handler, the other ones should be middleware
func (g *Gonoleks) Handle(httpMethod, path string, handlers ...handlerFunc) *Route {
	totalHandlers := len(g.middlewares) + len(handlers)
	finalHandlers := make(handlersChain, totalHandlers)

	copy(finalHandlers[:len(g.middlewares)], g.middlewares)
	copy(finalHandlers[len(g.middlewares):], handlers)

	// Register the route in the router
	g.router.handle(httpMethod, path, finalHandlers)

	// Create and store route information
	route := &Route{
		Method:   httpMethod,
		Path:     path,
		Handlers: handlers,
	}
	g.registeredRoutes = append(g.registeredRoutes, route)

	return route
}

// Any registers a route that matches all the HTTP methods
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE
func (g *Gonoleks) Any(path string, handlers ...handlerFunc) []*Route {
	return g.Match(AllHTTPMethods, path, handlers...)
}

// Match registers a route that matches the specified methods that you declared
func (g *Gonoleks) Match(methods []string, path string, handlers ...handlerFunc) []*Route {
	routes := make([]*Route, 0, len(methods))

	for _, method := range methods {
		routes = append(routes, g.Handle(method, path, handlers...))
	}

	return routes
}

// GET registers a route for the HTTP GET method
func (g *Gonoleks) GET(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodGet, path, handlers)
}

// HEAD registers a route for the HTTP HEAD method
func (g *Gonoleks) HEAD(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodHead, path, handlers)
}

// POST registers a route for the HTTP POST method
func (g *Gonoleks) POST(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodPost, path, handlers)
}

// PUT registers a route for the HTTP PUT method
func (g *Gonoleks) PUT(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodPut, path, handlers)
}

// PATCH registers a route for the HTTP PATCH method
func (g *Gonoleks) PATCH(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodPatch, path, handlers)
}

// DELETE registers a route for the HTTP DELETE method
func (g *Gonoleks) DELETE(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodDelete, path, handlers)
}

// CONNECT registers a route for the HTTP CONNECT method
func (g *Gonoleks) CONNECT(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodConnect, path, handlers)
}

// OPTIONS registers a route for the HTTP OPTIONS method
func (g *Gonoleks) OPTIONS(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodOptions, path, handlers)
}

// TRACE registers a route for the HTTP TRACE method
func (g *Gonoleks) TRACE(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodTrace, path, handlers)
}

// Handler returns the fasthttp.RequestHandler for the gonoleks instance
// This allows external access to the underlying router handler for testing and benchmarking
func (g *Gonoleks) Handler() fasthttp.RequestHandler {
	return g.router.Handler
}

// printStartupMessage displays server startup information in the console
func printStartupMessage(addr string) {
	if prefork.IsChild() {
		log.Infof("Started child proc #%d", os.Getpid())
	} else {
		port := addr[strings.LastIndex(addr, ":"):]
		log.Infof("Gonoleks started on %s", port)
	}
}
