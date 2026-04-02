package generator

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// RSSFeed represents an RSS 2.0 feed
type RSSFeed struct {
	XMLName      xml.Name   `xml:"rss"`
	Version      string     `xml:"version,attr"`
	XmlnsAtom    string     `xml:"xmlns:atom,attr"`
	XmlnsContent string     `xml:"xmlns:content,attr"`
	Channel      RSSChannel `xml:"channel"`
}

// RSSChannel represents the channel element in RSS
type RSSChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language,omitempty"`
	PubDate       string    `xml:"pubDate"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Generator     string    `xml:"generator"`
	AtomLink      AtomLink  `xml:"atom:link"`
	ManaginEditor string    `xml:"managingEditor,omitempty"`
	WebMaster     string    `xml:"webMaster,omitempty"`
	Items         []RSSItem `xml:"item"`
}

// AtomLink represents an atom link in RSS
type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

// RSSItem represents an item in RSS feed
type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Author      string `xml:"author,omitempty"`
	Category    string `xml:"category,omitempty"`
	PubDate     string `xml:"pubDate"`
	GUID        GUID   `xml:"guid"`
	Content     string `xml:"content:encoded,omitempty"`
}

// GUID represents a globally unique identifier
type GUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr,omitempty"`
}

// AtomFeed represents an Atom 1.0 feed
type AtomFeed struct {
	XMLName   xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title     string      `xml:"title"`
	Subtitle  string      `xml:"subtitle,omitempty"`
	ID        string      `xml:"id"`
	Updated   string      `xml:"updated"`
	Generator Generator   `xml:"generator"`
	Link      []Link      `xml:"link"`
	Author    Author      `xml:"author,omitempty"`
	Entries   []AtomEntry `xml:"entry"`
}

// Generator represents the generator element
type Generator struct {
	Value   string `xml:",chardata"`
	Version string `xml:"version,attr,omitempty"`
	URI     string `xml:"uri,attr,omitempty"`
}

// Link represents a link element
type Link struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

// Author represents an author element
type Author struct {
	Name  string `xml:"name"`
	Email string `xml:"email,omitempty"`
	URI   string `xml:"uri,omitempty"`
}

// AtomEntry represents an entry in Atom feed
type AtomEntry struct {
	Title     string `xml:"title"`
	Link      Link   `xml:"link"`
	ID        string `xml:"id"`
	Updated   string `xml:"updated"`
	Published string `xml:"published,omitempty"`
	Summary   string `xml:"summary,omitempty"`
	Content   string `xml:"content,omitempty"`
	Author    Author `xml:"author,omitempty"`
}

// GenerateRSSFeed generates an RSS 2.0 feed
func GenerateRSSFeed(posts []*parser.Post, cfg *config.Config, outputDir string) error {
	if len(posts) == 0 {
		return nil
	}

	sortedPosts := sortedPostsByDateDesc(posts)

	now := time.Now().UTC()
	buildDate := now.Format(time.RFC1123Z)

	feed := RSSFeed{
		Version:      "2.0",
		XmlnsAtom:    "http://www.w3.org/2005/Atom",
		XmlnsContent: "http://purl.org/rss/1.0/modules/content/",
		Channel: RSSChannel{
			Title:         cfg.Title,
			Link:          cfg.BaseURL,
			Description:   cfg.Description,
			Language:      cfg.LanguageCode,
			PubDate:       buildDate,
			LastBuildDate: buildDate,
			Generator:     "gobin",
			AtomLink: AtomLink{
				Href: joinURL(cfg.BaseURL, "index.xml"),
				Rel:  "self",
				Type: "application/rss+xml",
			},
		},
	}

	// Limit to most recent 50 posts
	maxPosts := 50
	if len(sortedPosts) < maxPosts {
		maxPosts = len(sortedPosts)
	}

	for _, post := range sortedPosts[:maxPosts] {
		item := RSSItem{
			Title:       post.Title,
			Link:        joinURL(cfg.BaseURL, post.URL),
			Description: post.Summary,
			Author:      cfg.Author,
			PubDate:     post.Date.UTC().Format(time.RFC1123Z),
			GUID: GUID{
				Value:       joinURL(cfg.BaseURL, post.URL),
				IsPermaLink: "true",
			},
			Content: post.ContentHTML,
		}

		// Add first category if available
		if len(post.Categories) > 0 {
			item.Category = post.Categories[0]
		}

		feed.Channel.Items = append(feed.Channel.Items, item)
	}

	// Create output file
	outputPath := filepath.Join(outputDir, "index.xml")
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create RSS feed file: %w", err)
	}
	defer file.Close()

	// Write XML header
	if _, err := file.WriteString(xml.Header); err != nil {
		return err
	}

	// Encode XML
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	if err := encoder.Encode(feed); err != nil {
		return fmt.Errorf("failed to encode RSS feed: %w", err)
	}

	return nil
}

