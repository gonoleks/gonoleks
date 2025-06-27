package gonoleks

import (
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/prefork"
)

const (
	// defaultCacheSize is maximum number of entries in the routing cache
	defaultCacheSize = 10000

	// defaultConcurrency is the maximum number of concurrent connections
	defaultConcurrency = 512 * 1024

	// defaultMaxRequestBodySize is the maximum request body size the server
	defaultMaxRequestBodySize = 4 * 1024 * 1024

	// defaultMaxRouteParams is the maximum number of routes params
	defaultMaxRouteParams = 1024

	// defaultMaxRequestURLLength is the maximum request URL length
	defaultMaxRequestURLLength = 2048

	// defaultReadBufferSize is the default size of the read buffer
	defaultReadBufferSize = 8192 * 2

	// defaultMaxCachedParams is the maximum number of parameters to cache per route
	defaultMaxCachedParams = 20

	// defaultMaxCachedPathLength is the maximum length of path to cache
	defaultMaxCachedPathLength = 200
)

// Gonoleks interface defines the core functionality of the web framework
type Gonoleks interface {
	Run(addr ...string) error
	Shutdown() error
	Group(path string) *RouterGroup
	Static(path, root string)
	StaticFile(path, filepath string)
	Use(middleware ...handlerFunc)
	NoRoute(handlers ...handlerFunc)
	NoMethod(handlers ...handlerFunc)
	SecureJsonPrefix(prefix string)
	HandleContext(c *Context)
	LoadHTMLGlob(pattern string) error
	LoadHTMLFiles(files ...string) error
	SetFuncMap(funcMap map[string]any)
	Delims(left, right string)
	Handle(httpMethod, path string, handlers ...handlerFunc) *Route
	Any(path string, handlers ...handlerFunc) []*Route
	Match(methods []string, path string, handlers ...handlerFunc) []*Route
	GET(path string, handlers ...handlerFunc) *Route
	HEAD(path string, handlers ...handlerFunc) *Route
	PATCH(path string, handlers ...handlerFunc) *Route
	POST(path string, handlers ...handlerFunc) *Route
	PUT(path string, handlers ...handlerFunc) *Route
	DELETE(path string, handlers ...handlerFunc) *Route
	CONNECT(path string, handlers ...handlerFunc) *Route
	OPTIONS(path string, handlers ...handlerFunc) *Route
	TRACE(path string, handlers ...handlerFunc) *Route
}

// gonoleks implements Gonoleks interface
type gonoleks struct {
	httpServer       *fasthttp.Server
	router           *router
	registeredRoutes []*Route
	address          string
	middlewares      handlersChain
	options          *Options
	htmlRender       HTMLRender
	secureJsonPrefix string
}

