package gonoleks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeType(t *testing.T) {
	assert.Equal(t, nodeType(0), static, "static should be 0")
	assert.Equal(t, nodeType(1), root, "root should be 1")
	assert.Equal(t, nodeType(2), param, "param should be 2")
	assert.Equal(t, nodeType(3), catchAll, "catchAll should be 3")
}

func TestNodeStructure(t *testing.T) {
	n := &node{
		path:     "test",
		children: make(map[string]*node),
		nType:    static,
		handlers: handlersChain{func(c *Context) {}},
	}

	assert.Equal(t, "test", n.path, "Path should be 'test'")
	assert.NotNil(t, n.children, "Children map should be initialized")
	assert.Equal(t, static, n.nType, "Node type should be static")
	assert.Len(t, n.handlers, 1, "Handlers chain should have 1 handler")
}

func TestAddRoute(t *testing.T) {
	root := createRootNode()
	handler := func(c *Context) {}

	// Test adding a simple route
	root.addRoute("/test", handlersChain{handler})
	assert.NotNil(t, root.children["test"], "Static node should be created")
	assert.Equal(t, static, root.children["test"].nType, "Node type should be static")
	assert.NotNil(t, root.children["test"].handlers, "Handlers should be set")

	// Test adding a route with parameters
	root.addRoute("/users/:id", handlersChain{handler})
	assert.NotNil(t, root.children["users"], "Static node should be created")
	assert.NotNil(t, root.children["users"].param, "Parameter node should be created")
	assert.Equal(t, ":id", root.children["users"].param.path, "Parameter path should be ':id'")
	assert.Equal(t, param, root.children["users"].param.nType, "Node type should be param")

	// Test adding a route with catch-all parameter
	root.addRoute("/files/*filepath", handlersChain{handler})
	assert.NotNil(t, root.children["files"], "Static node should be created")
	assert.NotNil(t, root.children["files"].param, "Parameter node should be created")
	assert.Equal(t, "*filepath", root.children["files"].param.path, "Parameter path should be '*filepath'")
	assert.Equal(t, catchAll, root.children["files"].param.nType, "Node type should be catchAll")

	// Test adding a route with compound parameters
	root.addRoute("/docs/:file.:ext", handlersChain{handler})
	assert.NotNil(t, root.children["docs"], "Static node should be created")
	assert.NotNil(t, root.children["docs"].children[":file.:ext"], "Compound node should be created")
}

func TestSetHandlers(t *testing.T) {
	n := &node{
		path:     "test",
		children: make(map[string]*node),
		nType:    static,
	}

	handler1 := func(c *Context) {}
	handler2 := func(c *Context) {}
	handlers := handlersChain{handler1, handler2}

	// Test setting handlers on a node without existing handlers
	n.setHandlers(n, handlers, "/test")
	assert.Len(t, n.handlers, 2, "Handlers chain should have 2 handlers")
}

func TestHandleParameterSegment(t *testing.T) {
	paramNames := make(map[string]bool)

	// Test handling a parameter segment
	root1 := createRootNode()
	paramNode := root1.handleParameterSegment(root1, ":id", "/users/:id", paramNames)
	assert.Equal(t, ":id", paramNode.path, "Parameter path should be ':id'")
	assert.Equal(t, param, paramNode.nType, "Node type should be param")
	assert.True(t, paramNames["id"], "Parameter name should be registered")

	// Test handling a catch-all segment (use a different root node)
	root2 := createRootNode()
	catchAllNode := root2.handleParameterSegment(root2, "*filepath", "/files/*filepath", paramNames)
	assert.Equal(t, "*filepath", catchAllNode.path, "Parameter path should be '*filepath'")
	assert.Equal(t, catchAll, catchAllNode.nType, "Node type should be catchAll")

	// Test handling a parameter segment with existing parameter (should not panic if same name)
	assert.NotPanics(t, func() {
		root1.handleParameterSegment(root1, ":id", "/users/:id", paramNames)
	}, "Handling a parameter segment with same name should not panic")

	// Test handling a parameter segment with conflicting parameter (should panic)
	assert.Panics(t, func() {
		root1.handleParameterSegment(root1, ":name", "/users/:name", paramNames)
	}, "Handling a parameter segment with different name should panic")

	// Test handling a catch-all segment with length > 1 (should panic)
	assert.Panics(t, func() {
		root2.handleParameterSegment(root2, "*", "/files/*", paramNames)
	}, "Handling a catch-all segment with length = 1 should panic")
}

func TestHandleStaticSegment(t *testing.T) {
	root := createRootNode()

	// Test handling a static segment
	staticNode := root.handleStaticSegment(root, "users", "/users")
	assert.Equal(t, "users", staticNode.path, "Static path should be 'users'")
	assert.Equal(t, static, staticNode.nType, "Node type should be static")

	// Test handling the same static segment again (should return existing node)
	sameNode := root.handleStaticSegment(root, "users", "/users")
	assert.Same(t, staticNode, sameNode, "Should return the same node for the same segment")

	// Create a node with a parameter child
	nodeWithParam := &node{
		path:     "test",
		children: make(map[string]*node),
		nType:    static,
		param: &node{
			path:     ":id",
			children: make(map[string]*node),
			nType:    param,
		},
	}

	// Test handling a static segment on a node with a parameter (should panic)
	assert.Panics(t, func() {
		root.handleStaticSegment(nodeWithParam, "static", "/test/static")
	}, "Handling a static segment on a node with a parameter should panic")
}

