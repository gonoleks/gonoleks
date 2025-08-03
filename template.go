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
	set     *jet.Set
	delims  [2]string
	funcMap map[string]any
	mu      sync.RWMutex
}

// jetRender implements Render interface for Jet templates
type jetRender struct {
	template *jet.Template
	data     any
}

// Pool for VarMap reuse to reduce allocations
var varMapPool = sync.Pool{
	New: func() any {
		return make(jet.VarMap)
	},
}

// NewTemplateEngine creates a new Jet-based template engine
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{}
}

// SetDelims sets the template delimiters
func (te *TemplateEngine) SetDelims(left, right string) {
	te.mu.Lock()
	te.delims = [2]string{left, right}
	if te.set != nil {
		te.recreateSet()
	}
	te.mu.Unlock()
}

// SetFuncMap sets the function map for templates
func (te *TemplateEngine) SetFuncMap(funcMap map[string]any) {
	te.mu.Lock()
	te.funcMap = funcMap
	if te.set != nil {
		te.addFunctionsToSet()
	}
	te.mu.Unlock()
}

// LoadGlob loads templates using glob pattern
func (te *TemplateEngine) LoadGlob(pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	return te.LoadFiles(files...)
}

// LoadFiles loads templates from specified files
func (te *TemplateEngine) LoadFiles(files ...string) error {
	if len(files) == 0 {
		return nil
	}

	te.mu.Lock()
	defer te.mu.Unlock()

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
	te.set = jet.NewSet(
		loader,
		jet.WithDelims(te.delims[0], te.delims[1]),
	)

	// Add functions to the set
	te.addFunctionsToSet()

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
func (te *TemplateEngine) recreateSet() {
	if te.set == nil {
		return
	}
}

// addFunctionsToSet adds custom functions to the Jet set
func (te *TemplateEngine) addFunctionsToSet() {
	if te.set == nil {
		return
	}

	for name, fn := range te.funcMap {
		te.set.AddGlobal(name, fn)
	}
}

// Instance creates a render instance for the specified template
func (te *TemplateEngine) Instance(name string, data any) Render {
	// Use read lock for better concurrency
	te.mu.RLock()
	set := te.set
	te.mu.RUnlock()

	if set == nil {
		return &jetRender{
			template: nil,
			data:     data,
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

	return &jetRender{
		template: template,
		data:     data,
	}
}

// Render renders the template to the writer
func (jr *jetRender) Render(w io.Writer) error {
	if jr.template == nil {
		return ErrTemplateNotFound
	}

	// Use pooled VarMap to reduce allocations
	vars := varMapPool.Get().(jet.VarMap)
	defer func() {
		// Clear and return to pool
		clear(vars)
		varMapPool.Put(vars)
	}()

	// Convert data to variables if it's a map
	if dataMap, ok := jr.data.(map[string]any); ok {
		for key, value := range dataMap {
			vars.Set(key, value)
		}
	}

	// Execute template
	return jr.template.Execute(w, vars, jr.data)
}
