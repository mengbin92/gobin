package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mengbin92/gobin/internal/log"
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

	// Markdown and rendering configuration
	Markup *MarkupConfig `yaml:"markup"`

	// Static asset pipeline configuration
	Assets *AssetsConfig `yaml:"assets"`

	// Logging configuration (v1.5.0). When set, configures the default
	// diagnostic logger. CLI flags and GOBIN_LOG_* env vars override the
	// values from this section at startup.
	Logging *LoggingConfig `yaml:"logging"`

	// Extended parameters
	Params map[string]interface{} `yaml:"params"`
}

// LoggingConfig configures the default diagnostic logger. v1.5.0.
//
// The three fields mirror the v1.4.0 --verbose / --log-format / --log-file
// CLI flags. Resolution priority (high -> low) is documented in
// internal/log/log.go: CLI flag > GOBIN_LOG_* env var > config > default.
type LoggingConfig struct {
	// Level is the minimum level to emit. One of: debug, info, warn, error.
	// Unknown values from config or env are treated as errors at startup;
	// unknown values passed via CLI flag are tolerated (matching v1.4.0).
	Level string `yaml:"level"`
	// Format selects the slog handler. One of: text, json.
	Format string `yaml:"format"`
	// File is a path. Empty means stderr. Append-mode; parent directory
	// must exist and be writable when the logger is initialized.
	File string `yaml:"file"`
}

// MarkupConfig controls Markdown rendering behavior.
type MarkupConfig struct {
	AllowUnsafeHTML *bool `yaml:"allowUnsafeHTML"`
}

// AssetsConfig controls static asset processing.
type AssetsConfig struct {
	Fingerprint *AssetsFingerprintConfig `yaml:"fingerprint"`
}

// AssetsFingerprintConfig controls how assetURL versions static asset URLs.
//
// Strategy selects the cache-busting style:
//   - "query"    (default, backward-compatible): assetURL appends ?v=<hash>
//     to the URL; on-disk asset filenames are unchanged.
//   - "filename": fingerprintable assets are copied as <name>.<hash>.<ext>
//     and assetURL resolves to that path. Files whose extension is not in
//     Extensions are still copied under their original name and assetURL
//     returns the unversioned path (no query string).
//
// Extensions lists file extensions (with leading dot) eligible for
// filename-level fingerprinting. Ignored when Strategy is "query". Defaults
// to common static-asset extensions when unset.
type AssetsFingerprintConfig struct {
	Strategy   string   `yaml:"strategy"`
	Extensions []string `yaml:"extensions"`
}

// Fingerprint strategy identifiers.
const (
	AssetsFingerprintStrategyQuery    = "query"
	AssetsFingerprintStrategyFilename = "filename"
)

