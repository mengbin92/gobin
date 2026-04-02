package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
)

func generateRobotsTXT(cfg *config.Config, outputDir string) error {
	if cfg == nil || !cfg.EnableRobotsTXT || !outputEnabled(cfg, "robots", true) {
		return nil
	}

	lines := []string{
		"User-agent: *",
		"Allow: /",
	}

	if baseURL := strings.TrimSpace(cfg.BaseURL); hasAbsoluteBaseURL(baseURL) {
		lines = append(lines, fmt.Sprintf("Sitemap: %s", joinURL(baseURL, "sitemap.xml")))
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(outputDir, "robots.txt"), []byte(content), 0644)
}
