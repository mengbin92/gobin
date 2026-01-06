package generator

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// Sitemap represents a sitemap.xml file
type Sitemap struct {
	XMLName xml.Name    `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []SitemapURL `xml:"url"`
}

// SitemapURL represents a URL entry in the sitemap
type SitemapURL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty"`
	Priority   float32 `xml:"priority,omitempty"`
}

// GenerateSitemap generates a sitemap.xml file
func GenerateSitemap(posts []*parser.Post, cfg *config.Config, outputDir string, tagList, categoryList []string) error {
	if cfg.BaseURL == "" {
		return fmt.Errorf("baseURL is required for sitemap generation")
	}

	now := time.Now().Format("2006-01-02")

	sitemap := Sitemap{
		URLs: []SitemapURL{
			{
				Loc:        cfg.BaseURL,
				LastMod:    now,
				ChangeFreq: "daily",
				Priority:   1.0,
			},
		},
	}

	// Add post URLs
	for _, post := range posts {
		sitemap.URLs = append(sitemap.URLs, SitemapURL{
			Loc:        fmt.Sprintf("%s%s", cfg.BaseURL, post.URL),
			LastMod:    post.Date.Format("2006-01-02"),
			ChangeFreq: "monthly",
			Priority:   0.8,
		})
	}

	// Add pagination URLs (estimate max pages)
	if cfg.Paginate > 0 && len(posts) > cfg.Paginate {
		maxPages := (len(posts) + cfg.Paginate - 1) / cfg.Paginate
		for i := 2; i <= maxPages; i++ {
			sitemap.URLs = append(sitemap.URLs, SitemapURL{
				Loc:        fmt.Sprintf("%s/%s/%d/", cfg.BaseURL, cfg.PaginatePath, i),
				LastMod:    now,
				ChangeFreq: "daily",
				Priority:   0.6,
			})
		}
	}

	// Add tag index and individual tag pages
	sitemap.URLs = append(sitemap.URLs, SitemapURL{
		Loc:        fmt.Sprintf("%s/tags/", cfg.BaseURL),
		LastMod:    now,
		ChangeFreq: "weekly",
		Priority:   0.5,
	})

	for _, tag := range tagList {
		sitemap.URLs = append(sitemap.URLs, SitemapURL{
			Loc:        fmt.Sprintf("%s/tags/%s/", cfg.BaseURL, urlize(tag)),
			LastMod:    now,
			ChangeFreq: "weekly",
			Priority:   0.5,
		})
	}

	// Add category index and individual category pages
	sitemap.URLs = append(sitemap.URLs, SitemapURL{
		Loc:        fmt.Sprintf("%s/categories/", cfg.BaseURL),
		LastMod:    now,
		ChangeFreq: "weekly",
		Priority:   0.5,
	})

	for _, category := range categoryList {
		sitemap.URLs = append(sitemap.URLs, SitemapURL{
			Loc:        fmt.Sprintf("%s/categories/%s/", cfg.BaseURL, urlize(category)),
			LastMod:    now,
			ChangeFreq: "weekly",
			Priority:   0.5,
		})
	}

	// Create output file
	outputPath := filepath.Join(outputDir, "sitemap.xml")
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create sitemap file: %w", err)
	}
	defer file.Close()

	// Write XML header
	if _, err := file.WriteString(xml.Header); err != nil {
		return err
	}

	// Encode XML
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	if err := encoder.Encode(sitemap); err != nil {
		return fmt.Errorf("failed to encode sitemap: %w", err)
	}

	return nil
}
