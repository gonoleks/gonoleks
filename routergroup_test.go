package gonoleks

import (
	"embed"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/test_file.txt
var testFS embed.FS

func TestRouteGroupMiddleware(t *testing.T) {
	app := New()

	// Test group-specific middleware
	group := app.Group("/api")
	group.Use(func(c *Context) {
		c.Next()
	})

	// Register a route on the group
	group.GET("/test", func(c *Context) {})

	// Verify middleware is registered on the group
	assert.Equal(t, 1, len(group.middlewares), "Group middleware should be registered")
}

func TestRouteGroupMiddlewareInheritance(t *testing.T) {
	app := New()

	// Add global middleware
	app.Use(func(c *Context) {
		c.Next()
	})

	// Create parent group with middleware
	parentGroup := app.Group("/api")
	parentGroup.Use(func(c *Context) {
		c.Next()
	})

	// Create child group with additional middleware
	childGroup := parentGroup.Group("/v1")
	childGroup.Use(func(c *Context) {
		c.Next()
	})

	// Register a route on child group
	childGroup.GET("/test", func(c *Context) {})

	// Verify middleware inheritance
	assert.Equal(t, 1, len(app.middlewares), "App should have 1 global middleware")
	assert.Equal(t, 1, len(parentGroup.middlewares), "Parent group should have 1 middleware")
	assert.Equal(t, 2, len(childGroup.middlewares), "Child group should inherit parent middleware plus its own")
}

func TestRouteGroupMiddlewareIsolation(t *testing.T) {
	app := New()

	// Create two separate groups
	group1 := app.Group("/api1")
	group1.Use(func(c *Context) {
		c.Next()
	})

	group2 := app.Group("/api2")
	group2.Use(func(c *Context) {
		c.Next()
	})
	group2.Use(func(c *Context) {
		c.Next()
	})

	// Verify middleware isolation
	assert.Equal(t, 1, len(group1.middlewares), "Group1 should have 1 middleware")
	assert.Equal(t, 2, len(group2.middlewares), "Group2 should have 2 middlewares")

	// Verify they don't affect each other
	group1.Use(func(c *Context) {
		c.Next()
	})
	assert.Equal(t, 2, len(group1.middlewares), "Group1 should now have 2 middlewares")
	assert.Equal(t, 2, len(group2.middlewares), "Group2 should still have 2 middlewares")
}

func TestRouteGroupUseReturnsInterface(t *testing.T) {
	app := New()
	group := app.Group("/api")

	// Test that Use returns IRoutes interface
	result := group.Use(func(c *Context) {
		c.Next()
	})

	assert.NotNil(t, result, "Use should return non-nil IRoutes")
	assert.Implements(t, (*IRoutes)(nil), result, "Use should return IRoutes interface")

	// Test method chaining
	chainedGroup := group.Use(func(c *Context) {
		c.Next()
	}).Group("/v1")

	assert.NotNil(t, chainedGroup, "Method chaining should work")
	assert.Equal(t, "/api/v1", chainedGroup.BasePath(), "Chained group should have correct base path")
}

func TestRouteGroupEmptyMiddleware(t *testing.T) {
	app := New()
	group := app.Group("/api")

	// Test empty middleware slice
	group.Use()
	assert.Equal(t, 0, len(group.middlewares), "Empty Use() should not add any middleware")

	// Test nil middleware
	group.Use(nil)
	assert.Equal(t, 1, len(group.middlewares), "Nil middleware should still be added")
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

func TestRouteGroupEmptyPrefix(t *testing.T) {
	app := New()

	// Test group with empty prefix
	group := app.Group("")
	route := group.GET("/test", func(c *Context) {})

	assert.NotNil(t, route, "Route should be created even with empty group prefix")
	assert.Equal(t, "/test", route.Path, "Route path should not have any prefix")
}

func TestRouteGroupSlashHandling(t *testing.T) {
	app := New()

	tests := []struct {
		name        string
		groupPrefix string
		routePath   string
		expected    string
	}{
		{"Normal case", "/api", "/users", "/api/users"},
		{"Group without leading slash", "api", "/users", "api/users"},
		{"Route without leading slash", "/api", "users", "/apiusers"},
		{"Both without leading slash", "api", "users", "apiusers"},
		{"Group with trailing slash", "/api/", "/users", "/api//users"},
		{"Route with leading slash on trailing group", "/api/", "/users", "/api//users"},
		{"Multiple slashes", "/api//", "//users", "/api////users"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			group := app.Group(test.groupPrefix)
			route := group.GET(test.routePath, func(c *Context) {})
			assert.Equal(t, test.expected, route.Path, "Route path should match the concatenated prefix and path")
		})
	}
}

func TestDeepNestedRouteGroups(t *testing.T) {
	app := New()

	// Create deeply nested groups
	api := app.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")
	profile := users.Group("/profile")

	route := profile.GET("/settings", func(c *Context) {})

	assert.NotNil(t, route, "Route should be created in deeply nested group")
	assert.Equal(t, "/api/v1/users/profile/settings", route.Path, "Route path should have all nested prefixes")
}

func TestRouteGroupCaseInsensitive(t *testing.T) {
	app := New()
	app.CaseInSensitive = true

	// Test group with case-insensitive routing
	group := app.Group("/ApI")
	assert.NotNil(t, group, "Group() should return a non-nil RouterGroup")

	groupRoute := group.GET("/UsErS", func(c *Context) {})
	assert.Equal(t, "/api/users", groupRoute.Path, "Group path should be converted to lowercase with case-insensitive routing")
}

func TestRouteGroupBasePath(t *testing.T) {
	app := New()

	// Test root group
	rootGroup := app.Group("")
	assert.Equal(t, "", rootGroup.BasePath(), "Root group should have empty base path")

	// Test simple group
	apiGroup := app.Group("/api")
	assert.Equal(t, "/api", apiGroup.BasePath(), "API group should have /api base path")

	// Test nested groups
	v1Group := apiGroup.Group("/v1")
	assert.Equal(t, "/api/v1", v1Group.BasePath(), "Nested group should have combined base path")

	// Test deeply nested groups
	usersGroup := v1Group.Group("/users")
	profileGroup := usersGroup.Group("/profile")
	assert.Equal(t, "/api/v1/users/profile", profileGroup.BasePath(), "Deeply nested group should have full combined base path")
}

func TestRouteGroupHandle(t *testing.T) {
	app := New()
	group := app.Group("/api")

	// Test custom method
	customMethod := "CUSTOM"
	route := group.Handle(customMethod, "/custom", func(c *Context) {})

	assert.NotNil(t, route, "Handle() should return a non-nil route")
	assert.Equal(t, "/api/custom", route.Path, "Route path should have group prefix")
	assert.Equal(t, customMethod, route.Method, "Route method should match custom method")
}

func TestRouteGroupTrailingSlashNormalization(t *testing.T) {
	app := New()
	group := app.Group("/api")

	// Register route with trailing slash
	route := group.GET("/users/", func(c *Context) {})
	assert.Equal(t, "/api/users/", route.Path, "Route with trailing slash should preserve it")

	// Verify both routes are registered (with and without trailing slash)
	foundWithSlash := false
	foundWithoutSlash := false
	for _, registeredRoute := range app.registeredRoutes {
		if registeredRoute.Path == "/api/users/" && registeredRoute.Method == MethodGet {
			foundWithSlash = true
		}
		if registeredRoute.Path == "/api/users" && registeredRoute.Method == MethodGet {
			foundWithoutSlash = true
		}
	}
	assert.True(t, foundWithSlash, "Route with trailing slash should be registered")
	assert.True(t, foundWithoutSlash, "Route without trailing slash should also be registered")
}

func TestRouteGroupAnyMethod(t *testing.T) {
	app := New()
	group := app.Group("/api")

	routes := group.Any("/any", func(c *Context) {})
	assert.Equal(t, 9, len(routes), "Any() should register 9 routes (one for each HTTP method)")

	methods := make(map[string]bool)
	for _, route := range routes {
		assert.Equal(t, "/api/any", route.Path, "All routes should have the group prefix")
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

func TestRouteGroupMatchMethod(t *testing.T) {
	app := New()
	group := app.Group("/api")

	methods := []string{MethodGet, MethodPost, MethodPut}
	routes := group.Match(methods, "/match", func(c *Context) {})

	assert.Equal(t, len(methods), len(routes), "Match() should register the exact number of routes specified")

	registeredMethods := make(map[string]bool)
	for _, route := range routes {
		assert.Equal(t, "/api/match", route.Path, "All routes should have the group prefix")
		registeredMethods[route.Method] = true
	}

	// Verify only specified methods are registered
	assert.True(t, registeredMethods[MethodGet], "GET method should be registered")
	assert.True(t, registeredMethods[MethodPost], "POST method should be registered")
	assert.True(t, registeredMethods[MethodPut], "PUT method should be registered")
	assert.False(t, registeredMethods[MethodPatch], "PATCH method should not be registered")
}

func TestRouteGroupMatchWithEmptyMethods(t *testing.T) {
	app := New()
	group := app.Group("/api")

	// Test with empty methods slice
	routes := group.Match([]string{}, "/empty", func(c *Context) {})
	assert.Equal(t, 0, len(routes), "Match() with empty methods should register no routes")
}

func TestRouteGroupMatchWithDuplicateMethods(t *testing.T) {
	app := New()
	group := app.Group("/api")

	// Test with duplicate methods
	methods := []string{MethodGet, MethodGet, MethodPost}
	routes := group.Match(methods, "/duplicate", func(c *Context) {})
	assert.Equal(t, 3, len(routes), "Match() should register routes for all methods, including duplicates")
}

func TestRouteGroupHTTPMethods(t *testing.T) {
	app := New()
	group := app.Group("/api")

	// Test all HTTP methods on route groups
	tests := []struct {
		method   string
		expected string
	}{
		{"GET", MethodGet},
		{"POST", MethodPost},
		{"PUT", MethodPut},
		{"DELETE", MethodDelete},
		{"PATCH", MethodPatch},
		{"HEAD", MethodHead},
		{"OPTIONS", MethodOptions},
		{"CONNECT", MethodConnect},
		{"TRACE", MethodTrace},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			var route *Route
			switch test.method {
			case "GET":
				route = group.GET("/test", func(c *Context) {})
			case "POST":
				route = group.POST("/test", func(c *Context) {})
			case "PUT":
				route = group.PUT("/test", func(c *Context) {})
			case "DELETE":
				route = group.DELETE("/test", func(c *Context) {})
			case "PATCH":
				route = group.PATCH("/test", func(c *Context) {})
			case "HEAD":
				route = group.HEAD("/test", func(c *Context) {})
			case "OPTIONS":
				route = group.OPTIONS("/test", func(c *Context) {})
			case "CONNECT":
				route = group.CONNECT("/test", func(c *Context) {})
			case "TRACE":
				route = group.TRACE("/test", func(c *Context) {})
			}

			assert.NotNil(t, route, "%s() should return a non-nil route", test.method)
			assert.Equal(t, "/api/test", route.Path, "Route path should be prefixed with group path")
			assert.Equal(t, test.expected, route.Method, "Route method should match")
		})
	}
}

func TestRouteGroupStaticFile(t *testing.T) {
	app := New()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "gonoleks-group-test-*.txt")
	require.NoError(t, err, "Failed to create temporary file")
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temporary file: %v", err)
		}
	}()

	// Write some content to the file
	content := "Hello from route group!"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err, "Failed to write to temporary file")
	if err := tmpFile.Close(); err != nil {
		t.Logf("Failed to close temporary file: %v", err)
	}

	// Create a route group and register the static file
	api := app.Group("/api/v1")
	api.StaticFile("/test-file", tmpFile.Name())

	// Verify that the route was registered with the correct prefix
	found := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/v1/test-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route with group prefix")
}

