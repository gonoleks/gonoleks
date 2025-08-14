package gonoleks

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/template/*.tmpl
var testTemplateFS embed.FS

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

func TestTemplateEngine_Basic(t *testing.T) {
	engine := NewTemplateEngine()
	assert.NotNil(t, engine)
	assert.Equal(t, [2]string{"", ""}, engine.delims)
	assert.Nil(t, engine.funcMap)
	assert.Nil(t, engine.set)

	// Test setting delimiters
	engine.SetDelims("<%", "%>")
	assert.Equal(t, [2]string{"<%", "%>"}, engine.delims)

	// Test setting function map
	funcMap := map[string]any{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
	}
	engine.SetFuncMap(funcMap)
	assert.Equal(t, funcMap, engine.funcMap)
}

func TestTemplateEngine_Loading(t *testing.T) {
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

	// Test loading with glob pattern
	engine2 := NewTemplateEngine()
	pattern := filepath.Join(tempDir, "*.jet")
	err = engine2.LoadGlob(pattern)
	assert.NoError(t, err)
	assert.NotNil(t, engine2.set)

	// Test invalid glob pattern
	err = engine2.LoadGlob("[invalid")
	assert.Error(t, err)
}

func TestTemplateEngine_Instance(t *testing.T) {
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

	// Test requesting nonexistent template
	render = engine.Instance("nonexistent.jet", map[string]any{"name": "World"})
	assert.NotNil(t, render)

	buf.Reset()
	err = render.Render(&buf)
	assert.Error(t, err)
}

func TestTemplateEngine_Advanced(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Test custom delimiters
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

	// Test function map
	engine2 := NewTemplateEngine()
	engine2.SetFuncMap(map[string]any{
		"upper": strings.ToUpper,
	})
	err = engine2.LoadFiles(filepath.Join(tempDir, "greeting.jet"))
	require.NoError(t, err)

	render = engine2.Instance("greeting.jet", map[string]any{
		"greeting": "hello",
		"name":     "world",
	})

	buf.Reset()
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "HELLO world", buf.String())
}

func TestJetRender(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine := NewTemplateEngine()
	err := engine.LoadFiles(filepath.Join(tempDir, "welcome.jet"))
	require.NoError(t, err)

	// Test with complex data
	data := map[string]any{
		"user": map[string]any{"name": "Arman"},
		"site": "Gonoleks",
	}
	render := engine.Instance("welcome.jet", data)
	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "Welcome Arman to Gonoleks", buf.String())

	// Test with nil template
	nilRender := &jetRender{
		template: nil,
		data:     map[string]any{"name": "World"},
	}
	buf.Reset()
	err = nilRender.Render(&buf)
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
		{"relative paths", "docs/templates", "docs", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSubPath(tt.child, tt.parent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplateEngine_TestdataTemplates(t *testing.T) {
	engine := NewTemplateEngine()
	engine.SetDelims("{[{", "}]}")

	// Load testdata template files
	testdataDir := filepath.Join("testdata", "template")
	pattern := filepath.Join(testdataDir, "*.tmpl")
	err := engine.LoadGlob(pattern)
	require.NoError(t, err)

	// Test hello template
	render := engine.Instance("hello.tmpl", map[string]any{"name": "Arman"})
	assert.NotNil(t, render)

	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "<h1>Hello Arman</h1>", buf.String())

	// Test with function map
	engine.SetFuncMap(map[string]any{
		"formatAsDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
	})
	err = engine.LoadGlob(pattern)
	require.NoError(t, err)

	testTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	render = engine.Instance("raw.tmpl", map[string]any{"now": testTime})
	buf.Reset()
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "Date: 2025-01-15", buf.String())
}

func TestTemplateEngine_LoadFS(t *testing.T) {
	engine := NewTemplateEngine()
	engine.SetDelims("{[{", "}]}")

	subFS, err := fs.Sub(testTemplateFS, "testdata/template")
	require.NoError(t, err)

	err = engine.LoadFS(subFS, "*.tmpl")
	assert.NoError(t, err)
	assert.NotNil(t, engine.set)

	render := engine.Instance("hello.tmpl", map[string]any{"name": "FS Test"})
	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "<h1>Hello FS Test</h1>", buf.String())
}
