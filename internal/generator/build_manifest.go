package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

// buildManifestName is the on-disk filename for the incremental build cache,
// written to the publish directory.
const buildManifestName = ".gobin-build.json"

// buildManifestVersion is bumped whenever the schema changes incompatibly.
// Mismatched versions cause the manifest to be ignored (clean fallback).
//
//	v1 - initial schema (source_hash + single aggregate_hash).
//	v2 - per-category aggregate hashes (list/feed/search/sitemap) so a
//	     post body edit no longer invalidates list / sitemap output.
const buildManifestVersion = 2

// BuildManifest captures the per-source content fingerprints needed to skip
// unchanged work on subsequent builds.
type BuildManifest struct {
	Version      int                      `json:"version"`
	BuildEnvHash string                   `json:"build_env_hash"`
	Posts        []BuildManifestPostEntry `json:"posts"`
	Pages        []BuildManifestPageEntry `json:"pages"`
}

// BuildManifestPostEntry tracks a single post source / output pair.
//
// SourceHash drives single-post-page invalidation: any file-byte difference
// (front-matter reformatting, body edit, trailing whitespace) flips it.
//
// The four category hashes below describe what each aggregate consumes from
// this post. They derive from the parsed Post (not file bytes), so cosmetic
// edits that leave the parsed semantics intact do not invalidate aggregates.
//
//	ListHash    - title/date/url/summary/description/tags/categories/draft/published
//	              used by list, taxonomy, and 404 page invalidation.
//	FeedHash    - ListHash inputs + content (RSS/Atom carry post bodies).
//	SearchHash  - title/description/tags/categories/summary/content/draft/published
//	SitemapHash - url/lastmod/date/draft/published
type BuildManifestPostEntry struct {
	SourcePath  string `json:"source_path"`
	SourceHash  string `json:"source_hash"`
	OutputPath  string `json:"output_path"`
	ListHash    string `json:"list_hash"`
	FeedHash    string `json:"feed_hash"`
	SearchHash  string `json:"search_hash"`
	SitemapHash string `json:"sitemap_hash"`
}

// BuildManifestPageEntry tracks a single standalone page source / output pair.
type BuildManifestPageEntry struct {
	SourcePath string `json:"source_path"`
	SourceHash string `json:"source_hash"`
	OutputPath string `json:"output_path"`
}

// readBuildManifest loads the manifest from outputDir, returning an empty
// (zero-value) manifest if the file is missing or unreadable. A non-nil error
// is returned only for unexpected I/O errors. Corrupted manifest files (bad
// JSON or mismatched version) silently fall back to the empty value so the
// caller degrades to a full build.
func readBuildManifest(outputDir string) (*BuildManifest, error) {
	path := filepath.Join(outputDir, buildManifestName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &BuildManifest{Version: buildManifestVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read build manifest: %w", err)
	}

	manifest := &BuildManifest{}
	if jsonErr := json.Unmarshal(data, manifest); jsonErr != nil {
		return &BuildManifest{Version: buildManifestVersion}, nil
	}
	if manifest.Version != buildManifestVersion {
		return &BuildManifest{Version: buildManifestVersion}, nil
	}
	return manifest, nil
}

// writeBuildManifest serializes the manifest into outputDir. The manifest is
// canonicalized (entries sorted by source path) so repeated builds produce
// byte-identical files when inputs are unchanged.
func writeBuildManifest(outputDir string, manifest *BuildManifest) error {
	if manifest == nil {
		return nil
	}
	manifest.Version = buildManifestVersion
	sort.Slice(manifest.Posts, func(i, j int) bool {
		return manifest.Posts[i].SourcePath < manifest.Posts[j].SourcePath
	})
	sort.Slice(manifest.Pages, func(i, j int) bool {
		return manifest.Pages[i].SourcePath < manifest.Pages[j].SourcePath
	})

	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode build manifest: %w", err)
	}
	content = append(content, '\n')

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create publish dir for manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(outputDir, buildManifestName), content, 0644)
}

// hashFile returns the 12-character SHA-256 prefix of the file at path. The
// short prefix is sufficient for change detection across builds and keeps the
// manifest compact.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}

