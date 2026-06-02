package shortcode

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/textutil"
)

// Registry holds the compiled shortcode templates available to a render,
// keyed by shortcode name. It is built once per build/serve render by the
// command layer and carried into the parser via RenderOptions.
type Registry struct {
	templates map[string]*template.Template
}

// Lookup returns the template registered for name, if any. It is nil-safe.
func (r *Registry) Lookup(name string) (*template.Template, bool) {
	if r == nil {
		return nil, false
	}
	t, ok := r.templates[name]
	return t, ok
}

// Names returns the registered shortcode names in sorted order.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ShortcodeContext is the data passed to a shortcode template. It exposes
// Hugo-style accessors: .Get for positional (int) and named (string) arguments,
// .Inner for the body of a paired shortcode, and .Name.
type ShortcodeContext struct {
	Name       string
	Inner      string
	positional []string
	named      map[string]string
}

// Get returns a positional argument when key is an int, or a named argument
// when key is a string. Missing arguments return an empty string.
func (c ShortcodeContext) Get(key any) any {
	switch k := key.(type) {
	case int:
		if k >= 0 && k < len(c.positional) {
			return c.positional[k]
		}
	case string:
		if v, ok := c.named[k]; ok {
			return v
		}
	}
	return ""
}

// Param is an alias for the named-argument lookup of Get.
func (c ShortcodeContext) Param(key string) any {
	return c.Get(key)
}

func newContext(name string, a *args, inner string) ShortcodeContext {
	ctx := ShortcodeContext{Name: name, Inner: inner}
	if a != nil {
		ctx.positional = a.positional
		ctx.named = a.named
	}
	return ctx
}

// LoadRegistry builds the shortcode registry for cfg. It starts from the
// built-in shortcodes, then overlays theme shortcodes
// (<themesDir>/<theme>/layouts/shortcodes), then site shortcodes
// (templates/shortcodes). Later sources override earlier ones, mirroring the
// templates/layouts override precedence used by the generator.
func LoadRegistry(cfg *config.Config) (*Registry, error) {
	cfg = config.Normalize(cfg)
	funcs := shortcodeFuncMap(cfg)

	reg := &Registry{templates: make(map[string]*template.Template)}

	for _, name := range sortedKeys(builtinShortcodes) {
		t, err := template.New(name).Funcs(funcs).Parse(builtinShortcodes[name])
		if err != nil {
			return nil, fmt.Errorf("parse built-in shortcode %q: %w", name, err)
		}
		reg.templates[name] = t
	}

	var dirs []string
	if cfg.Theme != "" {
		dirs = append(dirs, filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "shortcodes"))
	}
	dirs = append(dirs, filepath.Join("templates", "shortcodes"))

	for _, dir := range dirs {
		if err := loadShortcodeDir(reg, dir, funcs); err != nil {
			return nil, err
		}
	}

	return reg, nil
}

func loadShortcodeDir(reg *Registry, dir string, funcs template.FuncMap) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read shortcodes dir %s: %w", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".html" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, fname := range names {
		path := filepath.Join(dir, fname)
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read shortcode %s: %w", path, err)
		}
		name := strings.TrimSuffix(fname, filepath.Ext(fname))
		t, err := template.New(name).Funcs(funcs).Parse(string(body))
		if err != nil {
			return fmt.Errorf("parse shortcode %s: %w", path, err)
		}
		reg.templates[name] = t
	}

	return nil
}

// shortcodeFuncMap is the small, self-contained set of template helpers exposed
// to shortcode templates. It intentionally duplicates a subset of the
// generator's funcMap to keep this package free of a generator import.
func shortcodeFuncMap(cfg *config.Config) template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"absURL": func(path string) string {
			return joinURL(cfg.BaseURL, path)
		},
		"urlize": textutil.Slug,
		"default": func(fallback, value any) any {
			switch v := value.(type) {
			case string:
				if v == "" {
					return fallback
				}
			case nil:
				return fallback
			}
			return value
		},
	}
}

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")

	if base == "" {
		if path == "" {
			return "/"
		}
		return "/" + path
	}
	if path == "" {
		return base + "/"
	}
	return base + "/" + path
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
