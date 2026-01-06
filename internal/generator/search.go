package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// SearchIndex represents a search index for Lunr.js or Fuse.js
type SearchIndex struct {
	Index []SearchDocument `json:"index"`
}

// SearchDocument represents a document in the search index
type SearchDocument struct {
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Date     string   `json:"date"`
	Tags     []string `json:"tags"`
	Summary  string   `json:"summary"`
	Content  string   `json:"content,omitempty"`
	Author   string   `json:"author,omitempty"`
	Category string   `json:"category,omitempty"`
}

// GenerateSearchIndex generates a search index JSON file
func GenerateSearchIndex(posts []*parser.Post, cfg *config.Config, outputDir string) error {
	index := SearchIndex{
		Index: []SearchDocument{},
	}

	// Limit indexed content to avoid huge files
	maxContentLength := 2000

	for _, post := range posts {
		// Skip draft posts
		if post.Draft {
			continue
		}

		doc := SearchDocument{
			Title:   post.Title,
			URL:     post.URL,
			Date:    post.Date.Format("2006-01-02"),
			Tags:    post.Tags,
			Summary: post.Summary,
			Author:  cfg.Author,
		}

		// Add first category if available
		if len(post.Categories) > 0 {
			doc.Category = post.Categories[0]
		}

		// Optionally include full content (with length limit)
		if len(post.Content) > maxContentLength {
			doc.Content = post.Content[:maxContentLength] + "..."
		} else {
			doc.Content = post.Content
		}

		// Clean content (remove excessive whitespace)
		doc.Content = strings.Join(strings.Fields(doc.Content), " ")

		index.Index = append(index.Index, doc)
	}

	// Create output file
	outputPath := filepath.Join(outputDir, "search-index.json")
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create search index file: %w", err)
	}
	defer file.Close()

	// Encode JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(index); err != nil {
		return fmt.Errorf("failed to encode search index: %w", err)
	}

	return nil
}

// GenerateSearchIndexMin generates a minimal search index (only metadata, no content)
func GenerateSearchIndexMin(posts []*parser.Post, cfg *config.Config, outputDir string) error {
	index := SearchIndex{
		Index: []SearchDocument{},
	}

	for _, post := range posts {
		// Skip draft posts
		if post.Draft {
			continue
		}

		doc := SearchDocument{
			Title:   post.Title,
			URL:     post.URL,
			Date:    post.Date.Format("2006-01-02"),
			Tags:    post.Tags,
			Summary: post.Summary,
			Author:  cfg.Author,
		}

		// Add first category if available
		if len(post.Categories) > 0 {
			doc.Category = post.Categories[0]
		}

		index.Index = append(index.Index, doc)
	}

	// Create output file
	outputPath := filepath.Join(outputDir, "search-index-min.json")
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create minimal search index file: %w", err)
	}
	defer file.Close()

	// Encode JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(index); err != nil {
		return fmt.Errorf("failed to encode minimal search index: %w", err)
	}

	return nil
}

// GenerateSearch generates both full and minimal search indexes
func GenerateSearch(posts []*parser.Post, cfg *config.Config, outputDir string) error {
	// Generate full search index with content
	if err := GenerateSearchIndex(posts, cfg, outputDir); err != nil {
		return fmt.Errorf("failed to generate search index: %w", err)
	}

	// Generate minimal search index (metadata only)
	if err := GenerateSearchIndexMin(posts, cfg, outputDir); err != nil {
		return fmt.Errorf("failed to generate minimal search index: %w", err)
	}

	return nil
}