// Options struct holds server configuration options
type Options struct {
	// ServerName to send in response headers
	ServerName string // Default: "Gonoleks"

	// Controls the number of operating system threads that can execute
	// user-level Go code simultaneously. Zero means use runtime.NumCPU()
	MaxProcs int // Default: 0 (use runtime.NumCPU())

	// Aggressively reduces memory usage at the cost of higher CPU usage if set to true
	ReduceMemoryUsage bool // Default: false

	// Rejects all non-GET requests if set to true
	GetOnly bool // Default: false

	// Enables case-insensitive routing
	CaseInSensitive bool // Default: false

	// Maximum size of LRU cache used for routing optimization
	CacheSize int // Default: 10000

	// Disables LRU caching used to optimize routing performance
	DisableCaching bool // Default: false

	// Controls which HTTP methods are cached (comma-separated list)
	CacheMethods string // Default: "GET,HEAD"

	// Maximum number of parameters to cache per route
	// Routes with more parameters than this will not be cached
	MaxCachedParams int // Default: 10

	// Maximum length of path to cache
	// Paths longer than this will not be cached
	MaxCachedPathLength int // Default: 100

	// Per-connection buffer size for request reading
	// This also limits the maximum header size
	// Increase this buffer for clients sending multi-KB RequestURIs
	// and/or multi-KB headers (e.g., large cookies)
	ReadBufferSize int // Default: 8192 * 2

	// Enables HTTP 405 Method Not Allowed responses when a route exists
	// but the requested method is not supported, otherwise returns 404
	HandleMethodNotAllowed bool // Default: false

	// Enables automatic replies to OPTIONS requests when no handlers
	// are explicitly registered for that route
	HandleOPTIONS bool // Default: false

	// Enables automatic recovery from panics during handler execution
	// by responding with HTTP 500 and logging the error without stopping the service
	AutoRecover bool // Default: true

	// Maximum request body size
	MaxRequestBodySize int // Default: 4 * 1024 * 1024

	// Maximum number of route parameters count
	MaxRouteParams int // Default: 1024

	// Maximum request URL length
	MaxRequestURLLength int // Default: 2048

	// Maximum number of concurrent connections
	Concurrency int // Default: 512 * 1024

	// When enabled, spawns multiple Go processes listening on the same port
	Prefork bool // Default: false

	// Suppresses the Gonoleks startup message in console output
	DisableStartupMessage bool // Default: false

	// Disables keep-alive connections, causing the server to close connections
	// after sending the first response to client
	DisableKeepalive bool // Default: false

	// When enabled, excludes the default Date header from responses
	DisableDefaultDate bool // Default: false

	// When enabled, excludes the default Content-Type header from responses
	DisableDefaultContentType bool // Default: false

	// When enabled, prevents header name normalization (e.g., conteNT-tYPE -> Content-Type)
	DisableHeaderNamesNormalizing bool // Default: true

	// Maximum time allowed to read the full request including body
	ReadTimeout time.Duration // Default: 0

	// Maximum duration before timing out writes of the response
	WriteTimeout time.Duration // Default: 0

	// Maximum time to wait for the next request when keep-alive is enabled
	IdleTimeout time.Duration // Default: 0

	// Enables TLS (HTTPS) support
	TLSEnabled bool // Default: false

	// File path to the TLS certificate
	TLSCertPath string // Default: ""

	// File path to the TLS key
	TLSKeyPath string // Default: ""

	// Disables HTTP transaction logging
	DisableLogging bool // Default: false

	// Format string for log timestamps
	LogTimeFormat string // Default: "2006/01/02 15:04:05"

	// Prefix for log messages
	LogPrefix string // Default: ""

	// Controls whether the caller information is included in logs
	LogReportCaller bool // Default: false

	// Controls whether the host information is included in logs
	LogReportHost bool // Default: false

	// Controls whether client IP addresses are included in logs
	LogReportIP bool // Default: false

	// Controls whether the user agent information is included in logs
	LogReportUserAgent bool // Default: false

	// Controls whether request headers are included in logs
	LogReportRequestHeaders bool // Default: false

	// Controls whether request bodies are included in logs
	LogReportRequestBody bool // Default: false

	// Controls whether response bodies are included in logs
	LogReportResponseBody bool // Default: false

	// Controls whether error responses are included in logs
	LogReportResponseError bool // Default: false
}

// Route struct stores information about a registered HTTP route
type Route struct {
	Method   string
	Path     string
	Handlers handlersChain
}

// New returns a new blank instance of Gonoleks without any middleware attached
func New(opts ...*Options) Gonoleks {
	if len(opts) > 0 {
		return createInstance(opts[0])
	}

	return createInstance(&Options{
		AutoRecover:           false,
		DisableStartupMessage: true,
		DisableLogging:        true,
	})
}

// Default returns a new instance of Gonoleks with default options
func Default(opts ...*Options) Gonoleks {
	if len(opts) > 0 {
		return createInstance(opts[0])
	}

	return createInstance(&Options{
		AutoRecover: true,
	})
}

// createInstance creates a new instance of Gonoleks with provided options
func createInstance(opts *Options) Gonoleks {
	g := &gonoleks{
		registeredRoutes: make([]*Route, 0),
		middlewares:      make(handlersChain, 0),
		options:          opts,
		secureJsonPrefix: "while(1);",
	}

	g.setDefaultOptions()
	setLoggerOptions(g.options)

	cacheSize := g.options.CacheSize
	if g.options.DisableCaching {
		cacheSize = 1
	}

	cache, err := lru.New[string, any](cacheSize)
	if err != nil {
		log.Error(ErrCacheCreationFailed, "error", err, "requestedSize", cacheSize)
		cache, _ = lru.New[string, any](10)
	}

	g.router = &router{
		options: g.options,
		cache:   cache,
		pool: sync.Pool{
			New: func() any { return new(Context) },
		},
		app: g,
	}

	g.httpServer = g.newHTTPServer()
	return g
}