func TestHandleCompoundSegment(t *testing.T) {
	root := createRootNode()
	paramNames := make(map[string]bool)

	// Test handling a compound segment with dot notation
	compoundNode := root.handleCompoundSegment(root, ":file.:ext", "/docs/:file.:ext", paramNames)
	assert.Equal(t, ":file.:ext", compoundNode.path, "Compound path should be ':file.:ext'")
	assert.Equal(t, static, compoundNode.nType, "Node type should be static")
	assert.True(t, paramNames["file"], "Parameter 'file' should be registered")
	assert.True(t, paramNames["ext"], "Parameter 'ext' should be registered")

	// Test handling a compound segment with dash notation
	compoundNode = root.handleCompoundSegment(root, ":from-:to", "/route/:from-:to", paramNames)
	assert.Equal(t, ":from-:to", compoundNode.path, "Compound path should be ':from-:to'")
	assert.Equal(t, static, compoundNode.nType, "Node type should be static")
	assert.True(t, paramNames["from"], "Parameter 'from' should be registered")
	assert.True(t, paramNames["to"], "Parameter 'to' should be registered")
}

func TestExtractParamNames(t *testing.T) {
	paramNames := make(map[string]bool)

	// Test extracting parameter names from a compound segment with dot notation
	extractParamNames(":file.:ext", paramNames)
	assert.True(t, paramNames["file"], "Parameter 'file' should be extracted")
	assert.True(t, paramNames["ext"], "Parameter 'ext' should be extracted")

	// Clear the map
	for k := range paramNames {
		delete(paramNames, k)
	}

	// Test extracting parameter names from a compound segment with dash notation
	extractParamNames(":from-:to", paramNames)
	assert.True(t, paramNames["from"], "Parameter 'from' should be extracted")
	assert.True(t, paramNames["to"], "Parameter 'to' should be extracted")

	// Clear the map
	for k := range paramNames {
		delete(paramNames, k)
	}

	// Test extracting parameter names from a complex compound segment
	extractParamNames(":year.:month.:day-:hour.:minute", paramNames)
	assert.True(t, paramNames["year"], "Parameter 'year' should be extracted")
	assert.True(t, paramNames["month"], "Parameter 'month' should be extracted")
	assert.True(t, paramNames["day"], "Parameter 'day' should be extracted")
	assert.True(t, paramNames["hour"], "Parameter 'hour' should be extracted")
	assert.True(t, paramNames["minute"], "Parameter 'minute' should be extracted")
}

func TestMatchRoute(t *testing.T) {
	root := createRootNode()
	handler1 := func(c *Context) {}
	handler2 := func(c *Context) {}
	handler3 := func(c *Context) {}

	// Add routes to the tree
	root.addRoute("/", handlersChain{handler1})
	root.addRoute("/users", handlersChain{handler1})
	root.addRoute("/users/:id", handlersChain{handler2})
	root.addRoute("/files/*filepath", handlersChain{handler3})
	root.addRoute("/docs/:file.:ext", handlersChain{handler3})
	root.addRoute("/missions/:from-:to", handlersChain{handler3})

	// Test cases
	testCases := []struct {
		name           string
		path           string
		expectedFound  bool
		expectedParams map[string]string
	}{
		{
			name:           "Root path",
			path:           "/",
			expectedFound:  true,
			expectedParams: map[string]string{},
		},
		{
			name:           "Static path",
			path:           "/users",
			expectedFound:  true,
			expectedParams: map[string]string{},
		},
		{
			name:           "Parameter path",
			path:           "/users/123",
			expectedFound:  true,
			expectedParams: map[string]string{"id": "123"},
		},
		{
			name:           "Catch-all path",
			path:           "/files/path/to/file.txt",
			expectedFound:  true,
			expectedParams: map[string]string{"filepath": "path/to/file.txt"},
		},
		{
			name:           "Compound path with dot",
			path:           "/docs/readme.md",
			expectedFound:  true,
			expectedParams: map[string]string{"file": "readme", "ext": "md"},
		},
		{
			name:           "Compound path with dash",
			path:           "/missions/earth-mars",
			expectedFound:  true,
			expectedParams: map[string]string{"from": "earth", "to": "mars"},
		},
		{
			name:           "Non-existent path",
			path:           "/nonexistent",
			expectedFound:  false,
			expectedParams: map[string]string{},
		},
		{
			name:           "Path with trailing slash",
			path:           "/users/",
			expectedFound:  true,
			expectedParams: map[string]string{},
		},
		{
			name:           "Path with multiple slashes",
			path:           "/users//123",
			expectedFound:  true,
			expectedParams: map[string]string{"id": "123"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &Context{
				paramValues: make(map[string]string),
			}

			handlers := root.matchRoute(tc.path, ctx)
			if tc.expectedFound {
				assert.NotNil(t, handlers, "Handlers should be found for path: %s", tc.path)
				for k, v := range tc.expectedParams {
					assert.Equal(t, v, ctx.paramValues[k], "Parameter %s should have value %s", k, v)
				}
			} else {
				assert.Nil(t, handlers, "Handlers should not be found for path: %s", tc.path)
			}
		})
	}
}