// computeBuildEnvHash captures every input outside of individual content
// files that can change rendered output: config fields, render options,
// template files (site + theme), and active theme assets. A change to any of
// these forces the next build to ignore the previous manifest.
func computeBuildEnvHash(cfg *config.Config, opts parser.RenderOptions) (string, error) {
	parts := make([]string, 0, 8)

	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	parts = append(parts, "config="+hashBytes(cfgBytes))

	optsBytes, err := json.Marshal(opts)
	if err != nil {
		return "", fmt.Errorf("encode render options: %w", err)
	}
	parts = append(parts, "render="+hashBytes(optsBytes))

	tmplHash, err := hashDirectoryTree("templates")
	if err != nil {
		return "", fmt.Errorf("hash templates: %w", err)
	}
	parts = append(parts, "templates="+tmplHash)

	if cfg != nil && cfg.Theme != "" && cfg.ThemesDir != "" {
		themeDir := filepath.Join(cfg.ThemesDir, cfg.Theme)
		themeHash, err := hashDirectoryTree(themeDir)
		if err != nil {
			return "", fmt.Errorf("hash theme: %w", err)
		}
		parts = append(parts, "theme="+themeHash)
	}

	return hashBytes([]byte(strings.Join(parts, "|"))), nil
}

// buildManifestForRun builds the manifest that should be written after a
// successful generation. It records source content hashes and output paths
// for every post and standalone page that participates in the build, plus a
// build_env_hash that pins config, render options, and template/theme dir
// state so the next build can detect global invalidation.
func buildManifestForRun(cfg *config.Config, posts []*parser.Post, pages []*parser.Page) (*BuildManifest, error) {
	envHash, err := computeBuildEnvHash(cfg, renderOptionsForCfg(cfg))
	if err != nil {
		return nil, err
	}

	manifest := &BuildManifest{
		Version:      buildManifestVersion,
		BuildEnvHash: envHash,
		Posts:        make([]BuildManifestPostEntry, 0, len(posts)),
		Pages:        make([]BuildManifestPageEntry, 0, len(pages)),
	}

	for _, post := range posts {
		if post == nil || post.FilePath == "" {
			continue
		}
		hash, err := hashFile(post.FilePath)
		if err != nil {
			return nil, fmt.Errorf("hash post %s: %w", post.FilePath, err)
		}
		category := computePostCategoryHashes(post)
		manifest.Posts = append(manifest.Posts, BuildManifestPostEntry{
			SourcePath:  filepath.ToSlash(post.FilePath),
			SourceHash:  hash,
			OutputPath:  postOutputPath(post),
			ListHash:    category.List,
			FeedHash:    category.Feed,
			SearchHash:  category.Search,
			SitemapHash: category.Sitemap,
		})
	}

	for _, page := range pages {
		if page == nil || page.FilePath == "" {
			continue
		}
		hash, err := hashFile(page.FilePath)
		if err != nil {
			return nil, fmt.Errorf("hash page %s: %w", page.FilePath, err)
		}
		manifest.Pages = append(manifest.Pages, BuildManifestPageEntry{
			SourcePath: filepath.ToSlash(page.FilePath),
			SourceHash: hash,
			OutputPath: filepath.ToSlash(standalonePageOutputPath(page.URL)),
		})
	}

	return manifest, nil
}

func postOutputPath(post *parser.Post) string {
	url := strings.TrimPrefix(strings.TrimSuffix(post.URL, "/"), "/")
	if url == "" {
		return "index.html"
	}
	return url + "/index.html"
}

// renderOptionsForCfg mirrors cmd/gobin/commands.renderOptionsFromConfig but
// stays inside the generator package so build_env_hash is reproducible
// without importing the CLI layer.
func renderOptionsForCfg(cfg *config.Config) parser.RenderOptions {
	opts := parser.DefaultRenderOptions()
	if cfg != nil && cfg.Markup != nil && cfg.Markup.AllowUnsafeHTML != nil {
		opts.AllowUnsafeHTML = *cfg.Markup.AllowUnsafeHTML
	}
	return opts
}

