package gonoleks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTemplate1                = `Hello, {{name}}!`
	testTemplate2                = `Welcome {{user.name}} to {{site}}`
	testTemplate3                = `{{upper(greeting)}} {{name}}`
	testTemplateWithCustomDelims = `<% greeting %> <% name %>`
)

func setupTestTemplates(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "gonoleks_template_test")
	require.NoError(t, err)

	// Create test template files
	templates := map[string]string{
		"hello.jet":    testTemplate1,
		"welcome.jet":  testTemplate2,
		"greeting.jet": testTemplate3,
		"custom.jet":   testTemplateWithCustomDelims,
	}

	for filename, content := range templates {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0o644)
		require.NoError(t, err)
	}

	cleanup := func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}

	return tempDir, cleanup
}

func TestNewTemplateEngine(t *testing.T) {
	engine := NewTemplateEngine()
	assert.NotNil(t, engine)
	assert.Equal(t, [2]string{"", ""}, engine.delims)
	assert.Nil(t, engine.funcMap)
	assert.Nil(t, engine.set)
}

func TestTemplateEngine_SetDelims(t *testing.T) {
	engine := NewTemplateEngine()

	// Test setting delimiters
	engine.SetDelims("<%", "%>")
	assert.Equal(t, [2]string{"<%", "%>"}, engine.delims)

	// Test setting different delimiters
	engine.SetDelims("{{", "}}")
	assert.Equal(t, [2]string{"{{", "}}"}, engine.delims)
}

func TestTemplateEngine_SetFuncMap(t *testing.T) {
	engine := NewTemplateEngine()

	funcMap := map[string]any{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
	}

	engine.SetFuncMap(funcMap)
	assert.Equal(t, funcMap, engine.funcMap)
}

func TestTemplateEngine_LoadFiles(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine := NewTemplateEngine()

	// Test loading files
	files := []string{
		filepath.Join(tempDir, "hello.jet"),
		filepath.Join(tempDir, "welcome.jet"),
	}

	err := engine.LoadFiles(files...)
	assert.NoError(t, err)
	assert.NotNil(t, engine.set)

	// Test loading empty files list
	err = engine.LoadFiles()
	assert.NoError(t, err)
}

func TestTemplateEngine_LoadGlob(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine := NewTemplateEngine()

	// Test loading with glob pattern
	pattern := filepath.Join(tempDir, "*.jet")
	err := engine.LoadGlob(pattern)
	assert.NoError(t, err)
	assert.NotNil(t, engine.set)

	// Test invalid glob pattern
	err = engine.LoadGlob("[invalid")
	assert.Error(t, err)
}

func TestTemplateEngine_Instance_WithoutSet(t *testing.T) {
	engine := NewTemplateEngine()

	// Test instance creation without loading templates
	render := engine.Instance("nonexistent", map[string]any{"name": "World"})
	assert.NotNil(t, render)

	// Test rendering should return error
	var buf bytes.Buffer
	err := render.Render(&buf)
	assert.Error(t, err)
	assert.Equal(t, ErrTemplateNotFound, err)
}

func TestTemplateEngine_Instance_Basic(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine := NewTemplateEngine()
	err := engine.LoadFiles(filepath.Join(tempDir, "hello.jet"))
	require.NoError(t, err)

	// Test basic template rendering
	render := engine.Instance("hello.jet", map[string]any{"name": "World"})
	assert.NotNil(t, render)

	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World!", buf.String())
}

func TestTemplateEngine_Instance_NonexistentTemplate(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine := NewTemplateEngine()
	err := engine.LoadFiles(filepath.Join(tempDir, "hello.jet"))
	require.NoError(t, err)

	// Test requesting nonexistent template
	render := engine.Instance("nonexistent.jet", map[string]any{"name": "World"})
	assert.NotNil(t, render)

	// Should return error when rendering
	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.Error(t, err)
}

func TestTemplateEngine_WithCustomDelimiters(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine := NewTemplateEngine()
	engine.SetDelims("<%", "%>")

	err := engine.LoadFiles(filepath.Join(tempDir, "custom.jet"))
	require.NoError(t, err)

	render := engine.Instance("custom.jet", map[string]any{
		"greeting": "Hello",
		"name":     "World",
	})

	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World", buf.String())
}

func TestTemplateEngine_WithFuncMap(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine := NewTemplateEngine()
	engine.SetFuncMap(map[string]any{
		"upper": strings.ToUpper,
	})

	err := engine.LoadFiles(filepath.Join(tempDir, "greeting.jet"))
	require.NoError(t, err)

	render := engine.Instance("greeting.jet", map[string]any{
		"greeting": "hello",
		"name":     "world",
	})

	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "HELLO world", buf.String())
}

func TestJetRender_Render_WithMapData(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine := NewTemplateEngine()
	err := engine.LoadFiles(filepath.Join(tempDir, "welcome.jet"))
	require.NoError(t, err)

	data := map[string]any{
		"user": map[string]any{"name": "Arman"},
		"site": "Gonoleks",
	}

	render := engine.Instance("welcome.jet", data)
	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "Welcome Arman to Gonoleks", buf.String())
}

func TestJetRender_Render_WithNilTemplate(t *testing.T) {
	render := &jetRender{
		template: nil,
		data:     map[string]any{"name": "World"},
	}

	var buf bytes.Buffer
	err := render.Render(&buf)
	assert.Error(t, err)
	assert.Equal(t, ErrTemplateNotFound, err)
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name     string
		child    string
		parent   string
		expected bool
	}{
		{"child is subdirectory", "/home/user/docs", "/home/user", true},
		{"child is same as parent", "/home/user", "/home/user", true},
		{"child is not subdirectory", "/home/other", "/home/user", false},
		{"parent is subdirectory of child", "/home", "/home/user", false},
		{"relative paths", "docs/templates", "docs", true},
		{"relative paths not sub", "other/templates", "docs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSubPath(tt.child, tt.parent)
			assert.Equal(t, tt.expected, result)
		})
	}
}
