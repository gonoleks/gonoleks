package gonoleks

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
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
	templates     map[string]*template.Template
	lock          sync.RWMutex
	delims        [2]string
	dir           string
	funcMap       template.FuncMap
	templateCache map[string]*template.Template
}

// NewTemplateEngine creates a new template engine instance
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates:     make(map[string]*template.Template),
		templateCache: make(map[string]*template.Template),
		delims:        [2]string{"{{", "}}"},
		dir:           "",
		funcMap:       make(template.FuncMap),
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
		if err := e.loadFileUnsafe(name, file); err != nil {
			return err
		}
	}

	// Clear cache after loading new templates
	e.clearCacheUnsafe()
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
		if err := e.loadFileUnsafe(name, file); err != nil {
			return err
		}
	}

	// Clear cache after loading new templates
	e.clearCacheUnsafe()
	return nil
}

// loadFileUnsafe loads a single template file without locking (internal use)
func (e *TemplateEngine) loadFileUnsafe(name, path string) error {
	// Read the file content
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Create a new template with the given name
	tmpl := template.New(name)

	// Set custom delimiters if they are different from default
	if e.delims[0] != "{{" || e.delims[1] != "}}" {
		tmpl = tmpl.Delims(e.delims[0], e.delims[1])
	}

	// Set function map if available
	if len(e.funcMap) > 0 {
		tmpl = tmpl.Funcs(e.funcMap)
	}

	// Parse the template content directly from bytes
	tmpl, err = tmpl.Parse(string(content))
	if err != nil {
		return err
	}

	// Store the parsed template
	e.templates[name] = tmpl
	return nil
}

// loadFile loads a single template file (public version with locking)
func (e *TemplateEngine) loadFile(name, path string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.loadFileUnsafe(name, path)
}

// clearCacheUnsafe clears the template cache without locking
func (e *TemplateEngine) clearCacheUnsafe() {
	for k := range e.templateCache {
		delete(e.templateCache, k)
	}
}

// SetTemplate sets a pre-compiled template (for compatibility)
func (e *TemplateEngine) SetTemplate(tmpl any) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Handle different template types
	switch t := tmpl.(type) {
	case *template.Template:
		e.templates["default"] = t
	case string:
		// Parse string as template
		if parsedTmpl := e.parseTemplateUnsafe("default", t); parsedTmpl != nil {
			e.templates["default"] = parsedTmpl
		}
	case []byte:
		// Parse bytes as template
		if parsedTmpl := e.parseTemplateUnsafe("default", string(t)); parsedTmpl != nil {
			e.templates["default"] = parsedTmpl
		}
	}

	// Clear cache after setting new template
	e.clearCacheUnsafe()
}

// parseTemplateUnsafe parses template content without locking
func (e *TemplateEngine) parseTemplateUnsafe(name, content string) *template.Template {
	parsedTmpl := template.New(name)
	if e.delims[0] != "{{" || e.delims[1] != "}}" {
		parsedTmpl = parsedTmpl.Delims(e.delims[0], e.delims[1])
	}
	if len(e.funcMap) > 0 {
		parsedTmpl = parsedTmpl.Funcs(e.funcMap)
	}
	parsedTmpl, err := parsedTmpl.Parse(content)
	if err != nil {
		return nil
	}
	return parsedTmpl
}

// SetFuncMap sets template functions
func (e *TemplateEngine) SetFuncMap(funcMap any) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Convert funcMap to template.FuncMap
	switch fm := funcMap.(type) {
	case template.FuncMap:
		e.funcMap = fm
	case map[string]any:
		// Convert map[string]any to template.FuncMap
		if e.funcMap == nil {
			e.funcMap = make(template.FuncMap, len(fm))
		} else {
			// Clear existing map
			for k := range e.funcMap {
				delete(e.funcMap, k)
			}
		}
		for k, v := range fm {
			e.funcMap[k] = v
		}
	default:
		// For other types, initialize empty map
		if e.funcMap == nil {
			e.funcMap = make(template.FuncMap)
		}
	}

	// Clear cache after changing function map
	e.clearCacheUnsafe()
}

// SetDelims sets the delimiters used for template parsing
func (e *TemplateEngine) SetDelims(left, right string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.delims = [2]string{left, right}
	// Clear cache after changing delimiters
	e.clearCacheUnsafe()
}

// Instance returns a renderer for the specified template
func (e *TemplateEngine) Instance(name string, data any) Render {
	e.lock.RLock()

	// First check direct template lookup
	tmpl, ok := e.templates[name]
	if ok {
		e.lock.RUnlock()
		return &templateRender{
			Template: tmpl,
			Data:     data,
		}
	}

	// Check cache for name with extension
	extendedName := name + ".html"
	if cachedTmpl, exists := e.templateCache[extendedName]; exists {
		e.lock.RUnlock()
		return &templateRender{
			Template: cachedTmpl,
			Data:     data,
		}
	}

	// Try with extension
	tmpl, ok = e.templates[extendedName]
	if ok {
		// Cache the result for future lookups
		e.templateCache[extendedName] = tmpl
		e.lock.RUnlock()
		return &templateRender{
			Template: tmpl,
			Data:     data,
		}
	}

	e.lock.RUnlock()
	return &errorRender{err: errors.Join(ErrTemplateNotFound, errors.New(name))}
}

// templateRender implements the Render interface
type templateRender struct {
	Template *template.Template
	Data     any
}

// Render executes the template with the provided data
func (r *templateRender) Render(w io.Writer) error {
	if r.Template == nil {
		return ErrTemplateNotFound
	}

	// Execute the template with the provided data
	return r.Template.Execute(w, r.Data)
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
	case fmt.Stringer:
		return v.String()
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