func TestMatchCompoundPattern(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		pattern        string
		segment        string
		expectedMatch  bool
		expectedParams map[string]string
	}{
		{
			name:           "File extension pattern",
			pattern:        ":file.:ext",
			segment:        "readme.md",
			expectedMatch:  true,
			expectedParams: map[string]string{"file": "readme", "ext": "md"},
		},
		{
			name:           "From-to pattern",
			pattern:        ":from-:to",
			segment:        "earth-mars",
			expectedMatch:  true,
			expectedParams: map[string]string{"from": "earth", "to": "mars"},
		},
		{
			name:           "Complex pattern",
			pattern:        ":year.:month.:day-:hour.:minute",
			segment:        "2023.01.15-14.30",
			expectedMatch:  true,
			expectedParams: map[string]string{"year": "2023", "month": "01", "day": "15", "hour": "14", "minute": "30"},
		},
		{
			name:           "Non-matching pattern (missing dot)",
			pattern:        ":file.:ext",
			segment:        "readme",
			expectedMatch:  false,
			expectedParams: map[string]string{},
		},
		{
			name:           "Non-matching pattern (missing dash)",
			pattern:        ":from-:to",
			segment:        "earth",
			expectedMatch:  false,
			expectedParams: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &Context{
				paramValues: make(map[string]string),
			}

			match := matchCompoundPattern(tc.pattern, tc.segment, ctx)
			assert.Equal(t, tc.expectedMatch, match, "Match result should be %v for pattern %s and segment %s", tc.expectedMatch, tc.pattern, tc.segment)

			if tc.expectedMatch {
				for k, v := range tc.expectedParams {
					assert.Equal(t, v, ctx.paramValues[k], "Parameter %s should have value %s", k, v)
				}
			}
		})
	}
}

func TestAddRouteConflicts(t *testing.T) {
	// Test cases for route conflicts
	testCases := []struct {
		name          string
		firstRoute    string
		secondRoute   string
		expectedPanic bool
	}{
		{
			name:          "Same static route",
			firstRoute:    "/users",
			secondRoute:   "/users",
			expectedPanic: false,
		},
		{
			name:          "Parameter conflict",
			firstRoute:    "/users/:id",
			secondRoute:   "/users/:name",
			expectedPanic: true,
		},
		{
			name:          "Static and parameter conflict",
			firstRoute:    "/users/:id",
			secondRoute:   "/users/profile",
			expectedPanic: true,
		},
		{
			name:          "Catch-all and parameter conflict",
			firstRoute:    "/files/*filepath",
			secondRoute:   "/files/:name",
			expectedPanic: true,
		},
		{
			name:          "No conflict with different paths",
			firstRoute:    "/users/:id",
			secondRoute:   "/posts/:id",
			expectedPanic: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := createRootNode()
			handler := func(c *Context) {}

			// Add the first route (should always succeed)
			root.addRoute(tc.firstRoute, handlersChain{handler})

			// Try to add the second route
			if tc.expectedPanic {
				assert.Panics(t, func() {
					root.addRoute(tc.secondRoute, handlersChain{handler})
				}, "Adding route %s after %s should panic", tc.secondRoute, tc.firstRoute)
			} else {
				assert.NotPanics(t, func() {
					root.addRoute(tc.secondRoute, handlersChain{handler})
				}, "Adding route %s after %s should not panic", tc.secondRoute, tc.firstRoute)
			}
		})
	}
}

func TestAddRouteCatchAllRestrictions(t *testing.T) {
	root := createRootNode()
	handler := func(c *Context) {}

	// Test that catch-all routes are only allowed at the end of the path
	assert.Panics(t, func() {
		root.addRoute("/files/*filepath/extra", handlersChain{handler})
	}, "Catch-all routes should only be allowed at the end of the path")

	// Test that catch-all routes with no name are valid
	assert.NotPanics(t, func() {
		root.addRoute("/files/*", handlersChain{handler})
	}, "Catch-all routes with no name should be valid")
}

func TestCreateRootNode(t *testing.T) {
	root := createRootNode()
	assert.Equal(t, "/", root.path, "Root path should be '/'")
	assert.Equal(t, nodeType(1), root.nType, "Root type should be root")
	assert.NotNil(t, root.children, "Children map should be initialized")
	assert.Nil(t, root.handlers, "Handlers should be nil")
}
