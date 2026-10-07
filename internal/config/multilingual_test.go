package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestIsMultilingual(t *testing.T) {
	if (&Config{}).IsMultilingual() {
		t.Error("empty config should not be multilingual")
	}
	cfg := &Config{Languages: map[string]*LanguageConfig{"zh": {}}}
	if !cfg.IsMultilingual() {
		t.Error("config with languages should be multilingual")
	}
}

func TestLanguageNames_Sorted(t *testing.T) {
	cfg := &Config{Languages: map[string]*LanguageConfig{
		"zh": {}, "en": {}, "ja": {},
	}}
	got := cfg.LanguageNames()
	want := []string{"en", "ja", "zh"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LanguageNames() = %v, want %v", got, want)
	}

	if names := (&Config{}).LanguageNames(); names != nil {
		t.Errorf("LanguageNames() without languages = %v, want nil", names)
	}
}

func TestResolveLanguage_Defaults(t *testing.T) {
	cfg := Normalize(&Config{
		Languages: map[string]*LanguageConfig{"zh": {}},
	})

	resolved := cfg.ResolveLanguage("zh")
	if resolved.LanguageCode != "zh" {
		t.Errorf("LanguageCode = %q, want %q", resolved.LanguageCode, "zh")
	}
	if resolved.ContentDir != "_posts/zh" {
		t.Errorf("ContentDir = %q, want %q", resolved.ContentDir, "_posts/zh")
	}
	if resolved.PageDir != "pages/zh" {
		t.Errorf("PageDir = %q, want %q", resolved.PageDir, "pages/zh")
	}
}

func TestResolveLanguage_Overrides(t *testing.T) {
	cfg := Normalize(&Config{
		Languages: map[string]*LanguageConfig{
			"zh": {
				LanguageCode: "zh-CN",
				ContentDir:   "content/zh",
				PageDir:      "content/zh-pages",
			},
		},
	})

	resolved := cfg.ResolveLanguage("zh")
	if resolved.LanguageCode != "zh-CN" {
		t.Errorf("LanguageCode = %q, want %q", resolved.LanguageCode, "zh-CN")
	}
	if resolved.ContentDir != "content/zh" {
		t.Errorf("ContentDir = %q, want %q", resolved.ContentDir, "content/zh")
	}
	if resolved.PageDir != "content/zh-pages" {
		t.Errorf("PageDir = %q, want %q", resolved.PageDir, "content/zh-pages")
	}
}

func TestDeriveForLanguage_BaseURLForms(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"empty", "", "/zh"},
		{"root slash", "/", "/zh"},
		{"host only", "https://example.com", "https://example.com/zh"},
		{"host with path", "https://example.com/blog", "https://example.com/blog/zh"},
		{"trailing slash", "https://example.com/blog/", "https://example.com/blog/zh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{BaseURL: tt.baseURL, Languages: map[string]*LanguageConfig{"zh": {}}}
			derived := cfg.DeriveForLanguage("zh")
			if derived.BaseURL != tt.want {
				t.Errorf("BaseURL = %q, want %q", derived.BaseURL, tt.want)
			}
			if cfg.BaseURL != tt.baseURL {
				t.Errorf("original BaseURL mutated to %q", cfg.BaseURL)
			}
		})
	}
}

func TestDeriveForLanguage_OverridesAndMerges(t *testing.T) {
	cfg := &Config{
		Title:        "Default Title",
		Description:  "Default Description",
		LanguageCode: "en",
		BaseURL:      "https://example.com",
		Params:       map[string]interface{}{"shared": "top", "override": "top"},
		Strings:      map[string]string{"readMore": "Read more", "shared": "top"},
		Languages: map[string]*LanguageConfig{
			"zh": {
				Title:   "中文标题",
				Params:  map[string]interface{}{"override": "lang", "extra": "lang"},
				Strings: map[string]string{"readMore": "阅读更多"},
			},
		},
	}

	derived := cfg.DeriveForLanguage("zh")

	if derived.Title != "中文标题" {
		t.Errorf("Title = %q, want %q", derived.Title, "中文标题")
	}
	if derived.Description != "Default Description" {
		t.Errorf("Description = %q, want inherited %q", derived.Description, "Default Description")
	}
	if derived.LanguageCode != "zh" {
		t.Errorf("LanguageCode = %q, want %q", derived.LanguageCode, "zh")
	}
	if derived.ActiveLanguage != "zh" {
		t.Errorf("ActiveLanguage = %q, want %q", derived.ActiveLanguage, "zh")
	}
	if derived.RootBaseURL != "https://example.com" {
		t.Errorf("RootBaseURL = %q, want %q", derived.RootBaseURL, "https://example.com")
	}
	if derived.RootLanguageCode != "en" {
		t.Errorf("RootLanguageCode = %q, want %q", derived.RootLanguageCode, "en")
	}

	wantParams := map[string]interface{}{"shared": "top", "override": "lang", "extra": "lang"}
	if !reflect.DeepEqual(derived.Params, wantParams) {
		t.Errorf("Params = %v, want %v", derived.Params, wantParams)
	}
	wantStrings := map[string]string{"readMore": "阅读更多", "shared": "top"}
	if !reflect.DeepEqual(derived.Strings, wantStrings) {
		t.Errorf("Strings = %v, want %v", derived.Strings, wantStrings)
	}

	// The original config must not be mutated by the merge.
	if cfg.Params["override"] != "top" {
		t.Errorf("original Params mutated: %v", cfg.Params)
	}
	if cfg.Strings["readMore"] != "Read more" {
		t.Errorf("original Strings mutated: %v", cfg.Strings)
	}
	if cfg.ActiveLanguage != "" {
		t.Errorf("original ActiveLanguage = %q, want empty", cfg.ActiveLanguage)
	}
}

