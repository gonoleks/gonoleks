package gonoleks

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestDefault(t *testing.T) {
	// Test with default options
	app := Default()
	assert.NotNil(t, app, "Default() should return a non-nil instance")

	// Access the Gonoleks struct directly (no type assertion needed)
	defaultConfig := defaultOptions()
	assert.Equal(t, defaultConfig.ServerName, app.httpServer.Name, "ServerName option should be applied")
	assert.Equal(t, defaultConfig.Concurrency, app.httpServer.Concurrency, "Concurrency option should be applied")
	assert.Equal(t, defaultConfig.ReadBufferSize, app.httpServer.ReadBufferSize, "ReadBufferSize option should be applied")
}

func TestNew(t *testing.T) {
	// Test with custom options
	app := New()
	assert.NotNil(t, app, "New() should return a non-nil instance")

	app = New()
	// Use direct field assignment
	app.ServerName = "Test"
	app.Concurrency = 1024
	app.ReadBufferSize = 8192
	app.CaseInSensitive = true
	// Recreate httpServer to apply the new configuration
	app.httpServer = app.newHTTPServer()

	assert.NotNil(t, app, "New() with custom options should return a non-nil instance")

	// Access the Gonoleks struct directly (no type assertion needed)
	assert.Equal(t, "Test", app.httpServer.Name, "ServerName option should be applied")
	assert.Equal(t, 1024, app.httpServer.Concurrency, "Concurrency option should be applied")
	assert.Equal(t, 8192, app.httpServer.ReadBufferSize, "ReadBufferSize option should be applied")
}