// DefaultAssetsFingerprintExtensions returns the default extensions used for
// filename-level fingerprinting when the user does not configure their own.
func DefaultAssetsFingerprintExtensions() []string {
	// v1.5.0: keep the conservative default but add .mjs and .avif to track
	// modern module and image formats. Fonts (.woff/.woff2/.ttf/.ico) remain
	// in the list because they are common cache-bust targets.
	return []string{".css", ".js", ".mjs", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".woff", ".woff2", ".ttf", ".ico"}
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

// Normalize applies default values and trims path-like settings into a stable form.
func Normalize(cfg *Config) *Config {
	if cfg == nil {
		cfg = &Config{}
	}

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

	if cfg.Assets != nil && cfg.Assets.Fingerprint != nil {
		fp := cfg.Assets.Fingerprint
		if strings.TrimSpace(fp.Strategy) == "" {
			fp.Strategy = AssetsFingerprintStrategyQuery
		}
		if fp.Strategy == AssetsFingerprintStrategyFilename && len(fp.Extensions) == 0 {
			fp.Extensions = DefaultAssetsFingerprintExtensions()
		}
	}

	return cfg
}

// Validate checks whether the normalized configuration is internally consistent.
func Validate(cfg *Config) error {
	return ValidateInDir(cfg, ".")
}

// ValidateInDir checks whether the normalized configuration is internally
// consistent when resolved from the provided base directory.
func ValidateInDir(cfg *Config, baseDir string) error {
	cfg = Normalize(cfg)

	if err := validateBaseURL(cfg.BaseURL); err != nil {
		return err
	}
	if err := validatePostsPermalink(cfg.Permalinks); err != nil {
		return err
	}
	if err := validatePublishDirScope(cfg.PublishDir); err != nil {
		return err
	}
	if err := validateThemeDir(cfg.Theme, cfg.ThemesDir, baseDir); err != nil {
		return err
	}
	if err := validateOutputDirConflicts(cfg); err != nil {
		return err
	}
	if err := validateAssetsFingerprint(cfg.Assets); err != nil {
		return err
	}

	return nil
}

func validateAssetsFingerprint(assets *AssetsConfig) error {
	if assets == nil || assets.Fingerprint == nil {
		return nil
	}
	switch assets.Fingerprint.Strategy {
	case AssetsFingerprintStrategyQuery, AssetsFingerprintStrategyFilename:
		return nil
	default:
		return fmt.Errorf("invalid assets.fingerprint.strategy %q: expected %q or %q",
			assets.Fingerprint.Strategy,
			AssetsFingerprintStrategyQuery,
			AssetsFingerprintStrategyFilename)
	}
}

func validateBaseURL(raw string) error {
	baseURL := strings.TrimSpace(raw)
	if baseURL == "" {
		return nil
	}
	if strings.HasPrefix(baseURL, "/") {
		return nil
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid baseURL %q: %w", raw, err)
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("invalid baseURL %q: must be an absolute URL", raw)
	}

	return nil
}

func validatePostsPermalink(permalinks map[string]string) error {
	if len(permalinks) == 0 {
		return nil
	}

	pattern := strings.TrimSpace(permalinks["posts"])
	if pattern == "" {
		return nil
	}
	if !strings.HasPrefix(pattern, "/") || !strings.HasSuffix(pattern, "/") {
		return fmt.Errorf("invalid permalinks.posts %q: must start and end with '/'", pattern)
	}
	if containsPathTraversal(pattern) {
		return fmt.Errorf("invalid permalinks.posts %q: must not contain path traversal segments", pattern)
	}

	return nil
}

func containsPathTraversal(path string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func validateThemeDir(theme string, themesDir string, baseDir string) error {
	theme = strings.TrimSpace(theme)
	if theme == "" {
		return nil
	}

	themePath := filepath.Join(resolveConfigPath(baseDir, strings.TrimSpace(themesDir)), theme)
	if _, err := os.Stat(themePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("invalid theme %q: theme directory %q does not exist", theme, themePath)
		}
		return fmt.Errorf("invalid theme %q: stat %q: %w", theme, themePath, err)
	}

	return nil
}

func validatePublishDirScope(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	if filepath.IsAbs(trimmed) {
		return fmt.Errorf("invalid publishDir %q: must be a relative path inside the site root", path)
	}

	cleaned := normalizeConfigPath(trimmed)
	if cleaned == "." || containsPathTraversal(cleaned) {
		return fmt.Errorf("invalid publishDir %q: must be a relative path inside the site root", path)
	}

	return nil
}

func validateOutputDirConflicts(cfg *Config) error {
	publishDir := normalizeConfigPath(cfg.PublishDir)
	if publishDir == "" {
		return nil
	}

	inputs := map[string]string{
		"contentDir": normalizeConfigPath(cfg.ContentDir),
		"pageDir":    normalizeConfigPath(cfg.PageDir),
		"staticDir":  normalizeConfigPath(cfg.StaticDir),
	}

	for name, path := range inputs {
		if path == "" {
			continue
		}
		if path == publishDir {
			return fmt.Errorf("invalid publishDir %q: must not overlap %s %q", cfg.PublishDir, name, path)
		}
	}

	return nil
}

func normalizeConfigPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	return filepath.ToSlash(cleaned)
}

func resolveConfigPath(baseDir string, path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return strings.TrimSpace(baseDir)
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(strings.TrimSpace(baseDir), trimmed)
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	logger := log.GetDefault().With("component", "config")
	logger.Debug("loading configuration", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	normalized := Normalize(&cfg)
	if err := ValidateInDir(normalized, filepath.Dir(path)); err != nil {
		return nil, err
	}

	logger.Debug("config loaded", "path", path, "title", normalized.Title, "publish_dir", normalized.PublishDir)
	return normalized, nil
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

// LoadIfPresent returns LoadDefault's result, or (nil, nil) when no
// config file exists. Use this in global paths (e.g. PersistentPreRun)
// that need to honor `logging:` config when the user is in a site
// directory, without forcing every command (like `version`) to require
// a config to be present.
func LoadIfPresent() (*Config, error) {
	cfg, err := LoadDefault()
	if err == nil {
		return cfg, nil
	}
	if _, statErr := os.Stat("config.yaml"); statErr == nil {
		// exists but failed to parse -> surface the real error
		return nil, err
	}
	if _, statErr := os.Stat("config.yml"); statErr == nil {
		return nil, err
	}
	return nil, nil
}
