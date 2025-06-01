package gonoleks

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTemplateEngine(t *testing.T) {
	engine := NewTemplateEngine()
	
	assert.NotNil(t, engine, "Engine should not be nil")
	assert.Empty(t, engine.templates, "Templates map should be empty")
	assert.Equal(t, [2]string{"{{", "}}"}, engine.delims, "Default delimiters should be {{ and }}")
	assert.Empty(t, engine.dir, "Directory should be empty")
}

func TestSetDelims(t *testing.T) {
	engine := NewTemplateEngine()
	
	engine.SetDelims("<<", ">>")
	
	assert.Equal(t, [2]string{"<<", ">>"}, engine.delims, "Delimiters should be updated")
}

func TestLoadFiles(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()
	
	file1Path := filepath.Join(tempDir, "test1.html")
	file2Path := filepath.Join(tempDir, "test2.html")
	
	err := os.WriteFile(file1Path, []byte("<h1>{{title}}</h1>"), 0644)
	require.NoError(t, err, "Failed to create test file 1")
	
	err = os.WriteFile(file2Path, []byte("<p>{{content}}</p>"), 0644)
	require.NoError(t, err, "Failed to create test file 2")
	
	// Test loading files
	engine := NewTemplateEngine()
	err = engine.LoadFiles(file1Path, file2Path)
	
	assert.NoError(t, err, "LoadFiles should not return an error")
	assert.Len(t, engine.templates, 2, "Should have loaded 2 templates")
	assert.Equal(t, []byte("<h1>{{title}}</h1>"), engine.templates["test1.html"], "Template content should match")
	assert.Equal(t, []byte("<p>{{content}}</p>"), engine.templates["test2.html"], "Template content should match")
	assert.Equal(t, tempDir, engine.dir, "Directory should be set correctly")
}

func TestLoadGlob(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()
	
	file1Path := filepath.Join(tempDir, "test1.html")
	file2Path := filepath.Join(tempDir, "test2.html")
	file3Path := filepath.Join(tempDir, "other.txt")
	
	err := os.WriteFile(file1Path, []byte("<h1>{{title}}</h1>"), 0644)
	require.NoError(t, err, "Failed to create test file 1")
	
	err = os.WriteFile(file2Path, []byte("<p>{{content}}</p>"), 0644)
	require.NoError(t, err, "Failed to create test file 2")
	
	err = os.WriteFile(file3Path, []byte("Not a template"), 0644)
	require.NoError(t, err, "Failed to create test file 3")
	
	// Test loading with glob
	engine := NewTemplateEngine()
	err = engine.LoadGlob(filepath.Join(tempDir, "*.html"))
	
	assert.NoError(t, err, "LoadGlob should not return an error")
	assert.Len(t, engine.templates, 2, "Should have loaded 2 templates")
	assert.Equal(t, []byte("<h1>{{title}}</h1>"), engine.templates["test1.html"], "Template content should match")
	assert.Equal(t, []byte("<p>{{content}}</p>"), engine.templates["test2.html"], "Template content should match")
	assert.NotContains(t, engine.templates, "other.txt", "Should not load non-matching files")
	assert.Equal(t, tempDir, engine.dir, "Directory should be set correctly")
}

func TestLoadGlobError(t *testing.T) {
	engine := NewTemplateEngine()
	err := engine.LoadGlob("[invalid-pattern")
	
	assert.Error(t, err, "LoadGlob should return an error for invalid pattern")
}

func TestLoadFilesEmpty(t *testing.T) {
	engine := NewTemplateEngine()
	err := engine.LoadFiles()
	
	assert.NoError(t, err, "LoadFiles with empty list should not return an error")
	assert.Empty(t, engine.templates, "Templates map should remain empty")
}