func TestNestedRouteGroupStatic(t *testing.T) {
	app := New()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "gonoleks-nested-test-*.txt")
	require.NoError(t, err, "Failed to create temporary file")
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temporary file: %v", err)
		}
	}()

	// Write some content to the file
	content := "Hello from nested route group!"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err, "Failed to write to temporary file")
	if err := tmpFile.Close(); err != nil {
		t.Logf("Failed to close temporary file: %v", err)
	}

	// Create nested route groups and register the static file
	api := app.Group("/api")
	v1 := api.Group("/v1")
	v1.StaticFile("/nested-file", tmpFile.Name())

	// Verify that the route was registered with the correct nested prefix
	found := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/v1/nested-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route with nested group prefix")
}

func TestRouteGroupStaticWithCaseInsensitive(t *testing.T) {
	app := New()
	app.CaseInSensitive = true

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "gonoleks-case-test-*.txt")
	require.NoError(t, err, "Failed to create temporary file")
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temporary file: %v", err)
		}
	}()

	// Write some content to the file
	content := "Hello from case insensitive route group!"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err, "Failed to write to temporary file")
	if err := tmpFile.Close(); err != nil {
		t.Logf("Failed to close temporary file: %v", err)
	}

	// Create a route group with mixed case and register the static file
	api := app.Group("/API/V1")
	api.StaticFile("/Test-File", tmpFile.Name())

	// Verify that the route was registered with lowercase path
	found := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/v1/test-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route with lowercase group prefix")
}