// postCategoryHashes folds a parsed Post's per-aggregate inputs into one
// short digest per category. The values are intentionally derived from the
// parsed Post struct (not from raw file bytes) so cosmetic edits — front
// matter reformatting, trailing whitespace, YAML key reorder — do not
// invalidate any aggregate, while changes that alter the rendered semantics
// do.
type postCategoryHashes struct {
	List    string
	Feed    string
	Search  string
	Sitemap string
}

func computePostCategoryHashes(post *parser.Post) postCategoryHashes {
	if post == nil {
		return postCategoryHashes{}
	}
	listKey := joinHashParts(
		post.Title,
		post.Date.UTC().Format("2006-01-02T15:04:05Z"),
		post.LastMod.UTC().Format("2006-01-02T15:04:05Z"),
		post.URL,
		post.Summary,
		post.Description,
		strings.Join(post.Tags, ","),
		strings.Join(post.Categories, ","),
		boolStr(post.Draft),
		publishedStr(post.Published),
		strings.Join(post.Aliases, ","),
	)
	listHash := hashBytes([]byte(listKey))

	feedHash := hashBytes([]byte(joinHashParts(listKey, post.ContentHTML)))
	searchHash := hashBytes([]byte(joinHashParts(
		post.Title,
		post.Description,
		strings.Join(post.Tags, ","),
		strings.Join(post.Categories, ","),
		post.Summary,
		post.Content,
		boolStr(post.Draft),
		publishedStr(post.Published),
	)))
	sitemapHash := hashBytes([]byte(joinHashParts(
		post.URL,
		post.LastMod.UTC().Format("2006-01-02T15:04:05Z"),
		post.Date.UTC().Format("2006-01-02T15:04:05Z"),
		boolStr(post.Draft),
		publishedStr(post.Published),
	)))

	return postCategoryHashes{
		List:    listHash,
		Feed:    feedHash,
		Search:  searchHash,
		Sitemap: sitemapHash,
	}
}

