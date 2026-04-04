package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the site configuration
type Config struct {
	Title        string `yaml:"title"`
	Description  string `yaml:"description"`
	Author       string `yaml:"author"`
	LanguageCode string `yaml:"languageCode"`
	BaseURL      string `yaml:"baseURL"`
	Timezone     string `yaml:"timezone"`

	// Directory configuration
	ContentDir string `yaml:"contentDir"`
	PageDir    string `yaml:"pageDir"`
	StaticDir  string `yaml:"staticDir"`
	PublishDir string `yaml:"publishDir"`
	ThemesDir  string `yaml:"themesDir"`

	// Theme configuration
	Theme string `yaml:"theme"`

	// Pagination
	Paginate     int    `yaml:"paginate"`
	PaginatePath string `yaml:"paginatePath"`

	// Permalink configuration
	Permalinks map[string]string `yaml:"permalinks"`

	// Navigation
	NavbarLinks []NavbarLink `yaml:"navbarLinks"`

	// Repository URL for source code link
	RepositoryURL string `yaml:"repositoryURL"`

	// Social media
	Social map[string]string `yaml:"social"`

	// Feature flags
	EnableEmoji     bool `yaml:"enableEmoji"`
	EnableGitInfo   bool `yaml:"enableGitInfo"`
	EnableRobotsTXT bool `yaml:"enableRobotsTXT"`

	// SEO configuration
	SEO *SEOConfig `yaml:"seo"`

	// Comments configuration
	Comments *CommentsConfig `yaml:"comments"`

	// Analytics configuration
	Analytics *AnalyticsConfig `yaml:"analytics"`

	// Output controls
	Outputs *OutputsConfig `yaml:"outputs"`

	// Extended parameters
	Params map[string]interface{} `yaml:"params"`
}

// OutputsConfig controls generated site-level artifacts.
type OutputsConfig struct {
	Feed    *bool `yaml:"feed"`
	Search  *bool `yaml:"search"`
	Sitemap *bool `yaml:"sitemap"`
	Robots  *bool `yaml:"robots"`
}

// SEOConfig represents SEO configuration
type SEOConfig struct {
	Enabled         bool   `yaml:"enabled"`
	OpenGraph       bool   `yaml:"openGraph"`
	TwitterCard     bool   `yaml:"twitterCard"`
	Image           string `yaml:"image"`
	TwitterUsername string `yaml:"twitterUsername"`
}

// CommentsConfig represents comments system configuration
type CommentsConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Provider   string            `yaml:"provider"` // disqus, gitalk, utterances
	Disqus     *DisqusConfig     `yaml:"disqus"`
	Gitalk     *GitalkConfig     `yaml:"gitalk"`
	Utterances *UtterancesConfig `yaml:"utterances"`
}

// DisqusConfig represents Disqus configuration
type DisqusConfig struct {
	Shortname string `yaml:"shortname"`
}

// GitalkConfig represents Gitalk configuration
type GitalkConfig struct {
	ClientID            string `yaml:"clientID"`
	ClientSecret        string `yaml:"clientSecret"`
	Repo                string `yaml:"repo"`
	Owner               string `yaml:"owner"`
	Admin               string `yaml:"admin"`
	DistractionFreeMode bool   `yaml:"distractionFreeMode"`
}

// UtterancesConfig represents Utterances configuration
type UtterancesConfig struct {
	Repo  string `yaml:"repo"`
	Theme string `yaml:"theme"`
	Label string `yaml:"label"`
}

// AnalyticsConfig represents analytics configuration
type AnalyticsConfig struct {
	Provider string                 `yaml:"provider"` // google, baidu, matomo
	Google   *GoogleAnalyticsConfig `yaml:"google"`
	Baidu    *BaiduAnalyticsConfig  `yaml:"baidu"`
	Matomo   *MatomoAnalyticsConfig `yaml:"matomo"`
}

// GoogleAnalyticsConfig represents Google Analytics configuration
type GoogleAnalyticsConfig struct {
	TrackingID string `yaml:"trackingID"`
}

// BaiduAnalyticsConfig represents Baidu Analytics configuration
type BaiduAnalyticsConfig struct {
	TrackingID string `yaml:"trackingID"`
}

// MatomoAnalyticsConfig represents Matomo Analytics configuration
type MatomoAnalyticsConfig struct {
	URL        string `yaml:"url"`
	SiteID     int    `yaml:"siteID"`
	TrackerURL string `yaml:"trackerURL"`
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
	if cfg.PageDir == "" {
		cfg.PageDir = "pages"
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

// LoadDefault loads configuration from the first existing default config path.
func LoadDefault() (*Config, error) {
	candidates := []string{
		"config.yaml",
		"config.yml",
		"_config.yml",
		"_config.yaml",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}

	return nil, fmt.Errorf("no config file found (tried: %s)", candidates)
}