func TestRouteGroupStaticWithSpecialCharacters(t *testing.T) {
	app := New()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "gonoleks-special-test-*.txt")
	require.NoError(t, err, "Failed to create temporary file")
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temporary file: %v", err)
		}
	}()

	// Write some content to the file
	content := "Hello from special characters!"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err, "Failed to write to temporary file")
	if err := tmpFile.Close(); err != nil {
		t.Logf("Failed to close temporary file: %v", err)
	}

	// Create a route group with special characters
	group := app.Group("/api-v1_test")
	group.StaticFile("/test-file_123", tmpFile.Name())

	// Verify that the route was registered with special characters preserved
	found := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api-v1_test/test-file_123" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route with special characters preserved")
}

func TestRouteGroupStaticFileFS(t *testing.T) {
	app := New()

	// Create a route group and register static file from embedded FS
	api := app.Group("/api")
	api.StaticFileFS("/embedded-file", "testdata/test_file.txt", testFS)

	// Verify that the route was registered with the correct prefix
	found := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/embedded-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFileFS should register a GET route with group prefix")
}

func TestRouteGroupStatic(t *testing.T) {
	app := New()

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "gonoleks-static-test-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temporary directory: %v", err)
		}
	}()

	// Create a route group and register static file serving
	assets := app.Group("/assets")
	assets.Static("/files", tmpDir)

	// Verify that the static routes were registered with the correct prefix
	foundBase := false
	foundWildcard := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/assets/files" && route.Method == MethodGet {
			foundBase = true
		}
		if route.Path == "/assets/files/*" && route.Method == MethodGet {
			foundWildcard = true
		}
	}
	assert.True(t, foundBase, "Static should register a base GET route with group prefix")
	assert.True(t, foundWildcard, "Static should register a wildcard GET route with group prefix")
}