func TestInstance(t *testing.T) {
	engine := NewTemplateEngine()
	
	// Add a template directly to the map
	engine.templates["test.html"] = []byte("<h1>{{title}}</h1>")
	
	// Test with existing template
	data := map[string]any{"title": "Hello World"}
	renderer := engine.Instance("test.html", data)
	
	assert.NotNil(t, renderer, "Renderer should not be nil")
	
	// Test with non-existent template
	renderer = engine.Instance("nonexistent", data)
	
	// Should return an error renderer
	var buf bytes.Buffer
	err := renderer.Render(&buf)
	assert.Error(t, err, "Rendering non-existent template should return an error")
	assert.Contains(t, err.Error(), "nonexistent", "Error should mention the template name")
}

func TestInstanceWithExtension(t *testing.T) {
	engine := NewTemplateEngine()
	
	// Add a template directly to the map
	engine.templates["test.html"] = []byte("<h1>{{title}}</h1>")
	
	// Test with name without extension
	data := map[string]any{"title": "Hello World"}
	renderer := engine.Instance("test", data)
	
	assert.NotNil(t, renderer, "Renderer should not be nil")
	
	var buf bytes.Buffer
	err := renderer.Render(&buf)
	assert.NoError(t, err, "Rendering should not return an error")
	assert.Equal(t, "<h1>Hello World</h1>", buf.String(), "Rendered content should match")
}

func TestTemplateRender(t *testing.T) {
	// Create a template renderer directly
	renderer := &templateRender{
		Template: []byte("<h1>{{title}}</h1><p>{{content}}</p>"),
		Data:     map[string]any{"title": "Hello", "content": "World"},
		Delims:   [2]string{"{{", "}}"},
	}
	
	var buf bytes.Buffer
	err := renderer.Render(&buf)
	
	assert.NoError(t, err, "Rendering should not return an error")
	assert.Equal(t, "<h1>Hello</h1><p>World</p>", buf.String(), "Rendered content should match")
}

func TestTemplateRenderInvalidData(t *testing.T) {
	// Create a template renderer with non-map data
	renderer := &templateRender{
		Template: []byte("<h1>{{title}}</h1>"),
		Data:     "not a map", // Invalid data type
		Delims:   [2]string{"{{", "}}"},
	}
	
	var buf bytes.Buffer
	err := renderer.Render(&buf)
	
	assert.Error(t, err, "Rendering with invalid data should return an error")
	assert.Equal(t, ErrDataMustBeMapStringAny, err, "Error should be ErrDataMustBeMapStringAny")
}

func TestToString(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected string
	}{
		{"String", "hello", "hello"},
		{"Bytes", []byte("world"), "world"},
		{"Error", errors.New("test error"), "test error"},
		{"Nil", nil, ""},
		{"Integer", 42, "42"},
		{"Float", 3.14, "3.14"},
		{"Boolean", true, "true"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := toString(tc.input)
			assert.Equal(t, tc.expected, result, "toString should convert correctly")
		})
	}
}

func TestErrorRender(t *testing.T) {
	testErr := errors.New("test error")
	renderer := &errorRender{err: testErr}
	
	var buf bytes.Buffer
	err := renderer.Render(&buf)
	
	assert.Equal(t, testErr, err, "Error renderer should return the stored error")
	assert.Empty(t, buf.String(), "Buffer should remain empty")
}

func TestSetTemplateAndFuncMap(t *testing.T) {
	engine := NewTemplateEngine()
	
	// These methods don't do anything, but we should test they don't crash
	engine.SetTemplate("dummy")
	engine.SetFuncMap(map[string]any{"func": func() {}})
	
	// No assertions needed as these methods are no-ops
	assert.NotNil(t, engine, "Engine should still exist after calling no-op methods")
}

func TestLoadFileError(t *testing.T) {
	engine := NewTemplateEngine()
	
	// Try to load a non-existent file
	err := engine.loadFile("test.html", "/nonexistent/path/to/file.html")
	
	assert.Error(t, err, "loadFile should return an error for non-existent file")
}
