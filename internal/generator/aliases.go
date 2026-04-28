package generator

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func generateAliasPages(posts []*parser.Post, cfg *config.Config, outputDir string) error {
	generated := make(map[string]string)

	for _, post := range posts {
		for _, alias := range post.Aliases {
			aliasPath, outputPath, err := resolveAliasOutput(alias, outputDir)
			if err != nil {
				return fmt.Errorf("invalid alias %q for post %q: %w", alias, post.Title, err)
			}

			if owner, exists := generated[outputPath]; exists {
				return fmt.Errorf("alias %q for post %q conflicts with alias generated for %q", aliasPath, post.Title, owner)
			}
			if _, err := os.Stat(outputPath); err == nil {
				return fmt.Errorf("alias %q for post %q conflicts with an existing generated page", aliasPath, post.Title)
			} else if !os.IsNotExist(err) {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(outputPath, []byte(renderAliasHTML(cfg, aliasPath, post.URL)), 0644); err != nil {
				return err
			}

			generated[outputPath] = post.Title
		}
	}

	return nil
}

func resolveAliasOutput(alias, outputDir string) (string, string, error) {
	aliasPath := normalizeAliasPath(alias)
	if aliasPath == "/" {
		return "", "", fmt.Errorf("root path is not allowed")
	}

	trimmed := strings.TrimPrefix(aliasPath, "/")
	if trimmed == "" {
		return "", "", fmt.Errorf("empty alias path")
	}

	if strings.HasSuffix(trimmed, ".html") {
		outputPath, err := safeOutputPath(outputDir, trimmed)
		return aliasPath, outputPath, err
	}

	trimmed = strings.TrimSuffix(trimmed, "/")
	if trimmed == "" {
		return "", "", fmt.Errorf("empty alias path")
	}

	outputPath, err := safeOutputPath(outputDir, filepath.ToSlash(filepath.Join(trimmed, "index.html")))
	return ensureTrailingSlash(aliasPath), outputPath, err
}

func normalizeAliasPath(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return "/"
	}

	if !strings.HasPrefix(alias, "/") {
		alias = "/" + alias
	}

	alias = strings.ReplaceAll(alias, "//", "/")
	if strings.HasSuffix(alias, "/index.html") {
		alias = strings.TrimSuffix(alias, "index.html")
	}

	if !strings.HasSuffix(alias, "/") && !strings.HasSuffix(alias, ".html") {
		alias += "/"
	}

	return alias
}

func ensureTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}

func renderAliasHTML(cfg *config.Config, aliasPath, targetPath string) string {
	canonical := html.EscapeString(joinURL(cfg.BaseURL, targetPath))
	aliasCanonical := html.EscapeString(joinURL(cfg.BaseURL, aliasPath))
	targetEscaped := html.EscapeString(targetPath)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <title>Redirecting...</title>
    <meta name="robots" content="noindex">
    <link rel="canonical" href="%s">
    <meta http-equiv="refresh" content="0; url=%s">
    <script>location.replace(%q);</script>
</head>
<body>
    <p>This page has moved to <a href="%s">%s</a>.</p>
    <p>Alias: <code>%s</code></p>
</body>
</html>
`, canonical, targetEscaped, targetPath, targetEscaped, targetEscaped, aliasCanonical)
}
