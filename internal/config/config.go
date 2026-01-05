package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the site configuration
type Config struct {
	Title           string            `yaml:"title"`
	Description     string            `yaml:"description"`
	Author          string            `yaml:"author"`
	LanguageCode    string            `yaml:"languageCode"`
	BaseURL         string            `yaml:"baseURL"`
	Timezone        string            `yaml:"timezone"`

	// Directory configuration
	ContentDir      string            `yaml:"contentDir"`
	StaticDir       string            `yaml:"staticDir"`
	PublishDir      string            `yaml:"publishDir"`
	ThemesDir       string            `yaml:"themesDir"`

	// Pagination
	Paginate        int               `yaml:"paginate"`
	PaginatePath    string            `yaml:"paginatePath"`

	// Permalink configuration
	Permalinks      map[string]string `yaml:"permalinks"`

	// Navigation
	NavbarLinks     []NavbarLink      `yaml:"navbarLinks"`

	// Social media
	Social          map[string]string `yaml:"social"`

	// Feature flags
	EnableEmoji     bool              `yaml:"enableEmoji"`
	EnableGitInfo   bool              `yaml:"enableGitInfo"`
	EnableRobotsTXT bool              `yaml:"enableRobotsTXT"`

	// Extended parameters
	Params          map[string]interface{} `yaml:"params"`
}

// NavbarLink represents a navigation link
type NavbarLink struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Paginate == 0 {
		cfg.Paginate = 10
	}
	if cfg.PaginatePath == "" {
		cfg.PaginatePath = "page"
	}
	if cfg.ContentDir == "" {
		cfg.ContentDir = "_posts"
	}
	if cfg.PublishDir == "" {
		cfg.PublishDir = "public"
	}
	if cfg.StaticDir == "" {
		cfg.StaticDir = "assets"
	}
	if cfg.ThemesDir == "" {
		cfg.ThemesDir = "themes"
	}

	return &cfg, nil
}
