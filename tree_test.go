package gonoleks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeBasics(t *testing.T) {
	// Test node types
	assert.Equal(t, nodeType(0), static, "Static type should be 0")
	assert.Equal(t, nodeType(1), root, "Root type should be 1")
	assert.Equal(t, nodeType(2), param, "Param type should be 2")
	assert.Equal(t, nodeType(3), catchAll, "CatchAll type should be 3")

	// Test node structure
	node := &node{
		path:     "/users",
		nType:    static,
		children: make(map[string]*node),
	}
	assert.Equal(t, "/users", node.path, "Path should be '/users'")
	assert.Equal(t, static, node.nType, "Type should be static")
	assert.NotNil(t, node.children, "Children should not be nil")

	// Test root node creation
	rootNode := createRootNode()
	assert.Equal(t, "/", rootNode.path, "Root path should be '/'")
	assert.Equal(t, root, rootNode.nType, "Root type should be root")
	assert.NotNil(t, rootNode.children, "Children map should be initialized")
	assert.Nil(t, rootNode.handlers, "Handlers should be nil")
}

func TestRouteOperations(t *testing.T) {
	root := createRootNode()
	handler1 := func(c *Context) {}
	handler2 := func(c *Context) {}

	// Test adding basic routes
	root.addRoute("/", handlersChain{handler1})
	root.addRoute("/users", handlersChain{handler1})
	root.addRoute("/users/:id", handlersChain{handler2})
	root.addRoute("/files/*filepath", handlersChain{handler2})

	// Test setting handlers
	root.setHandlers(root, handlersChain{handler1})
	assert.NotNil(t, root.handlers, "Handlers should be set")
	assert.Equal(t, 1, len(root.handlers), "Should have one handler")

	// Test route conflicts
	assert.Panics(t, func() {
		root.addRoute("/users/:name", handlersChain{handler1})
	}, "Parameter conflict should panic")

	// Test catch-all restrictions
	assert.Panics(t, func() {
		root.addRoute("/files/*filepath/extra", handlersChain{handler1})
	}, "Catch-all routes should only be allowed at the end")
}

func TestSegmentHandling(t *testing.T) {
	root := createRootNode()
	paramNames := make(map[string]bool)

	// Test static segment handling
	staticNode := root.handleStaticSegment(root, "users")
	assert.Equal(t, "users", staticNode.path, "Static path should be 'users'")
	assert.Equal(t, static, staticNode.nType, "Node type should be static")

	// Test parameter segment handling
	paramNode := root.handleParameterSegment(root, ":id", "/users/:id", paramNames)
	assert.Equal(t, ":id", paramNode.path, "Parameter path should be ':id'")
	assert.Equal(t, param, paramNode.nType, "Node type should be param")
	assert.True(t, paramNames["id"], "Parameter 'id' should be registered")

	// Test compound segment handling
	compoundNode := root.handleCompoundSegment(root, ":file.:ext", paramNames)
	assert.Equal(t, ":file.:ext", compoundNode.path, "Compound path should be ':file.:ext'")
	assert.True(t, paramNames["file"], "Parameter 'file' should be registered")
	assert.True(t, paramNames["ext"], "Parameter 'ext' should be registered")

	// Test parameter extraction
	paramNames = make(map[string]bool)
	extractParamNames(":from-:to", paramNames)
	assert.True(t, paramNames["from"], "Parameter 'from' should be extracted")
	assert.True(t, paramNames["to"], "Parameter 'to' should be extracted")

	// Test panic conditions
	assert.Panics(t, func() {
		root.handleParameterSegment(root, ":name", "/users/:id", paramNames)
	}, "Handling a parameter segment with different name should panic")
}

func TestRouteMatching(t *testing.T) {
	root := createRootNode()
	handler := func(c *Context) {}

	// Add test routes
	root.addRoute("/", handlersChain{handler})
	root.addRoute("/users", handlersChain{handler})
	root.addRoute("/users/:id", handlersChain{handler})
	root.addRoute("/files/*filepath", handlersChain{handler})
	root.addRoute("/docs/:file.:ext", handlersChain{handler})

	// Test basic matching
	ctx := &Context{paramValues: make(map[string]string)}
	handlers := root.matchRoute("/", ctx)
	assert.NotNil(t, handlers, "Root route should match")

	// Test parameter matching
	ctx = &Context{paramValues: make(map[string]string)}
	handlers = root.matchRoute("/users/123", ctx)
	assert.NotNil(t, handlers, "Parameter route should match")
	assert.Equal(t, "123", ctx.paramValues["id"], "Parameter should be extracted")

	// Test catch-all matching
	ctx = &Context{paramValues: make(map[string]string)}
	handlers = root.matchRoute("/files/path/to/file.txt", ctx)
	assert.NotNil(t, handlers, "Catch-all route should match")
	assert.Equal(t, "path/to/file.txt", ctx.paramValues["filepath"], "Catch-all should be extracted")

	// Test compound pattern matching
	ctx = &Context{paramValues: make(map[string]string)}
	match := matchCompoundPattern(":file.:ext", "readme.md", ctx)
	assert.True(t, match, "Compound pattern should match")
	assert.Equal(t, "readme", ctx.paramValues["file"], "File parameter should be extracted")
	assert.Equal(t, "md", ctx.paramValues["ext"], "Extension parameter should be extracted")

	// Test non-matching cases
	ctx = &Context{paramValues: make(map[string]string)}
	handlers = root.matchRoute("/nonexistent", ctx)
	assert.Nil(t, handlers, "Non-existent route should not match")

	// Test compound pattern non-matching
	ctx = &Context{paramValues: make(map[string]string)}
	match = matchCompoundPattern(":file.:ext", "readme", ctx)
	assert.False(t, match, "Pattern without extension should not match")
}
