package generator

import "github.com/mengbin92/gobin/internal/config"

func outputEnabled(cfg *config.Config, name string, fallback bool) bool {
	if cfg == nil || cfg.Outputs == nil {
		return fallback
	}

	switch name {
	case "feed":
		if cfg.Outputs.Feed != nil {
			return *cfg.Outputs.Feed
		}
	case "search":
		if cfg.Outputs.Search != nil {
			return *cfg.Outputs.Search
		}
	case "sitemap":
		if cfg.Outputs.Sitemap != nil {
			return *cfg.Outputs.Sitemap
		}
	case "robots":
		if cfg.Outputs.Robots != nil {
			return *cfg.Outputs.Robots
		}
	}

	return fallback
}