func TestRouteGroupStaticDirectory(t *testing.T) {
	app := New()

	// Create a temporary directory with a file for testing
	tmpDir, err := os.MkdirTemp("", "gonoleks-dir-test-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temporary directory: %v", err)
		}
	}()

	// Create a file in the directory
	tmpFile, err := os.CreateTemp(tmpDir, "test-*.txt")
	require.NoError(t, err, "Failed to create temporary file in directory")
	_, err = tmpFile.WriteString("Hello from directory!")
	require.NoError(t, err, "Failed to write to temporary file")
	if err := tmpFile.Close(); err != nil {
		t.Logf("Failed to close temporary file: %v", err)
	}

	// Create nested route groups and register static directory serving
	api := app.Group("/api")
	v1 := api.Group("/v1")
	v1.Static("/files", tmpDir)

	// Verify that the static routes were registered with the correct nested prefix
	foundBase := false
	foundWildcard := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/v1/files" && route.Method == MethodGet {
			foundBase = true
		}
		if route.Path == "/api/v1/files/*" && route.Method == MethodGet {
			foundWildcard = true
		}
	}
	assert.True(t, foundBase, "Static should register a base GET route with nested group prefix")
	assert.True(t, foundWildcard, "Static should register a wildcard GET route with nested group prefix")
}

func TestRouteGroupStaticWithRootPath(t *testing.T) {
	app := New()

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "gonoleks-root-test-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temporary directory: %v", err)
		}
	}()

	// Create a root path group and register static file serving
	rootGroup := app.Group("/")
	rootGroup.Static("/", tmpDir)

	// Verify that the static routes were registered correctly for root path
	foundBase := false
	foundWildcard := false
	for _, route := range app.registeredRoutes {
		if route.Path == "//" && route.Method == MethodGet {
			foundBase = true
		}
		if route.Path == "///*" && route.Method == MethodGet {
			foundWildcard = true
		}
	}
	assert.True(t, foundBase, "Static should register a base GET route for root path")
	assert.True(t, foundWildcard, "Static should register a wildcard GET route for root path")
}

func TestRouteGroupStaticFS(t *testing.T) {
	app := New()

	// Create a route group and register static files from embedded FS
	assets := app.Group("/assets")
	assets.StaticFS("/embedded", testFS)

	// Verify that the static routes were registered with the correct prefix
	foundBase := false
	foundWildcard := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/assets/embedded" && route.Method == MethodGet {
			foundBase = true
		}
		if route.Path == "/assets/embedded/*" && route.Method == MethodGet {
			foundWildcard = true
		}
	}
	assert.True(t, foundBase, "StaticFS should register a base GET route with group prefix")
	assert.True(t, foundWildcard, "StaticFS should register a wildcard GET route with group prefix")
}

func TestRouteGroupStaticFSWithDirFS(t *testing.T) {
	app := New()

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "gonoleks-dirfs-test-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temporary directory: %v", err)
		}
	}()

	// Create a file in the directory
	tmpFile, err := os.CreateTemp(tmpDir, "test-*.txt")
	require.NoError(t, err, "Failed to create temporary file in directory")
	_, err = tmpFile.WriteString("Hello from DirFS!")
	require.NoError(t, err, "Failed to write to temporary file")
	if err := tmpFile.Close(); err != nil {
		t.Logf("Failed to close temporary file: %v", err)
	}

	// Create a route group and register static files from os.DirFS
	api := app.Group("/api")
	api.StaticFS("/files", os.DirFS(tmpDir))

	// Verify that the static routes were registered with the correct prefix
	foundBase := false
	foundWildcard := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/files" && route.Method == MethodGet {
			foundBase = true
		}
		if route.Path == "/api/files/*" && route.Method == MethodGet {
			foundWildcard = true
		}
	}
	assert.True(t, foundBase, "StaticFS with DirFS should register a base GET route with group prefix")
	assert.True(t, foundWildcard, "StaticFS with DirFS should register a wildcard GET route with group prefix")
}
