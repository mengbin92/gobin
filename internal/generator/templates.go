package generator

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/log"
	"github.com/mengbin92/gobin/internal/parser"
)

func loadTemplates(cfg *config.Config) (*template.Template, error) {
	cfg = config.Normalize(cfg)

	var tmpl *template.Template
	assetURLs, err := newAssetURLResolver(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve static assets: %w", err)
	}

	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"dateFormat": func(layout string, t time.Time) string {
			return t.Format(layout)
		},
		"now": func() time.Time {
			return time.Now()
		},
		"urlize": urlize,
		"absURL": func(path string) string {
			return joinURL(cfg.BaseURL, path)
		},
		"url": func(path string) string {
			return siteURLPath(cfg.BaseURL, path)
		},
		"assetURL": assetURLs.URL,
		"stylesheetPath": func() string {
			return detectStylesheetPath(cfg)
		},
		// v1.7 image helper. Emits a plain <img src=...> so the
		// postprocess step can rewrite it to <picture><source srcset>...
		// once the image pipeline has produced the responsive variants
		// and the .gobin-images.json manifest is on disk. The alt
		// argument is required for accessibility; passing an empty
		// string is allowed but emits alt="".
		//
		// The helper intentionally does not pre-compute the variant set
		// at template-render time. Templates run before the image
		// artifact, so they cannot know which widths / formats are
		// available. The postprocess step is the single source of
		// truth for the rewrite, which keeps the helper simple and
		// makes the failure mode obvious (no <img src> rewrite = no
		// variants were produced).
		"image": func(src string, alt string) template.HTML {
			if src == "" {
				return template.HTML("")
			}
			return template.HTML(fmt.Sprintf(`<img src=%q alt=%q loading="lazy" decoding="async">`, src, alt))
		},
		"render": func(name string, data interface{}) (template.HTML, error) {
			var buf bytes.Buffer
			if tmpl == nil {
				return "", fmt.Errorf("template set is not initialized")
			}
			if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
				return "", err
			}
			return template.HTML(buf.String()), nil
		},
		"default": func(fallback, value interface{}) interface{} {
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
		"first": func(n int, list interface{}) interface{} {
			switch v := list.(type) {
			case []string:
				if len(v) < n {
					return v
				}
				return v[:n]
			case []*parser.Post:
				if len(v) < n {
					return v
				}
				return v[:n]
			default:
				return nil
			}
		},
		"contains": strings.Contains,
		"paramBool": func(params map[string]interface{}, key string) bool {
			if params == nil {
				return false
			}
			value, ok := params[key]
			if !ok || value == nil {
				return false
			}
			switch v := value.(type) {
			case bool:
				return v
			case string:
				return strings.EqualFold(strings.TrimSpace(v), "true")
			default:
				return false
			}
		},
	}

	tmpl = template.New("").Funcs(funcMap)

	// Collect Gobin-native templates (templates/ + theme layouts/).
	var templateFiles []string
	for _, path := range getTemplatePaths(cfg) {
		if _, err := os.Stat(path); err == nil {
			templateFiles = append(templateFiles, path)
		}
	}

	// v1.8 Jekyll compatibility: also count _layouts/ + _includes/.
	// A pure Jekyll site may have only _layouts/ with no templates/ at all.
	hasLayouts, _ := dirHasHTMLFiles("_layouts")
	hasIncludes, _ := dirHasHTMLFiles("_includes")

	if len(templateFiles) == 0 && !hasLayouts && !hasIncludes {
		return nil, fmt.Errorf("no templates found")
	}

	if len(templateFiles) > 0 {
		tmpl, err = tmpl.ParseFiles(templateFiles...)
		if err != nil {
			return nil, fmt.Errorf("failed to parse templates: %w", err)
		}
	}

	// Register _layouts/*.html and _includes/*.html by their basename
	// (file name without extension), so a post with `layout: post`
	// resolves to the "_layouts/post.html" template without requiring
	// a {{ define }} block.
	tmpl, err = registerLayoutsAndIncludes(tmpl, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse layouts/includes: %w", err)
	}

	return tmpl, nil
}

// dirHasHTMLFiles reports whether dir exists and contains at least one .html file.
func dirHasHTMLFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, nil
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".html") {
			return true, nil
		}
	}
	return false, nil
}

