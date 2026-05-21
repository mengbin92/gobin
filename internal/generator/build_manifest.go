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
const buildManifestVersion = 1

// BuildManifest captures the per-source content fingerprints needed to skip
// unchanged work on subsequent builds.
type BuildManifest struct {
	Version      int                      `json:"version"`
	BuildEnvHash string                   `json:"build_env_hash"`
	Posts        []BuildManifestPostEntry `json:"posts"`
	Pages        []BuildManifestPageEntry `json:"pages"`
}

// BuildManifestPostEntry tracks a single post source / output pair.
type BuildManifestPostEntry struct {
	SourcePath    string `json:"source_path"`
	SourceHash    string `json:"source_hash"`
	OutputPath    string `json:"output_path"`
	AggregateHash string `json:"aggregate_hash"`
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
		manifest.Posts = append(manifest.Posts, BuildManifestPostEntry{
			SourcePath:    filepath.ToSlash(post.FilePath),
			SourceHash:    hash,
			OutputPath:    postOutputPath(post),
			AggregateHash: hash,
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
