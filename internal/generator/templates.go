package generator

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func loadTemplates(cfg *config.Config) (*template.Template, error) {
	cfg = config.Normalize(cfg)

	var tmpl *template.Template

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
		"stylesheetPath": func() string {
			return detectStylesheetPath(cfg)
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

	var templateFiles []string
	for _, path := range getTemplatePaths(cfg) {
		if _, err := os.Stat(path); err == nil {
			templateFiles = append(templateFiles, path)
		}
	}

	if len(templateFiles) == 0 {
		return nil, fmt.Errorf("no templates found")
	}

	tmpl, err := tmpl.ParseFiles(templateFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return tmpl, nil
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
	for _, tmplFile := range tmplFiles {
		defaultTmplPath := filepath.Join(defaultDir, tmplFile)
		if _, err := os.Stat(defaultTmplPath); err == nil {
			paths = append(paths, defaultTmplPath)
		}
	}

	if cfg.Theme != "" {
		themeDir := filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts")
		if _, err := os.Stat(themeDir); err == nil {
			for _, tmplFile := range tmplFiles {
				themeTmplPath := filepath.Join(themeDir, tmplFile)
				if _, err := os.Stat(themeTmplPath); err == nil {
					paths = append(paths, themeTmplPath)
				}
			}
		}
	}

	return paths
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
