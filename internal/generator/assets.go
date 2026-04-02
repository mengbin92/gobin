package generator

import (
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

		return os.WriteFile(path, []byte(minifyContent(string(content), ext)), 0644)
	})
}

func minifyContent(content string, ext string) string {
	content = regexp.MustCompile(`<!--.*?-->`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = regexp.MustCompile(`>\s+<`).ReplaceAllString(content, "><")
	content = regexp.MustCompile(`\s*{\s*`).ReplaceAllString(content, "{")
	content = regexp.MustCompile(`\s*}\s*`).ReplaceAllString(content, "}")
	content = regexp.MustCompile(`\s*;\s*`).ReplaceAllString(content, ";")
	content = regexp.MustCompile(`\s*,\s*`).ReplaceAllString(content, ",")
	return strings.TrimSpace(content)
}