func joinHashParts(parts ...string) string {
	return strings.Join(parts, "\x1f")
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func publishedStr(p *bool) string {
	if p == nil {
		return "unset"
	}
	return boolStr(*p)
}

// ListStateHash, FeedStateHash, SearchStateHash, and SitemapStateHash each
// return a digest of the contribution every post + page makes to the
// corresponding aggregate output. Two builds with matching category state
// hashes can skip the artifacts that consume that category.
//
// Standalone pages contribute their source hash to every category (they
// rarely participate in feeds/sitemaps directly, but a page being added
// or removed should at minimum re-render list pages, and we keep the rule
// uniform across categories for simplicity).
func (m *BuildManifest) ListStateHash() string    { return m.categoryStateHash("list") }
func (m *BuildManifest) FeedStateHash() string    { return m.categoryStateHash("feed") }
func (m *BuildManifest) SearchStateHash() string  { return m.categoryStateHash("search") }
func (m *BuildManifest) SitemapStateHash() string { return m.categoryStateHash("sitemap") }

func (m *BuildManifest) categoryStateHash(category string) string {
	if m == nil {
		return ""
	}
	parts := make([]string, 0, len(m.Posts)+len(m.Pages))
	for _, entry := range m.Posts {
		var h string
		switch category {
		case "list":
			h = entry.ListHash
		case "feed":
			h = entry.FeedHash
		case "search":
			h = entry.SearchHash
		case "sitemap":
			h = entry.SitemapHash
		}
		parts = append(parts, "post:"+entry.OutputPath+":"+h)
	}
	for _, entry := range m.Pages {
		parts = append(parts, "page:"+entry.OutputPath+":"+entry.SourceHash)
	}
	sort.Strings(parts)
	return hashBytes([]byte(strings.Join(parts, "|")))
}

// applyIncrementalSkips loads the previous manifest from outputDir and marks
// pages and aggregate artifacts whose inputs are unchanged so the render
// pipeline can skip them.
//
// Skip rules:
//   - The previous manifest must exist and have the same build_env_hash as
//     the current build. Any drift forces a full render.
//   - Single-content pages (post + standalone page) are skipped when
//     source_hash and the on-disk output both match.
//   - List / taxonomy / 404 pages are aggregate-driven and skip when
//     ListStateHash matches between the two builds.
//   - Each aggregate artifact (feed, sitemap, search, aliases, robots) is
//     skipped independently based on its own per-category state hash.
//     This lets body edits flow into feed / search without invalidating
//     list / sitemap.
func applyIncrementalSkips(plan *generationPlan, outputDir string, current *BuildManifest) {
	if plan == nil || current == nil {
		return
	}
	previous, err := readBuildManifest(outputDir)
	if err != nil || previous == nil {
		return
	}
	if previous.BuildEnvHash == "" || previous.BuildEnvHash != current.BuildEnvHash {
		return
	}

	postOutputToHash := make(map[string]string, len(previous.Posts))
	for _, entry := range previous.Posts {
		postOutputToHash[entry.OutputPath] = entry.SourceHash
	}
	pageOutputToHash := make(map[string]string, len(previous.Pages))
	for _, entry := range previous.Pages {
		pageOutputToHash[entry.OutputPath] = entry.SourceHash
	}

	currentPostHash := make(map[string]string, len(current.Posts))
	for _, entry := range current.Posts {
		currentPostHash[entry.OutputPath] = entry.SourceHash
	}
	currentPageHash := make(map[string]string, len(current.Pages))
	for _, entry := range current.Pages {
		currentPageHash[entry.OutputPath] = entry.SourceHash
	}

	listUnchanged := previous.ListStateHash() == current.ListStateHash()
	feedUnchanged := previous.FeedStateHash() == current.FeedStateHash()
	searchUnchanged := previous.SearchStateHash() == current.SearchStateHash()
	sitemapUnchanged := previous.SitemapStateHash() == current.SitemapStateHash()

	for i := range plan.pagePlan.pages {
		spec := &plan.pagePlan.pages[i]
		outputPath, err := safeOutputPath(outputDir, spec.OutputPath)
		if err != nil {
			continue
		}
		if info, statErr := os.Stat(outputPath); statErr != nil || info.IsDir() {
			continue
		}

		if h, hit := currentPostHash[spec.OutputPath]; hit {
			if prev, ok := postOutputToHash[spec.OutputPath]; ok && prev == h && h != "" {
				spec.SkipReason = "unchanged-source"
			}
			continue
		}
		if h, hit := currentPageHash[spec.OutputPath]; hit {
			if prev, ok := pageOutputToHash[spec.OutputPath]; ok && prev == h && h != "" {
				spec.SkipReason = "unchanged-source"
			}
			continue
		}
		// Aggregate-driven pages (list, taxonomy, 404). They re-render only
		// when ListStateHash changes — title/date/url/summary/tags/categories
		// drive the rendered output, so a pure body edit leaves them stable.
		if listUnchanged {
			spec.SkipReason = "unchanged-aggregate"
		}
	}

	for i := range plan.artifacts.specs {
		spec := &plan.artifacts.specs[i]
		switch spec.Name {
		case "feed":
			if feedUnchanged {
				spec.SkipReason = "unchanged-aggregate"
			}
		case "search":
			if searchUnchanged {
				spec.SkipReason = "unchanged-aggregate"
			}
		case "sitemap":
			if sitemapUnchanged {
				spec.SkipReason = "unchanged-aggregate"
			}
		case "aliases", "robots":
			if listUnchanged {
				spec.SkipReason = "unchanged-aggregate"
			}
		}
	}
}

func isAggregateArtifact(name string) bool {
	switch name {
	case "feed", "sitemap", "search", "aliases", "robots":
		return true
	default:
		return false
	}
}
func hashDirectoryTree(root string) (string, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", root)
	}

	type entry struct {
		path string
		hash string
	}
	var entries []entry

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		h, err := hashFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{path: filepath.ToSlash(rel), hash: h})
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})

	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, e.path+":"+e.hash)
	}
	return hashBytes([]byte(strings.Join(parts, "\n"))), nil
}
