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
	templates map[string]*template.Template
	lock      sync.RWMutex
	delims    [2]string
	dir       string
	funcMap   template.FuncMap
}

// NewTemplateEngine creates a new template engine instance
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: make(map[string]*template.Template),
		delims:    [2]string{"{{", "}}"},
		dir:       "",
		funcMap:   make(template.FuncMap),
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

	// Parse the template content
	tmpl, err = tmpl.Parse(string(content))
	if err != nil {
		return err
	}

	// Store the parsed template
	e.templates[name] = tmpl
	return nil
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
		parsedTmpl := template.New("default")
		if e.delims[0] != "{{" || e.delims[1] != "}}" {
			parsedTmpl = parsedTmpl.Delims(e.delims[0], e.delims[1])
		}
		if len(e.funcMap) > 0 {
			parsedTmpl = parsedTmpl.Funcs(e.funcMap)
		}
		parsedTmpl, err := parsedTmpl.Parse(t)
		if err == nil {
			e.templates["default"] = parsedTmpl
		}
	case []byte:
		// Parse bytes as template
		parsedTmpl := template.New("default")
		if e.delims[0] != "{{" || e.delims[1] != "}}" {
			parsedTmpl = parsedTmpl.Delims(e.delims[0], e.delims[1])
		}
		if len(e.funcMap) > 0 {
			parsedTmpl = parsedTmpl.Funcs(e.funcMap)
		}
		parsedTmpl, err := parsedTmpl.Parse(string(t))
		if err == nil {
			e.templates["default"] = parsedTmpl
		}
	}
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
			e.funcMap = make(template.FuncMap)
		}
		for k, v := range fm {
			e.funcMap[k] = v
		}
	default:
		// For other types, try to use as is
		if e.funcMap == nil {
			e.funcMap = make(template.FuncMap)
		}
	}
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
	}
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