func TestNormalize_LeavesLanguagesUntouched(t *testing.T) {
	cfg := Normalize(&Config{
		Languages: map[string]*LanguageConfig{"zh": {}},
	})
	if cfg.Languages["zh"].ContentDir != "" {
		t.Errorf("Normalize filled language ContentDir: %q", cfg.Languages["zh"].ContentDir)
	}
	if cfg.Languages["zh"].LanguageCode != "" {
		t.Errorf("Normalize filled language LanguageCode: %q", cfg.Languages["zh"].LanguageCode)
	}
}

func TestValidateLanguages(t *testing.T) {
	tests := []struct {
		name      string
		languages map[string]*LanguageConfig
		mutate    func(*Config)
		wantErr   string
	}{
		{
			name:      "valid convention dirs",
			languages: map[string]*LanguageConfig{"zh": {}, "ja": {}},
		},
		{
			name:      "invalid key uppercase",
			languages: map[string]*LanguageConfig{"ZH": {}},
			wantErr:   "invalid language key",
		},
		{
			name:      "invalid key slash",
			languages: map[string]*LanguageConfig{"zh/cn": {}},
			wantErr:   "invalid language key",
		},
		{
			name:      "key conflicts with paginatePath",
			languages: map[string]*LanguageConfig{"page": {}},
			wantErr:   "conflicts with paginatePath",
		},
		{
			name:      "key conflicts with tags",
			languages: map[string]*LanguageConfig{"tags": {}},
			wantErr:   "conflicts with taxonomy",
		},
		{
			name:      "key conflicts with staticDir",
			languages: map[string]*LanguageConfig{"assets": {}},
			wantErr:   "conflicts with staticDir",
		},
		{
			name:      "key conflicts with publishDir",
			languages: map[string]*LanguageConfig{"public": {}},
			wantErr:   "conflicts with publishDir",
		},
		{
			name: "contentDir overlaps publishDir",
			languages: map[string]*LanguageConfig{
				"zh": {ContentDir: "public/zh"},
			},
			wantErr: "",
			mutate:  nil, // nested under publishDir is allowed; only equality is rejected
		},
		{
			name: "contentDir equals publishDir",
			languages: map[string]*LanguageConfig{
				"zh": {ContentDir: "public"},
			},
			wantErr: "must not overlap publishDir",
		},
		{
			name: "contentDir equals default contentDir",
			languages: map[string]*LanguageConfig{
				"zh": {ContentDir: "_posts"},
			},
			wantErr: "must differ from the default",
		},
		{
			name: "two languages share contentDir",
			languages: map[string]*LanguageConfig{
				"zh": {ContentDir: "content/shared"},
				"ja": {ContentDir: "content/shared"},
			},
			wantErr: "already used by language",
		},
		{
			name: "contentDir contains default contentDir",
			languages: map[string]*LanguageConfig{
				"zh": {ContentDir: "."},
			},
			wantErr: "must not contain the default",
		},
		{
			name: "pageDir contains default pageDir",
			languages: map[string]*LanguageConfig{
				"zh": {PageDir: "."},
			},
			wantErr: "must not contain the default",
		},
		{
			name: "language contentDir nests inside another language's",
			languages: map[string]*LanguageConfig{
				"zh": {ContentDir: "content"},
				"ja": {ContentDir: "content/ja"},
			},
			wantErr: "must not nest",
		},
		{
			name: "language contentDir contains another language's",
			languages: map[string]*LanguageConfig{
				"ja": {ContentDir: "content/ja"},
				"zh": {ContentDir: "content"},
			},
			wantErr: "must not nest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Normalize(&Config{Languages: tt.languages})
			if tt.mutate != nil {
				tt.mutate(cfg)
			}
			err := validateLanguages(cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateLanguages() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateLanguages() = nil, want error containing %q", tt.wantErr)
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("validateLanguages() error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}