// setDefaultOptions sets default values for options
func (g *gonoleks) setDefaultOptions() {
	if g.options.MaxProcs > 0 {
		runtime.GOMAXPROCS(g.options.MaxProcs)
	} else {
		// Use all CPU cores
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	if g.options.CacheSize <= 0 {
		g.options.CacheSize = defaultCacheSize
	}

	if g.options.CacheMethods == "" {
		g.options.CacheMethods = "GET,HEAD"
	}

	if g.options.MaxCachedParams <= 0 {
		g.options.MaxCachedParams = defaultMaxCachedParams
	}

	if g.options.MaxCachedPathLength <= 0 {
		g.options.MaxCachedPathLength = defaultMaxCachedPathLength
	}

	if g.options.MaxRequestBodySize <= 0 {
		g.options.MaxRequestBodySize = defaultMaxRequestBodySize
	}

	if g.options.MaxRouteParams <= 0 || g.options.MaxRouteParams > defaultMaxRouteParams {
		g.options.MaxRouteParams = defaultMaxRouteParams
	}

	if g.options.MaxRequestURLLength <= 0 || g.options.MaxRequestURLLength > defaultMaxRequestURLLength {
		g.options.MaxRequestURLLength = defaultMaxRequestURLLength
	}
	if g.options.Concurrency <= 0 {
		g.options.Concurrency = defaultConcurrency
	}

	if g.options.ReadBufferSize == 0 {
		g.options.ReadBufferSize = defaultReadBufferSize
	}
}

// Run starts the HTTP server and begins handling requests
func (g *gonoleks) Run(addr ...string) error {
	portStr := ""
	if len(addr) > 0 {
		portStr = addr[0]
	}

	address := resolveAddress(portStr)
	g.setupRouter()

	if g.options.Prefork {
		if !g.options.DisableStartupMessage {
			printStartupMessage(address)
		}
		pf := prefork.New(g.httpServer)
		pf.Reuseport = true
		pf.Network = "tcp4"
		if g.options.TLSEnabled {
			return pf.ListenAndServeTLS(address, g.options.TLSCertPath, g.options.TLSKeyPath)
		}
		return pf.ListenAndServe(address)
	}

	ln, err := net.Listen("tcp4", address)
	if err != nil {
		return err
	}
	g.address = address
	if !g.options.DisableStartupMessage {
		printStartupMessage(address)
	}
	if g.options.TLSEnabled {
		return g.httpServer.ServeTLS(ln, g.options.TLSCertPath, g.options.TLSKeyPath)
	}
	return g.httpServer.Serve(ln)
}

// newHTTPServer creates and configures a new fasthttp server instance
func (g *gonoleks) newHTTPServer() *fasthttp.Server {
	serverName := "Gonoleks"
	if g.options.ServerName != "" {
		serverName = g.options.ServerName
	}

	return &fasthttp.Server{
		Name:                          serverName,
		Handler:                       g.router.Handler,
		Concurrency:                   g.options.Concurrency,
		DisableKeepalive:              g.options.DisableKeepalive,
		ReadBufferSize:                g.options.ReadBufferSize,
		WriteBufferSize:               g.options.ReadBufferSize,
		ReadTimeout:                   g.options.ReadTimeout,
		WriteTimeout:                  g.options.WriteTimeout,
		IdleTimeout:                   g.options.IdleTimeout,
		MaxRequestBodySize:            g.options.MaxRequestBodySize,
		DisableHeaderNamesNormalizing: g.options.DisableHeaderNamesNormalizing,
		GetOnly:                       g.options.GetOnly,
		ReduceMemoryUsage:             g.options.ReduceMemoryUsage,
		NoDefaultServerHeader:         true,
		NoDefaultDate:                 g.options.DisableDefaultDate,
		NoDefaultContentType:          g.options.DisableDefaultContentType,
	}
}

// registerRoute adds a new route with the specified method, path, and handlers
func (g *gonoleks) registerRoute(method, path string, handlers handlersChain) *Route {
	if g.options.CaseInSensitive {
		path = strings.ToLower(path)
	}
	route := &Route{Path: path, Method: method, Handlers: handlers}
	g.registeredRoutes = append(g.registeredRoutes, route)
	return route
}

// setupRouter initializes the router with all registered routes
func (g *gonoleks) setupRouter() {
	for _, route := range g.registeredRoutes {
		g.router.handle(route.Method, route.Path, append(g.middlewares, route.Handlers...))
	}
	g.registeredRoutes = nil
	g.middlewares = nil
}

// Shutdown gracefully shuts down the server
func (g *gonoleks) Shutdown() error {
	err := g.httpServer.Shutdown()
	if err == nil && g.address != "" {
		log.Infof("Gonoleks stopped listening on %s", g.address)
		return nil
	}
	return err
}

// Group creates a new router group with the specified path prefix
func (g *gonoleks) Group(prefix string) *RouterGroup {
	if g.options.CaseInSensitive {
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
func (g *gonoleks) Static(path, root string) {
	if g.options.CaseInSensitive {
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
func (g *gonoleks) StaticFile(path, filepath string) {
	if g.options.CaseInSensitive {
		path = strings.ToLower(path)
	}

	handler := func(c *Context) {
		c.File(filepath)
	}

	g.GET(path, handler)
}

// Use registers global middleware functions to be executed for all routes
func (g *gonoleks) Use(middlewares ...handlerFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// NoRoute registers custom handlers for 404 Not Found responses
func (g *gonoleks) NoRoute(handlers ...handlerFunc) {
	g.router.noRoute = handlers
}

// NoMethod registers custom handlers for 405 Method Not Allowed responses
// Note: Only works when HandleMethodNotAllowed: true
func (g *gonoleks) NoMethod(handlers ...handlerFunc) {
	g.router.noMethod = handlers
}

// SecureJsonPrefix sets the secureJSONPrefix used in Context.SecureJSON
func (g *gonoleks) SecureJsonPrefix(prefix string) {
	g.secureJsonPrefix = prefix
}

// HandleContext re-enters a context that has been rewritten
// This can be done by setting c.Context.URI.SetPath to your new target
func (g *gonoleks) HandleContext(c *Context) {
	g.router.Handler(c.requestCtx)
}

// HTMLGlob loads HTML templates with the given pattern
// and associates the result with the HTML renderer
func (g *gonoleks) LoadHTMLGlob(pattern string) error {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	return g.htmlRender.(*TemplateEngine).LoadGlob(pattern)
}

// LoadHTMLFiles loads HTML templates from the given files
func (g *gonoleks) LoadHTMLFiles(files ...string) error {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	return g.htmlRender.(*TemplateEngine).LoadFiles(files...)
}

// SetFuncMap sets template function map
func (g *gonoleks) SetFuncMap(funcMap map[string]any) {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	g.htmlRender.(*TemplateEngine).SetFuncMap(funcMap)
}

// Delims sets template delimiters
func (g *gonoleks) Delims(left, right string) {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	g.htmlRender.(*TemplateEngine).SetDelims(left, right)
}

// Handle registers a new request handle and middleware with the given path and custom HTTP method
// The last handler should be the real handler, the other ones should be middleware
func (g *gonoleks) Handle(httpMethod, path string, handlers ...handlerFunc) *Route {
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
func (g *gonoleks) Any(path string, handlers ...handlerFunc) []*Route {
	return g.Match(AllHTTPMethods, path, handlers...)
}

// Match registers a route that matches the specified methods that you declared
func (g *gonoleks) Match(methods []string, path string, handlers ...handlerFunc) []*Route {
	routes := make([]*Route, 0, len(methods))

	for _, method := range methods {
		routes = append(routes, g.Handle(method, path, handlers...))
	}

	return routes
}

// GET registers a route for the HTTP GET method
func (g *gonoleks) GET(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodGet, path, handlers)
}

// HEAD registers a route for the HTTP HEAD method
func (g *gonoleks) HEAD(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodHead, path, handlers)
}

// POST registers a route for the HTTP POST method
func (g *gonoleks) POST(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodPost, path, handlers)
}

// PUT registers a route for the HTTP PUT method
func (g *gonoleks) PUT(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodPut, path, handlers)
}

// PATCH registers a route for the HTTP PATCH method
func (g *gonoleks) PATCH(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodPatch, path, handlers)
}

// DELETE registers a route for the HTTP DELETE method
func (g *gonoleks) DELETE(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodDelete, path, handlers)
}

// CONNECT registers a route for the HTTP CONNECT method
func (g *gonoleks) CONNECT(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodConnect, path, handlers)
}

// OPTIONS registers a route for the HTTP OPTIONS method
func (g *gonoleks) OPTIONS(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodOptions, path, handlers)
}

// TRACE registers a route for the HTTP TRACE method
func (g *gonoleks) TRACE(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodTrace, path, handlers)
}

// printStartupMessage displays server startup information in the console
func printStartupMessage(addr string) {
	if prefork.IsChild() {
		log.Infof("Started child proc #%d", os.Getpid())
	} else {
		log.Infof("Gonoleks started on http://%s", addr)
	}
}