// GenerateAtomFeed generates an Atom 1.0 feed
func GenerateAtomFeed(posts []*parser.Post, cfg *config.Config, outputDir string) error {
	if len(posts) == 0 {
		return nil
	}

	sortedPosts := sortedPostsByDateDesc(posts)

	now := time.Now().UTC()
	updated := now.Format(time.RFC3339)

	feed := AtomFeed{
		Title:    cfg.Title,
		Subtitle: cfg.Description,
		ID:       cfg.BaseURL + "/",
		Updated:  updated,
		Generator: Generator{
			Value:   "gobin",
			URI:     "https://github.com/mengbin92/gobin",
			Version: "1.0",
		},
		Link: []Link{
			{
				Href: cfg.BaseURL,
				Rel:  "alternate",
			},
			{
				Href: joinURL(cfg.BaseURL, "index.atom"),
				Rel:  "self",
				Type: "application/atom+xml",
			},
		},
		Author: Author{
			Name: cfg.Author,
		},
	}

	// Limit to most recent 50 posts
	maxPosts := 50
	if len(sortedPosts) < maxPosts {
		maxPosts = len(sortedPosts)
	}

	for _, post := range sortedPosts[:maxPosts] {
		entry := AtomEntry{
			Title: post.Title,
			Link: Link{
				Href: joinURL(cfg.BaseURL, post.URL),
				Rel:  "alternate",
			},
			ID:        joinURL(cfg.BaseURL, post.URL),
			Updated:   post.Date.UTC().Format(time.RFC3339),
			Published: post.Date.UTC().Format(time.RFC3339),
			Summary:   post.Summary,
			Content:   post.ContentHTML,
			Author: Author{
				Name: cfg.Author,
			},
		}

		feed.Entries = append(feed.Entries, entry)
	}

	// Create output file
	outputPath := filepath.Join(outputDir, "index.atom")
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create Atom feed file: %w", err)
	}
	defer file.Close()

	// Write XML header
	if _, err := file.WriteString(xml.Header); err != nil {
		return err
	}

	// Encode XML
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	if err := encoder.Encode(feed); err != nil {
		return fmt.Errorf("failed to encode Atom feed: %w", err)
	}

	return nil
}

// GenerateFeeds generates both RSS and Atom feeds
func GenerateFeeds(posts []*parser.Post, cfg *config.Config, outputDir string) error {
	// Generate RSS feed
	if err := GenerateRSSFeed(posts, cfg, outputDir); err != nil {
		return fmt.Errorf("failed to generate RSS feed: %w", err)
	}

	// Generate Atom feed
	if err := GenerateAtomFeed(posts, cfg, outputDir); err != nil {
		return fmt.Errorf("failed to generate Atom feed: %w", err)
	}

	return nil
}

func sortedPostsByDateDesc(posts []*parser.Post) []*parser.Post {
	cloned := append([]*parser.Post(nil), posts...)
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].Date.After(cloned[j].Date)
	})
	return cloned
}