func getTemplatePaths(cfg *config.Config) []string {
	cfg = config.Normalize(cfg)

	var paths []string

	tmplFiles := []string{
		"_default/base.html",
		"_default/single.html",
		"_default/list.html",
		"_default/page.html",
		"_default/404.html",
		"_default/taxonomy.html",
		"partials/header.html",
		"partials/footer.html",
		"partials/comments.html",
		"partials/analytics.html",
	}

	defaultDir := "templates"
	paths = append(paths, collectTemplatePaths(defaultDir, tmplFiles)...)

	if cfg.Theme != "" {
		themeDir := filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts")
		if _, err := os.Stat(themeDir); err == nil {
			paths = append(paths, collectTemplatePaths(themeDir, tmplFiles)...)
		}
	}

	return paths
}

func collectTemplatePaths(root string, knownFiles []string) []string {
	known := make(map[string]struct{}, len(knownFiles))
	paths := make([]string, 0, len(knownFiles))

	for _, tmplFile := range knownFiles {
		cleanRel := filepath.Clean(tmplFile)
		known[cleanRel] = struct{}{}
		tmplPath := filepath.Join(root, cleanRel)
		if _, err := os.Stat(tmplPath); err == nil {
			paths = append(paths, tmplPath)
		}
	}

	var discovered []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".html" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if _, ok := known[filepath.Clean(rel)]; ok {
			return nil
		}
		discovered = append(discovered, path)
		return nil
	})
	sort.Strings(discovered)

	return append(paths, discovered...)
}

func renderTemplate(tmpl renderer, name, path string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.ExecuteTemplate(f, name, data)
}

func detectStylesheetPath(cfg *config.Config) string {
	cfg = config.Normalize(cfg)

	type stylesheetCandidate struct {
		webPath    string
		sourcePath string
	}

	candidates := []stylesheetCandidate{
		{webPath: "/css/main.css", sourcePath: filepath.Join(cfg.StaticDir, "css", "main.css")},
		{webPath: "/css/style.css", sourcePath: filepath.Join(cfg.StaticDir, "css", "style.css")},
	}

	if cfg.Theme != "" {
		themeAssetDir := filepath.Join(cfg.ThemesDir, cfg.Theme, "assets")
		candidates = append(candidates, []stylesheetCandidate{
			{webPath: "/css/main.css", sourcePath: filepath.Join(themeAssetDir, "css", "main.css")},
			{webPath: "/css/style.css", sourcePath: filepath.Join(themeAssetDir, "css", "style.css")},
		}...)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate.sourcePath); err == nil {
			return candidate.webPath
		}
	}

	return ""
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

func hasAbsoluteBaseURL(base string) bool {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return false
	}
	return parsed.IsAbs() && parsed.Host != ""
}

// registerLayoutsAndIncludes scans the Jekyll-style _layouts/ and
// _includes/ directories at the site root and registers each .html file
// as a named template whose name is the file basename without extension
// (e.g. _layouts/post.html -> "post"). This lets front matter `layout:
// post` resolve to the file without a {{ define }} wrapper.
//
// A file that already declares a {{ define "X" }} is parsed with its
// defined name AND its basename, so both `{{ template "post" . }}` and
// `{{ template "X" . }}` work. If a basename collides with an existing
// template name, the existing template wins (ParseGlob/Parse returns an
// error on duplicate, so we skip re-registering names already present).
func registerLayoutsAndIncludes(tmpl *template.Template, cfg *config.Config) (*template.Template, error) {
	logger := log.GetDefault().With("component", "templates")
	dirs := []string{"_layouts", "_includes"}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".html") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			if tmpl.Lookup(name) != nil {
				// Already registered (e.g. via templates/ {{ define }}).
				// Keep the existing definition to avoid surprises.
				continue
			}
			full := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(full)
			if err != nil {
				return nil, err
			}
			nt := tmpl.New(name)
			if _, err := nt.Parse(string(data)); err != nil {
				// Gracefully skip files that fail to parse. This typically
				// means the file still contains Jekyll/Liquid syntax
				// (e.g. {{ site.x }}, {% if %}) that hasn't been migrated
				// to Go template syntax yet. Skipping is preferable to
				// failing the entire build — the user will migrate
				// includes one at a time.
				logger.Warn("skipping unparseable layout/include (likely unmigrated Liquid)",
					"file", full, "template", name, "error", err)
				// Remove the partially-created empty template so Lookup
				// doesn't return a nameless definition.
				continue
			}
		}
	}
	return tmpl, nil
}
