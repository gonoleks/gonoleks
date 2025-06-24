package gonoleks

import (
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/CloudyKit/jet/v6"
)

// HTMLRender interface defines the contract for HTML template rendering
type HTMLRender interface {
	Instance(name string, data any) Render
}

// Render interface defines the contract for rendering templates
type Render interface {
	Render(w io.Writer) error
}

// TemplateEngine implements HTMLRender using Jet template engine
type TemplateEngine struct {
	set           *jet.Set
	delims        [2]string
	funcMap       map[string]any
	templateCache sync.Map
	mu            sync.RWMutex
}

// jetRender implements Render interface for Jet templates
type jetRender struct {
	template *jet.Template
	data     any
}

// Pool for VarMap reuse to reduce allocations
var varMapPool = sync.Pool{
	New: func() interface{} {
		return make(jet.VarMap)
	},
}

// NewTemplateEngine creates a new Jet-based template engine
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{}
}

// SetDelims sets the template delimiters
func (e *TemplateEngine) SetDelims(left, right string) {
	e.mu.Lock()
	e.delims = [2]string{left, right}
	if e.set != nil {
		e.recreateSet()
	}
	e.mu.Unlock()
}

// SetFuncMap sets the function map for templates
func (e *TemplateEngine) SetFuncMap(funcMap map[string]any) {
	e.mu.Lock()
	e.funcMap = funcMap
	if e.set != nil {
		e.addFunctionsToSet()
	}
	e.mu.Unlock()
}

// LoadGlob loads templates using glob pattern
func (e *TemplateEngine) LoadGlob(pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	return e.LoadFiles(files...)
}

// LoadFiles loads templates from specified files
func (e *TemplateEngine) LoadFiles(files ...string) error {
	if len(files) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Find the common root directory for all template files
	var rootDir string
	if len(files) > 0 {
		// Get the directory from the first file
		firstDir := filepath.Dir(files[0])

		// Find the common parent directory
		rootDir = firstDir
		for _, file := range files[1:] {
			fileDir := filepath.Dir(file)
			// Find common path between rootDir and fileDir
			for !isSubPath(fileDir, rootDir) && !isSubPath(rootDir, fileDir) {
				rootDir = filepath.Dir(rootDir)
				if rootDir == "." || rootDir == "/" {
					break
				}
			}
			if isSubPath(rootDir, fileDir) {
				rootDir = fileDir
			}
		}
	}

	// Create Jet loader
	loader := jet.NewOSFileSystemLoader(rootDir)

	// Create Jet set with custom delimiters
	e.set = jet.NewSet(
		loader,
		jet.WithDelims(e.delims[0], e.delims[1]),
	)

	// Add functions to the set
	e.addFunctionsToSet()

	// Clear template cache when reloading
	e.templateCache = sync.Map{}

	return nil
}

// isSubPath checks if child is a subdirectory of parent
func isSubPath(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// recreateSet recreates the Jet set with new delimiters
func (e *TemplateEngine) recreateSet() {
	if e.set == nil {
		return
	}
	// Clear cache when recreating
	e.templateCache = sync.Map{}
}

// addFunctionsToSet adds custom functions to the Jet set
func (e *TemplateEngine) addFunctionsToSet() {
	if e.set == nil {
		return
	}

	for name, fn := range e.funcMap {
		e.set.AddGlobal(name, fn)
	}
}

// Instance creates a render instance for the specified template
func (e *TemplateEngine) Instance(name string, data any) Render {
	// Use read lock for better concurrency
	e.mu.RLock()
	set := e.set
	e.mu.RUnlock()

	if set == nil {
		return &jetRender{
			template: nil,
			data:     data,
		}
	}

	// Check cache first for instant access
	if cached, ok := e.templateCache.Load(name); ok {
		if template, ok := cached.(*jet.Template); ok {
			return &jetRender{
				template: template,
				data:     data,
			}
		}
	}

	// Get template from Jet set
	template, err := set.GetTemplate(name)
	if err != nil {
		return &jetRender{
			template: nil,
			data:     data,
		}
	}

	// Cache for future requests
	e.templateCache.Store(name, template)

	return &jetRender{
		template: template,
		data:     data,
	}
}

// Render renders the template to the writer
func (r *jetRender) Render(w io.Writer) error {
	if r.template == nil {
		return ErrTemplateNotFound
	}

	// Use pooled VarMap to reduce allocations
	vars := varMapPool.Get().(jet.VarMap)
	defer func() {
		// Clear and return to pool
		for k := range vars {
			delete(vars, k)
		}
		varMapPool.Put(vars)
	}()

	// Convert data to variables if it's a map
	if dataMap, ok := r.data.(map[string]any); ok {
		for key, value := range dataMap {
			vars.Set(key, value)
		}
	}

	// Execute template
	return r.template.Execute(w, vars, r.data)
}
