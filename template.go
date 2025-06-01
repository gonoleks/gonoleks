package gonoleks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type HTMLRender interface {
	Instance(name string, data any) Render
}

type Render interface {
	Render(io.Writer) error
}

// TemplateEngine implements a template engine
type TemplateEngine struct {
	templates map[string][]byte
	lock      sync.RWMutex
	delims    [2]string
	dir       string
	funcMap   any
}

// NewTemplateEngine creates a new template engine instance
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: make(map[string][]byte),
		delims:    [2]string{"{{", "}}"},
		dir:       "",
	}
}

// LoadGlob loads templates matching the specified pattern
func (e *TemplateEngine) LoadGlob(pattern string) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Find all files matching the pattern
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	// Store the directory for future reference
	if len(files) > 0 {
		e.dir = filepath.Dir(files[0])
	}

	// Load each file
	for _, file := range files {
		name := filepath.Base(file)
		if err := e.loadFile(name, file); err != nil {
			return err
		}
	}

	return nil
}

// LoadFiles loads templates from the specified files
func (e *TemplateEngine) LoadFiles(files ...string) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	if len(files) == 0 {
		return nil
	}

	// Store the directory for future reference
	e.dir = filepath.Dir(files[0])

	// Load each file
	for _, file := range files {
		name := filepath.Base(file)
		if err := e.loadFile(name, file); err != nil {
			return err
		}
	}

	return nil
}

// loadFile loads a single template file
func (e *TemplateEngine) loadFile(name, path string) error {
	// Read the file content
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Store the template content
	e.templates[name] = content
	return nil
}

// SetTemplate sets a pre-compiled template (for compatibility)
func (e *TemplateEngine) SetTemplate(tmpl any) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Handle different template types
	switch t := tmpl.(type) {
	case string:
		// Store as raw template content
		e.templates["default"] = []byte(t)
	case []byte:
		e.templates["default"] = t
	case map[string]string:
		// Multiple templates
		for name, content := range t {
			e.templates[name] = []byte(content)
		}
	case map[string][]byte:
		// Multiple templates as bytes
		for name, content := range t {
			e.templates[name] = content
		}
	}
}

// SetFuncMap sets template functions
func (e *TemplateEngine) SetFuncMap(funcMap any) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.funcMap = funcMap
}

// SetDelims sets the delimiters used for template parsing
func (e *TemplateEngine) SetDelims(left, right string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.delims = [2]string{left, right}
}

// Instance returns a renderer for the specified template
func (e *TemplateEngine) Instance(name string, data any) Render {
	e.lock.RLock()
	defer e.lock.RUnlock()

	tmpl, ok := e.templates[name]
	if !ok {
		// Try with extension
		tmpl, ok = e.templates[name+".html"]
		if !ok {
			return &errorRender{err: errors.Join(ErrTemplateNotFound, errors.New(name))}
		}
	}

	return &templateRender{
		Template: tmpl,
		Data:     data,
		Delims:   e.delims,
	}
}

// templateRender implements the Render interface
type templateRender struct {
	Template []byte
	Data     any
	Delims   [2]string
}

// Render executes the template with the provided data
func (r *templateRender) Render(w io.Writer) error {
	tmplStr := string(r.Template)

	// Convert data to map if possible
	dataMap, ok := r.Data.(map[string]any)
	if !ok {
		// If data is not a map, try to use it directly
		// This is a simplified approach
		return ErrDataMustBeMapStringAny
	}

	// Simple variable replacement
	for key, value := range dataMap {
		varName := r.Delims[0] + key + r.Delims[1]
		valueStr := toString(value)
		tmplStr = strings.ReplaceAll(tmplStr, varName, valueStr)
	}

	// Write the result
	_, err := w.Write([]byte(tmplStr))
	return err
}

// toString converts a value to string
func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case error:
		return v.Error()
	case nil:
		return ""
	default:
		// Use a bytes buffer to convert to string
		buf := new(bytes.Buffer)
		fmt.Fprint(buf, v)
		return buf.String()
	}
}

// errorRender is a renderer that always returns an error
type errorRender struct {
	err error
}

// Render returns the stored error
func (r *errorRender) Render(w io.Writer) error {
	return r.err
}