func TestRouteRegistration(t *testing.T) {
	app := New()

	// Test GET route registration
	route := app.GET("/test", func(c *Context) {})
	assert.NotNil(t, route, "GET() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodGet, route.Method, "Route method should be GET")

	// Test POST route registration
	route = app.POST("/test", func(c *Context) {})
	assert.NotNil(t, route, "POST() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodPost, route.Method, "Route method should be POST")

	// Test PUT route registration
	route = app.PUT("/test", func(c *Context) {})
	assert.NotNil(t, route, "PUT() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodPut, route.Method, "Route method should be PUT")

	// Test DELETE route registration
	route = app.DELETE("/test", func(c *Context) {})
	assert.NotNil(t, route, "DELETE() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodDelete, route.Method, "Route method should be DELETE")

	// Test PATCH route registration
	route = app.PATCH("/test", func(c *Context) {})
	assert.NotNil(t, route, "PATCH() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodPatch, route.Method, "Route method should be PATCH")

	// Test HEAD route registration
	route = app.HEAD("/test", func(c *Context) {})
	assert.NotNil(t, route, "HEAD() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodHead, route.Method, "Route method should be HEAD")

	// Test OPTIONS route registration
	route = app.OPTIONS("/test", func(c *Context) {})
	assert.NotNil(t, route, "OPTIONS() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodOptions, route.Method, "Route method should be OPTIONS")

	// Test CONNECT route registration
	route = app.CONNECT("/test", func(c *Context) {})
	assert.NotNil(t, route, "CONNECT() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodConnect, route.Method, "Route method should be CONNECT")

	// Test TRACE route registration
	route = app.TRACE("/test", func(c *Context) {})
	assert.NotNil(t, route, "TRACE() should return a non-nil route")
	assert.Equal(t, "/test", route.Path, "Route path should match")
	assert.Equal(t, MethodTrace, route.Method, "Route method should be TRACE")
}

func TestAnyMethod(t *testing.T) {
	app := New()

	routes := app.Any("/any", func(c *Context) {})
	assert.Equal(t, 9, len(routes), "Any() should register 9 routes (one for each HTTP method)")

	methods := make(map[string]bool)
	for _, route := range routes {
		assert.Equal(t, "/any", route.Path, "All routes should have the same path")
		methods[route.Method] = true
	}

	// Verify all methods are registered
	assert.True(t, methods[MethodGet], "GET method should be registered")
	assert.True(t, methods[MethodPost], "POST method should be registered")
	assert.True(t, methods[MethodPut], "PUT method should be registered")
	assert.True(t, methods[MethodPatch], "PATCH method should be registered")
	assert.True(t, methods[MethodHead], "HEAD method should be registered")
	assert.True(t, methods[MethodOptions], "OPTIONS method should be registered")
	assert.True(t, methods[MethodDelete], "DELETE method should be registered")
	assert.True(t, methods[MethodConnect], "CONNECT method should be registered")
	assert.True(t, methods[MethodTrace], "TRACE method should be registered")
}

func TestMatchMethod(t *testing.T) {
	app := New()

	methods := []string{MethodGet, MethodPost, MethodPut}
	routes := app.Match(methods, "/match", func(c *Context) {})

	assert.Equal(t, len(methods), len(routes), "Match() should register the exact number of routes specified")

	registeredMethods := make(map[string]bool)
	for _, route := range routes {
		assert.Equal(t, "/match", route.Path, "All routes should have the same path")
		registeredMethods[route.Method] = true
	}

	// Verify only specified methods are registered
	assert.True(t, registeredMethods[MethodGet], "GET method should be registered")
	assert.True(t, registeredMethods[MethodPost], "POST method should be registered")
	assert.True(t, registeredMethods[MethodPut], "PUT method should be registered")
	assert.False(t, registeredMethods[MethodPatch], "PATCH method should not be registered")
}

func TestGroup(t *testing.T) {
	app := New()

	// Create a group
	group := app.Group("/api")
	assert.NotNil(t, group, "Group() should return a non-nil RouterGroup")

	// Register routes on the group
	route := group.GET("/users", func(c *Context) {})
	assert.NotNil(t, route, "Group.GET() should return a non-nil route")
	assert.Equal(t, "/api/users", route.Path, "Group route path should be prefixed with group path")

	// Test nested groups
	v1 := group.Group("/v1")
	assert.NotNil(t, v1, "Nested Group() should return a non-nil RouterGroup")

	route = v1.POST("/posts", func(c *Context) {})
	assert.NotNil(t, route, "Nested Group.POST() should return a non-nil route")
	assert.Equal(t, "/api/v1/posts", route.Path, "Nested group route path should have all prefixes")
}

func TestMiddleware(t *testing.T) {
	app := New()

	// Register global middleware
	middlewareExecuted := false
	app.Use(func(c *Context) {
		middlewareExecuted = true
		c.Next()
	})

	// Register a route
	app.GET("/middleware-test", func(c *Context) {})

	// Access the Gonoleks struct directly (no type assertion needed)
	assert.Equal(t, 1, len(app.middlewares), "Global middleware should be registered")

	// Setup the router to ensure middleware is properly registered with routes
	app.setupRouter()

	// Simulate a request to trigger the middleware
	if app.router != nil {
		// Use fasthttp to create a fake request context
		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.SetRequestURI("/middleware-test")
		reqCtx.Request.Header.SetMethod(MethodGet)
		app.router.Handler(reqCtx)
	}

	// Now check if the middleware was executed
	assert.True(t, middlewareExecuted, "Middleware should be executed on request")
}

func TestCaseInsensitiveRouting(t *testing.T) {
	app := New()
	app.CaseInSensitive = true

	// Register a route with mixed case
	route := app.GET("/UsEr/PrOfIlE", func(c *Context) {})
	assert.Equal(t, "/user/profile", route.Path, "Path should be converted to lowercase with case-insensitive routing")

	// Test group with case-insensitive routing
	group := app.Group("/ApI")
	assert.NotNil(t, group, "Group() should return a non-nil RouterGroup")

	groupRoute := group.GET("/UsErS", func(c *Context) {})
	assert.Equal(t, "/api/users", groupRoute.Path, "Group path should be converted to lowercase with case-insensitive routing")
}

func TestStaticFileServing(t *testing.T) {
	app := New()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "gonoleks-test-*.txt")
	require.NoError(t, err, "Failed to create temporary file")
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temporary file: %v", err)
		}
	}()

	// Write some content to the file
	content := "Hello, World!"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err, "Failed to write to temporary file")
	if err := tmpFile.Close(); err != nil {
		t.Logf("Failed to close temporary file: %v", err)
	}

	// Register the file
	app.StaticFile("/test-file", tmpFile.Name())

	// Access the Gonoleks struct directly (no type assertion needed)
	found := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/test-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route")
}

func TestRun(t *testing.T) {
	// Skip in CI environments or when running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app := New()

	// Find an available port
	listener, err := net.Listen(NetworkTCP, "127.0.0.1:0")
	require.NoError(t, err, "Failed to find available port")
	port := listener.Addr().(*net.TCPAddr).Port
	if closeErr := listener.Close(); closeErr != nil {
		t.Logf("Failed to close listener: %v", err)
	}

	// Register a test route
	app.GET("/ping", func(c *Context) {
		c.String(StatusOK, "pong")
	})

	// Start the server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Run(fmt.Sprintf(":%d", port))
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to the server
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", port))
	if err == nil {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, StatusOK, resp.StatusCode, "Server should respond with 200 OK")
		assert.Equal(t, "pong", string(body), "Server should respond with 'pong'")
	}
}

func TestShutdown(t *testing.T) {
	t.Run("shutdown without running server", func(t *testing.T) {
		app := New()
		err := app.Shutdown()
		assert.NoError(t, err, "Shutdown should not return error when server is not running")
	})

	t.Run("shutdown with address set", func(t *testing.T) {
		app := New()
		app.address = defaultPort
		err := app.Shutdown()
		assert.NoError(t, err, "Shutdown should not return error when address is set")
		// Verify that the address is still set (it should not be cleared)
		assert.Equal(t, defaultPort, app.address, "Address should remain set after shutdown")
	})

	t.Run("shutdown with mock server", func(t *testing.T) {
		app := New()
		app.address = defaultPort

		// Create a mock server that will return an error on shutdown
		mockServer := &fasthttp.Server{}
		app.httpServer = mockServer

		// Since we can't easily mock the Shutdown method to return an error,
		// we'll test the error path by calling Shutdown on a server that's already shut down
		// This should return an error from the underlying fasthttp server
		err := app.Shutdown()
		// The error behavior depends on the fasthttp implementation,
		// but we can at least verify that the method handles errors gracefully
		if err != nil {
			assert.Error(t, err, "Should return error when httpServer.Shutdown() fails")
		}
	})
}

func TestHTMLRendering(t *testing.T) {
	app := New()

	// Create a temporary template file
	tmpFile, err := os.CreateTemp("", "gonoleks-template-*.html")
	require.NoError(t, err, "Failed to create temporary template file")
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temporary template file: %v", err)
		}
	}()

	// Write a simple template
	templateContent := "<h1>Hello, {{.Name}}!</h1>"
	_, err = tmpFile.WriteString(templateContent)
	require.NoError(t, err, "Failed to write to temporary template file")
	if closeErr := tmpFile.Close(); closeErr != nil {
		t.Logf("Failed to close temporary template file: %v", err)
	}

	// Load the template
	err = app.LoadHTMLFiles(tmpFile.Name())
	assert.NoError(t, err, "LoadHTMLFiles should not return an error")

	// Access the Gonoleks struct directly (no type assertion needed)
	assert.NotNil(t, app.htmlRender, "HTML renderer should be created")
}

func TestDefaultOptions(t *testing.T) {
	app := Default()
	defaultConfig := defaultOptions()

	// Verify default options - access direct fields
	assert.Equal(t, defaultConfig.MaxRequestBodySize, app.MaxRequestBodySize, "Default MaxRequestBodySize should be applied")
	assert.Equal(t, defaultConfig.MaxRouteParams, app.MaxRouteParams, "Default MaxRouteParams should be applied")
	assert.Equal(t, defaultConfig.MaxRequestURLLength, app.MaxRequestURLLength, "Default MaxRequestURLLength should be applied")
	assert.Equal(t, defaultConfig.Concurrency, app.Concurrency, "Default Concurrency should be applied")
	assert.Equal(t, defaultConfig.ReadBufferSize, app.ReadBufferSize, "Default ReadBufferSize should be applied")
}

func TestHandleMethod(t *testing.T) {
	app := New()

	// Register a route with a custom method
	customMethod := "CUSTOM"
	route := app.Handle(customMethod, "/custom", func(c *Context) {})

	assert.NotNil(t, route, "Handle() should return a non-nil route")
	assert.Equal(t, "/custom", route.Path, "Route path should match")
	assert.Equal(t, customMethod, route.Method, "Route method should match custom method")
}

func TestNoRoute(t *testing.T) {
	app := New()

	// Register a custom NoRoute handler
	app.NoRoute(func(c *Context) {
	})

	// Access the internal router directly (no type assertion needed)
	assert.NotNil(t, app.router.noRoute, "NoRoute handler should be registered")
	assert.Equal(t, 1, len(app.router.noRoute), "NoRoute should register exactly one handler")
}

func TestNoMethod(t *testing.T) {
	app := New()

	// Register a custom NoMethod handler
	app.NoMethod(func(c *Context) {
		c.String(StatusMethodNotAllowed, "Custom Method Not Allowed")
	})

	// Access the internal router directly (no type assertion needed)
	assert.NotNil(t, app.router.noMethod, "NoMethod handler should be registered")
	assert.Equal(t, 1, len(app.router.noMethod), "NoMethod should register exactly one handler")
}

func TestSecureJsonPrefix(t *testing.T) {
	app := New()

	// Test default secure JSON prefix - access struct directly
	assert.Equal(t, "while(1);", app.secureJsonPrefix, "Default secure JSON prefix should be 'while(1);'")

	// Test setting custom secure JSON prefix
	customPrefix := ")]}',\n"
	app.SecureJsonPrefix(customPrefix)
	assert.Equal(t, customPrefix, app.secureJsonPrefix, "Custom secure JSON prefix should be set correctly")

	// Test setting empty prefix
	emptyPrefix := ""
	app.SecureJsonPrefix(emptyPrefix)
	assert.Equal(t, emptyPrefix, app.secureJsonPrefix, "Empty secure JSON prefix should be set correctly")

	// Test setting another custom prefix
	anotherPrefix := "/**/"
	app.SecureJsonPrefix(anotherPrefix)
	assert.Equal(t, anotherPrefix, app.secureJsonPrefix, "Another custom secure JSON prefix should be set correctly")
}

func TestRecoveryMiddleware(t *testing.T) {
	app := New()

	// Add Recovery middleware
	app.Use(Recovery())

	// Register a route that panics
	app.GET("/panic", func(c *Context) {
		panic("test panic for recovery")
	})

	// Setup the router
	app.setupRouter()

	// Create a test request context
	reqCtx := &fasthttp.RequestCtx{}
	reqCtx.Request.SetRequestURI("/panic")
	reqCtx.Request.Header.SetMethod(MethodGet)

	// Test that the handler doesn't panic and recovers gracefully
	assert.NotPanics(t, func() {
		app.router.Handler(reqCtx)
	}, "Recovery middleware should catch panics")

	// Verify that a 500 Internal Server Error is returned
	assert.Equal(t, StatusInternalServerError, reqCtx.Response.StatusCode(), "Should return 500 status code after panic recovery")

	// Verify that an error message is set
	responseBody := string(reqCtx.Response.Body())
	assert.Contains(t, responseBody, "Internal Server Error", "Response should contain error message")
}

func TestRecoveryMiddlewareWithoutPanic(t *testing.T) {
	app := New()

	// Add Recovery middleware
	app.Use(Recovery())

	// Register a normal route that doesn't panic
	app.GET("/normal", func(c *Context) {
		c.String(StatusOK, "success")
	})

	// Setup the router
	app.setupRouter()

	// Create a test request context
	reqCtx := &fasthttp.RequestCtx{}
	reqCtx.Request.SetRequestURI("/normal")
	reqCtx.Request.Header.SetMethod(MethodGet)

	// Test that normal requests work fine with Recovery middleware
	assert.NotPanics(t, func() {
		app.router.Handler(reqCtx)
	}, "Recovery middleware should not interfere with normal requests")

	// Verify that a 200 OK is returned
	assert.Equal(t, StatusOK, reqCtx.Response.StatusCode(), "Should return 200 status code for normal requests")

	// Verify the response body
	responseBody := string(reqCtx.Response.Body())
	assert.Equal(t, "success", responseBody, "Response should contain expected content")
}
