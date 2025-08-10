package gonoleks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestTemplateEngine_TestdataTemplates(t *testing.T) {
	engine := NewTemplateEngine()

	// Set custom delimiters to match the testdata templates
	engine.SetDelims("{[{", "}]}")

	// Load testdata template files
	testdataDir := filepath.Join("testdata", "template")
	pattern := filepath.Join(testdataDir, "*.tmpl")
	err := engine.LoadGlob(pattern)
	require.NoError(t, err)

	t.Run("hello template", func(t *testing.T) {
		render := engine.Instance("hello.tmpl", map[string]any{
			"name": "Arman",
		})
		assert.NotNil(t, render)

		var buf bytes.Buffer
		if renderErr := render.Render(&buf); renderErr != nil {
			err = renderErr
		}
		assert.NoError(t, err)
		assert.Equal(t, "<h1>Hello Arman</h1>", buf.String())
	})

	t.Run("raw template with function", func(t *testing.T) {
		// Add a custom function for date formatting
		engine.SetFuncMap(map[string]any{
			"formatAsDate": func(t time.Time) string {
				return t.Format("2006-01-02")
			},
		})

		// Reload templates with the new function map
		err = engine.LoadGlob(pattern)
		require.NoError(t, err)

		testTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		render := engine.Instance("raw.tmpl", map[string]any{
			"now": testTime,
		})
		assert.NotNil(t, render)

		var buf bytes.Buffer
		err := render.Render(&buf)
		assert.NoError(t, err)
		assert.Equal(t, "Date: 2025-01-15", buf.String())
	})
}

func TestTemplateEngine_TestdataTemplates_LoadFiles(t *testing.T) {
	engine := NewTemplateEngine()
	engine.SetDelims("{[{", "}]}")

	// Test loading specific testdata template files
	testdataDir := filepath.Join("testdata", "template")
	files := []string{
		filepath.Join(testdataDir, "hello.tmpl"),
		filepath.Join(testdataDir, "raw.tmpl"),
	}

	err := engine.LoadFiles(files...)
	assert.NoError(t, err)
	assert.NotNil(t, engine.set)

	// Test hello template
	render := engine.Instance("hello.tmpl", map[string]any{
		"name": "World",
	})

	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	assert.Equal(t, "<h1>Hello World</h1>", buf.String())
}

func TestTemplateEngine_TestdataTemplates_FileExistence(t *testing.T) {
	testdataDir := filepath.Join("testdata", "template")

	// Test that template files exist
	expectedFiles := []string{"hello.tmpl", "raw.tmpl"}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(testdataDir, filename)
		t.Run("file_exists_"+filename, func(t *testing.T) {
			_, err := os.Stat(filePath)
			assert.NoError(t, err, "Template file %s should exist", filename)
		})
	}
}

func TestTemplateEngine_TestdataTemplates_InvalidData(t *testing.T) {
	engine := NewTemplateEngine()
	engine.SetDelims("{[{", "}]}")

	testdataDir := filepath.Join("testdata", "template")
	pattern := filepath.Join(testdataDir, "*.tmpl")
	err := engine.LoadGlob(pattern)
	require.NoError(t, err)

	t.Run("hello template with missing data", func(t *testing.T) {
		render := engine.Instance("hello.tmpl", map[string]any{})
		assert.NotNil(t, render)

		var buf bytes.Buffer
		err := render.Render(&buf)
		// Should error because the 'name' field is not provided
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no field or method 'name'")
	})

	t.Run("raw template with missing function", func(t *testing.T) {
		// Don't set the formatAsDate function
		testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		render := engine.Instance("raw.tmpl", map[string]any{
			"now": testTime,
		})
		assert.NotNil(t, render)

		var buf bytes.Buffer
		err := render.Render(&buf)
		// Should error because formatAsDate function is not defined
		assert.Error(t, err)
	})
}

func TestTemplateEngine_TestdataTemplates_WithDifferentDelimiters(t *testing.T) {
	engine := NewTemplateEngine()

	// Test with default delimiters (should not work with testdata templates)
	testdataDir := filepath.Join("testdata", "template")
	pattern := filepath.Join(testdataDir, "*.tmpl")
	err := engine.LoadGlob(pattern)
	require.NoError(t, err)

	render := engine.Instance("hello.tmpl", map[string]any{
		"name": "Test",
	})

	var buf bytes.Buffer
	err = render.Render(&buf)
	assert.NoError(t, err)
	// Should render literally because delimiters don't match
	assert.Equal(t, "<h1>Hello {[{.name}]}</h1>", buf.String())
}

func TestTemplateEngine_TestdataTemplates_ContentValidation(t *testing.T) {
	testdataDir := filepath.Join("testdata", "template")

	t.Run("hello template content", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(testdataDir, "hello.tmpl"))
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "{[{.name}]}", "hello.tmpl should contain the name variable")
		assert.Contains(t, contentStr, "<h1>", "hello.tmpl should contain HTML h1 tag")
		assert.Contains(t, contentStr, "</h1>", "hello.tmpl should contain closing HTML h1 tag")
	})

	t.Run("raw template content", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(testdataDir, "raw.tmpl"))
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "{[{.now | formatAsDate}]}", "raw.tmpl should contain the now variable with formatAsDate function")
		assert.Contains(t, contentStr, "Date:", "raw.tmpl should contain 'Date:' prefix")
	})
}
