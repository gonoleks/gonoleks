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
	NotFound(handlers ...handlerFunc)
	HandleContext(c *Context)
	LoadHTMLGlob(pattern string) error
	LoadHTMLFiles(files ...string) error
	SetHTMLTemplate(templ any)
	SetFuncMap(funcMap any)
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
	settings         *Settings
	htmlRender       HTMLRender
}

// Settings struct holds server configuration options
type Settings struct {
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

// Default returns a new instance of Gonoleks with default settings
func Default(settings ...*Settings) Gonoleks {
	if len(settings) > 0 {
		return createInstance(settings[0])
	}

	return createInstance(&Settings{
		AutoRecover: true,
	})
}

// New returns a new blank instance of Gonoleks without any middleware attached
func New(settings ...*Settings) Gonoleks {
	if len(settings) > 0 {
		return createInstance(settings[0])
	}

	return createInstance(&Settings{
		AutoRecover:           false,
		DisableStartupMessage: true,
		DisableLogging:        true,
	})
}

// createInstance creates a new instance of Gonoleks with provided settings
func createInstance(settings *Settings) Gonoleks {
	g := &gonoleks{
		registeredRoutes: make([]*Route, 0),
		middlewares:      make(handlersChain, 0),
		settings:         settings,
	}

	g.setDefaultSettings()
	setLoggerSettings(g.settings)

	cacheSize := g.settings.CacheSize
	if g.settings.DisableCaching {
		cacheSize = 1
	}

	cache, err := lru.New[string, any](cacheSize)
	if err != nil {
		log.Error(ErrCacheCreationFailed, "error", err, "requestedSize", cacheSize)
		cache, _ = lru.New[string, any](10)
	}

	g.router = &router{
		settings: g.settings,
		cache:    cache,
		pool: sync.Pool{
			New: func() any { return new(Context) },
		},
	}

	g.httpServer = g.newHTTPServer()
	return g
}

// setDefaultSettings sets default values for settings
func (g *gonoleks) setDefaultSettings() {
	if g.settings.MaxProcs > 0 {
		runtime.GOMAXPROCS(g.settings.MaxProcs)
	} else {
		// Use all CPU cores
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	if g.settings.CacheSize <= 0 {
		g.settings.CacheSize = defaultCacheSize
	}

	if g.settings.CacheMethods == "" {
		g.settings.CacheMethods = "GET,HEAD"
	}

	if g.settings.MaxCachedParams <= 0 {
		g.settings.MaxCachedParams = defaultMaxCachedParams
	}

	if g.settings.MaxCachedPathLength <= 0 {
		g.settings.MaxCachedPathLength = defaultMaxCachedPathLength
	}

	if g.settings.MaxRequestBodySize <= 0 {
		g.settings.MaxRequestBodySize = defaultMaxRequestBodySize
	}

	if g.settings.MaxRouteParams <= 0 || g.settings.MaxRouteParams > defaultMaxRouteParams {
		g.settings.MaxRouteParams = defaultMaxRouteParams
	}

	if g.settings.MaxRequestURLLength <= 0 || g.settings.MaxRequestURLLength > defaultMaxRequestURLLength {
		g.settings.MaxRequestURLLength = defaultMaxRequestURLLength
	}
	if g.settings.Concurrency <= 0 {
		g.settings.Concurrency = defaultConcurrency
	}

	if g.settings.ReadBufferSize == 0 {
		g.settings.ReadBufferSize = defaultReadBufferSize
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

	if g.settings.Prefork {
		if !g.settings.DisableStartupMessage {
			printStartupMessage(address)
		}
		pf := prefork.New(g.httpServer)
		pf.Reuseport = true
		pf.Network = "tcp4"
		if g.settings.TLSEnabled {
			return pf.ListenAndServeTLS(address, g.settings.TLSCertPath, g.settings.TLSKeyPath)
		}
		return pf.ListenAndServe(address)
	}

	ln, err := net.Listen("tcp4", address)
	if err != nil {
		return err
	}
	g.address = address
	if !g.settings.DisableStartupMessage {
		printStartupMessage(address)
	}
	if g.settings.TLSEnabled {
		return g.httpServer.ServeTLS(ln, g.settings.TLSCertPath, g.settings.TLSKeyPath)
	}
	return g.httpServer.Serve(ln)
}

// newHTTPServer creates and configures a new fasthttp server instance
func (g *gonoleks) newHTTPServer() *fasthttp.Server {
	serverName := "Gonoleks"
	if g.settings.ServerName != "" {
		serverName = g.settings.ServerName
	}

	return &fasthttp.Server{
		Name:                          serverName,
		Handler:                       g.router.Handler,
		Concurrency:                   g.settings.Concurrency,
		DisableKeepalive:              g.settings.DisableKeepalive,
		ReadBufferSize:                g.settings.ReadBufferSize,
		WriteBufferSize:               g.settings.ReadBufferSize,
		ReadTimeout:                   g.settings.ReadTimeout,
		WriteTimeout:                  g.settings.WriteTimeout,
		IdleTimeout:                   g.settings.IdleTimeout,
		MaxRequestBodySize:            g.settings.MaxRequestBodySize,
		DisableHeaderNamesNormalizing: g.settings.DisableHeaderNamesNormalizing,
		GetOnly:                       g.settings.GetOnly,
		ReduceMemoryUsage:             g.settings.ReduceMemoryUsage,
		NoDefaultServerHeader:         true,
		NoDefaultDate:                 g.settings.DisableDefaultDate,
		NoDefaultContentType:          g.settings.DisableDefaultContentType,
	}
}

// registerRoute adds a new route with the specified method, path, and handlers
func (g *gonoleks) registerRoute(method, path string, handlers handlersChain) *Route {
	if g.settings.CaseInSensitive {
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

// Stop gracefully shuts down the server
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
	if g.settings.CaseInSensitive {
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
	if g.settings.CaseInSensitive {
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
		if len(g.router.notFound) > 0 {
			g.router.notFound[0](c)
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
	if g.settings.CaseInSensitive {
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

// NotFound registers custom handlers for 404 Not Found responses
func (g *gonoleks) NotFound(handlers ...handlerFunc) {
	g.router.notFound = handlers
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

// SetHTMLTemplate sets a custom HTML template
func (g *gonoleks) SetHTMLTemplate(templ any) {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	g.htmlRender.(*TemplateEngine).SetTemplate(templ)
}

// SetFuncMap sets template function map
func (g *gonoleks) SetFuncMap(funcMap any) {
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

// HTMLRender sets the custom HTML renderer
func (g *gonoleks) HTMLRender(render HTMLRender) {
	g.htmlRender = render
}

// GET registers a route for the HTTP GET method
func (g *gonoleks) GET(path string, handlers ...handlerFunc) *Route {
	return g.registerRoute(MethodGet, path, handlers)
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

	return g.Match(methods, path, handlers...)
}

// Match registers a route that matches the specified methods that you declared
func (g *gonoleks) Match(methods []string, path string, handlers ...handlerFunc) []*Route {
	routes := make([]*Route, 0, len(methods))

	for _, method := range methods {
		routes = append(routes, g.Handle(method, path, handlers...))
	}

	return routes
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
