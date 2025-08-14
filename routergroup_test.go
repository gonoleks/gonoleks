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

	// Test middleware registration and inheritance
	parent := app.Group("/api")
	parent.Use(func(c *Context) { c.Next() })

	child := parent.Group("/v1")
	child.Use(func(c *Context) { c.Next() })

	// Test isolation between groups
	other := app.Group("/other")
	other.Use(func(c *Context) { c.Next() })

	assert.Equal(t, 1, len(parent.middlewares))
	assert.Equal(t, 2, len(child.middlewares))
	assert.Equal(t, 1, len(other.middlewares))

	// Test method chaining
	chained := parent.Use(func(c *Context) { c.Next() }).Group("/chained")
	assert.Equal(t, "/api/chained", chained.BasePath())
}

func testMiddleware1(c *Context) { c.Next() }
func testMiddleware2(c *Context) { c.Next() }

func TestGroup(t *testing.T) {
	app := New()

	// Test basic group creation and route registration
	group := app.Group("/api")
	assert.NotNil(t, group)
	assert.Equal(t, "/api", group.prefix)

	route := group.GET("/users", func(c *Context) {})
	assert.Equal(t, "/api/users", route.Path)

	// Test nested groups
	v1 := group.Group("/v1")
	route = v1.POST("/posts", func(c *Context) {})
	assert.Equal(t, "/api/v1/posts", route.Path)

	// Test group with handlers
	groupWithHandlers := app.Group("/v2", testMiddleware1, testMiddleware2)
	assert.Equal(t, 2, len(groupWithHandlers.middlewares))

	// Test BasePath
	assert.Equal(t, "/api/v1", v1.BasePath())
}

func TestRouteGroupPaths(t *testing.T) {
	app := New()

	// Test empty prefix
	emptyGroup := app.Group("")
	route := emptyGroup.GET("/test", func(c *Context) {})
	assert.Equal(t, "/test", route.Path)

	// Test slash handling
	group := app.Group("/api")
	route = group.GET("/users", func(c *Context) {})
	assert.Equal(t, "/api/users", route.Path)

	// Test case insensitive
	app.CaseInSensitive = true
	caseGroup := app.Group("/ApI")
	route = caseGroup.GET("/UsErS", func(c *Context) {})
	assert.Equal(t, "/api/users", route.Path)

	// Test deep nesting
	api := app.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")
	route = users.GET("/profile", func(c *Context) {})
	assert.Equal(t, "/api/v1/users/profile", route.Path)
}

func TestRouteGroupMethods(t *testing.T) {
	app := New()
	group := app.Group("/api")

	// Test basic HTTP methods
	getRoute := group.GET("/get", func(c *Context) {})
	postRoute := group.POST("/post", func(c *Context) {})
	assert.Equal(t, "/api/get", getRoute.Path)
	assert.Equal(t, MethodGet, getRoute.Method)
	assert.Equal(t, "/api/post", postRoute.Path)
	assert.Equal(t, MethodPost, postRoute.Method)

	// Test custom method
	customRoute := group.Handle("CUSTOM", "/custom", func(c *Context) {})
	assert.Equal(t, "/api/custom", customRoute.Path)
	assert.Equal(t, "CUSTOM", customRoute.Method)

	// Test Any method
	anyRoutes := group.Any("/any", func(c *Context) {})
	assert.Equal(t, 9, len(anyRoutes))

	// Test Match method
	matchRoutes := group.Match([]string{MethodGet, MethodPost}, "/match", func(c *Context) {})
	assert.Equal(t, 2, len(matchRoutes))
}

func TestRouteGroupStaticFileServing(t *testing.T) {
	app := New()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "gonoleks-static-test-*.txt")
	require.NoError(t, err, "Failed to create temporary file")
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temporary file: %v", err)
		}
	}()

	// Write some content to the file
	content := "Hello from static file!"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err, "Failed to write to temporary file")
	if closeErr := tmpFile.Close(); closeErr != nil {
		t.Logf("Failed to close temporary file: %v", err)
	}

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "gonoleks-static-dir-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temporary directory: %v", err)
		}
	}()

	// Test StaticFile with basic group
	api := app.Group("/api")
	api.StaticFile("/test-file", tmpFile.Name())

	// Verify StaticFile route registration
	found := false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/test-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route with group prefix")

	// Test nested groups
	v1 := api.Group("/v1")
	v1.StaticFile("/nested-file", tmpFile.Name())

	// Verify nested StaticFile route registration
	found = false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/v1/nested-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route with nested group prefix")

	// Test case insensitive
	app.CaseInSensitive = true
	caseGroup := app.Group("/API/V2")
	caseGroup.StaticFile("/Test-File", tmpFile.Name())

	// Verify case insensitive route registration
	found = false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api/v2/test-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route with lowercase group prefix")

	// Test special characters
	specialGroup := app.Group("/api-v1_test")
	specialGroup.StaticFile("/test-file_123", tmpFile.Name())

	// Verify special characters route registration
	found = false
	for _, route := range app.registeredRoutes {
		if route.Path == "/api-v1_test/test-file_123" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFile should register a GET route with special characters preserved")

	// Test StaticFileFS with embedded FS
	fsGroup := app.Group("/fs")
	fsGroup.StaticFileFS("/embedded-file", "testdata/test_file.txt", testFS)

	// Verify StaticFileFS route registration
	found = false
	for _, route := range app.registeredRoutes {
		if route.Path == "/fs/embedded-file" && route.Method == MethodGet {
			found = true
			break
		}
	}
	assert.True(t, found, "StaticFileFS should register a GET route with group prefix")

	// Test Static directory serving
	assets := app.Group("/assets")
	assets.Static("/files", tmpDir)

	// Verify Static directory route registration
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

	// Test StaticFS with embedded FS
	embedded := app.Group("/embedded")
	embedded.StaticFS("/files", testFS)

	// Verify StaticFS route registration
	foundBase = false
	foundWildcard = false
	for _, route := range app.registeredRoutes {
		if route.Path == "/embedded/files" && route.Method == MethodGet {
			foundBase = true
		}
		if route.Path == "/embedded/files/*" && route.Method == MethodGet {
			foundWildcard = true
		}
	}
	assert.True(t, foundBase, "StaticFS should register a base GET route with group prefix")
	assert.True(t, foundWildcard, "StaticFS should register a wildcard GET route with group prefix")

	// Test StaticFS with DirFS
	dirfs := app.Group("/dirfs")
	dirfs.StaticFS("/files", os.DirFS(tmpDir))

	// Verify DirFS route registration
	foundBase = false
	foundWildcard = false
	for _, route := range app.registeredRoutes {
		if route.Path == "/dirfs/files" && route.Method == MethodGet {
			foundBase = true
		}
		if route.Path == "/dirfs/files/*" && route.Method == MethodGet {
			foundWildcard = true
		}
	}
	assert.True(t, foundBase, "StaticFS with DirFS should register a base GET route with group prefix")
	assert.True(t, foundWildcard, "StaticFS with DirFS should register a wildcard GET route with group prefix")

	// Test root path group
	rootGroup := app.Group("/")
	rootGroup.Static("/root", tmpDir)

	// Verify root path route registration (root group creates double slashes)
	foundBase = false
	foundWildcard = false
	for _, route := range app.registeredRoutes {
		if route.Path == "//root" && route.Method == MethodGet {
			foundBase = true
		}
		if route.Path == "//root/*" && route.Method == MethodGet {
			foundWildcard = true
		}
	}
	assert.True(t, foundBase, "Static should register a base GET route for root path")
	assert.True(t, foundWildcard, "Static should register a wildcard GET route for root path")
}
