package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
)

func copyStaticAssets(cfg *config.Config, outputDir string) error {
	if cfg.Theme != "" && cfg.ThemesDir != "" {
		themeStaticDir := filepath.Join(cfg.ThemesDir, cfg.Theme, "assets")
		if _, err := os.Stat(themeStaticDir); err == nil {
			if err := copyStaticAssetsFromDir(themeStaticDir, outputDir, themeStaticDir); err != nil {
				return err
			}
		}
	}

	staticDir := cfg.StaticDir
	if staticDir == "" {
		staticDir = "assets"
	}

	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		return nil
	}

	return copyStaticAssetsFromDir(staticDir, outputDir, staticDir)
}

func copyStaticAssetsFromDir(sourceDir, outputDir, baseDir string) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(outputDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		return copyFile(path, destPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

func minifyOutput(outputDir string) error {
	return filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".html" && ext != ".css" && ext != ".js" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(path, []byte(minifyContent(string(content), ext)), info.Mode())
	})
}

func minifyContent(content string, ext string) string {
	switch ext {
	case ".css":
		return minifyCSSContent(content)
	case ".html":
		return minifyHTMLContent(content)
	case ".js":
		// Keep JavaScript source unchanged apart from surrounding whitespace.
		// Regex-based "minification" is too risky for string literals and regexes.
		return strings.TrimSpace(content)
	default:
		return strings.TrimSpace(content)
	}
}

var cssCommentPattern = regexp.MustCompile(`(?s)/\*.*?\*/`)
var cssWhitespacePattern = regexp.MustCompile(`\s+`)
var cssPunctuationPattern = regexp.MustCompile(`\s*([{}:;,>+~])\s*`)

func minifyCSSContent(content string) string {
	content = cssCommentPattern.ReplaceAllString(content, "")
	content = cssWhitespacePattern.ReplaceAllString(content, " ")
	content = cssPunctuationPattern.ReplaceAllString(content, "$1")
	content = strings.ReplaceAll(content, ";}", "}")
	return strings.TrimSpace(content)
}

func minifyHTMLContent(content string) string {
	var out bytes.Buffer
	var text bytes.Buffer
	preserveTag := ""

	flushText := func() {
		if text.Len() == 0 {
			return
		}
		if preserveTag != "" {
			out.Write(text.Bytes())
		} else {
			out.WriteString(collapseHTMLWhitespace(text.String()))
		}
		text.Reset()
	}

	for i := 0; i < len(content); {
		if strings.HasPrefix(content[i:], "<!--") {
			flushText()
			end := strings.Index(content[i+4:], "-->")
			if end < 0 {
				break
			}
			i += 4 + end + 3
			continue
		}

		if content[i] != '<' {
			text.WriteByte(content[i])
			i++
			continue
		}

		flushText()
		tagEnd := strings.IndexByte(content[i:], '>')
		if tagEnd < 0 {
			out.WriteString(content[i:])
			break
		}

		tag := content[i : i+tagEnd+1]
		tagName, closing, selfClosing := parseHTMLTag(tag)
		if tagName != "" && isHTMLWhitespaceSensitiveTag(tagName) {
			if !closing && !selfClosing {
				preserveTag = tagName
			} else if closing && preserveTag == tagName {
				preserveTag = ""
			}
		}

		out.WriteString(strings.TrimSpace(tag))
		i += tagEnd + 1
	}

	flushText()
	return strings.TrimSpace(out.String())
}

func collapseHTMLWhitespace(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func parseHTMLTag(tag string) (name string, closing bool, selfClosing bool) {
	trimmed := strings.TrimSpace(tag)
	trimmed = strings.TrimPrefix(trimmed, "<")
	trimmed = strings.TrimSuffix(trimmed, ">")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", false, false
	}

	if strings.HasPrefix(trimmed, "/") {
		closing = true
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "/"))
	}
	if strings.HasSuffix(trimmed, "/") {
		selfClosing = true
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "/"))
	}
	if trimmed == "" {
		return "", closing, selfClosing
	}

	for i, r := range trimmed {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return strings.ToLower(trimmed[:i]), closing, selfClosing
		}
	}

	return strings.ToLower(trimmed), closing, selfClosing
}

func isHTMLWhitespaceSensitiveTag(tag string) bool {
	switch tag {
	case "pre", "script", "style", "textarea":
		return true
	default:
		return false
	}
}
