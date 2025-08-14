package gonoleks

import (
	"io/fs"
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

	// Concurrency is the maximum number of concurrent connections
	Concurrency int

	// ReadBufferSize is the per-connection buffer size for request reading
	// This also limits the maximum header size
	// Increase this buffer for clients sending multi-KB RequestURIs
	// and/or multi-KB headers (e.g., large cookies)
	ReadBufferSize int

	// WriteBufferSize is the per-connection buffer size for response writing
	WriteBufferSize int

	// ReadTimeout is the maximum time allowed to read the full request including body
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration

	// IdleTimeout is the maximum time to wait for the next request when keep-alive is enabled
	IdleTimeout time.Duration

	// MaxRequestBodySize is the maximum request body size
	MaxRequestBodySize int

	// DisableKeepalive disables keep-alive connections, causing the server to close connections
	// after sending the first response to client
	DisableKeepalive bool

	// GETOnly rejects all non-GET requests if set to true
	GETOnly bool

	// DisableHeaderNamesNormalizing prevents header name normalization when enabled
	DisableHeaderNamesNormalizing bool

	// DisableDefaultServerHeader excludes the default Server header from responses when enabled
	DisableDefaultServerHeader bool

	// DisableDefaultDate excludes the default Date header from responses when enabled
	DisableDefaultDate bool

	// DisableDefaultContentType excludes the default Content-Type header from responses when enabled
	DisableDefaultContentType bool

	// CaseInSensitive enables case-insensitive routing
	CaseInSensitive bool

	// MaxRouteParams is the maximum number of route parameters count
	MaxRouteParams int

	// MaxRequestURLLength is the maximum request URL length
	MaxRequestURLLength int

	// HandleMethodNotAllowed enables HTTP 405 Method Not Allowed responses when a route exists
	// but the requested method is not supported, otherwise returns 404
	HandleMethodNotAllowed bool

	// Prefork spawns multiple Go processes listening on the same port when enabled
	Prefork bool
}

// Gonoleks is the main struct for the application
type Gonoleks struct {
	RouteHandler
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

	// Initialize the embedded RouteHandler
	g.RouteHandler = RouteHandler{
		app:         g,
		prefix:      "",
		middlewares: make(handlersChain, 0),
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
		g.printStartupMessage(address)
	}

	if tlsConfig != nil {
		return g.httpServer.ServeTLS(listener, tlsConfig.certFile, tlsConfig.keyFile)
	}
	return g.httpServer.Serve(listener)
}

// runWithPrefork runs the server in prefork mode
func (g *Gonoleks) runWithPrefork(address, networkProtocol string, tlsConfig *tlsConfig) error {
	if g.enableStartupMessage {
		g.printStartupMessage(address)
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
		Handler:                       g.router.Handler,
		Name:                          g.ServerName,
		Concurrency:                   g.Concurrency,
		ReadBufferSize:                g.ReadBufferSize,
		WriteBufferSize:               g.WriteBufferSize,
		ReadTimeout:                   g.ReadTimeout,
		WriteTimeout:                  g.WriteTimeout,
		IdleTimeout:                   g.IdleTimeout,
		MaxRequestBodySize:            g.MaxRequestBodySize,
		DisableKeepalive:              g.DisableKeepalive,
		ReduceMemoryUsage:             true,
		GetOnly:                       g.GETOnly,
		DisableHeaderNamesNormalizing: g.DisableHeaderNamesNormalizing,
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
		log.Infof("%s stopped listening on %s", g.ServerName, g.address)
		return nil
	}
	return err
}

// Use registers global middleware functions to be executed for all routes
func (g *Gonoleks) Use(middlewares ...handlerFunc) IRoutes {
	g.middlewares = append(g.middlewares, middlewares...)
	return g
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

// LoadHTMLFS loads an fs.FS and a slice of patterns
// and associates the result with HTML renderer
func (g *Gonoleks) LoadHTMLFS(fs fs.FS, patterns ...string) error {
	if g.htmlRender == nil {
		g.htmlRender = NewTemplateEngine()
	}
	return g.htmlRender.(*TemplateEngine).LoadFS(fs, patterns...)
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

// printStartupMessage displays server startup information in the console
func (g *Gonoleks) printStartupMessage(addr string) {
	if prefork.IsChild() {
		log.Info("Worker process started", "pid", os.Getpid())
	} else {
		port := addr[strings.LastIndex(addr, ":"):]
		log.Infof("%s started on %s", g.ServerName, port)
	}
}
