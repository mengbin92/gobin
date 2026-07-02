package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mengbin92/gobin/internal/log"
)

// assetCategory classifies a static asset by extension. v1.5.0 uses this
// to drive the default filename-fingerprint extension set and to label
// asset-related log output.
type assetCategory string

const (
	categoryCSS   assetCategory = "css"
	categoryJS    assetCategory = "js"
	categoryImage assetCategory = "image"
	categoryOther assetCategory = "other"
)

// PostprocessStats summarizes the post-render HTML reference rewrite.
type PostprocessStats struct {
	HTMLFilesScanned    int
	HTMLFilesChanged    int
	ReferencesFound     int
	ReferencesRewritten int
}

// PostprocessOptions controls a single postprocess pass.
type PostprocessOptions struct {
	OutputDir string
	// LogicalToOutput maps logical asset paths (no leading slash, e.g.
	// "css/site.css") to their on-disk output paths (e.g.
	// "css/site.abc123.css"). Entries where the two values are equal are
	// skipped. Only filename-level fingerprinting produces entries.
	LogicalToOutput map[string]string
}

// PostprocessHTML rewrites HTML href / src attributes in rendered output
// to point at fingerprinted asset paths. It is a post-render step: it walks
// the existing files under OutputDir and rewrites in place.
//
// Scope: HTML only. CSS url() and JS string references are out of scope
// for v1.5.0 (see spec §5).
func PostprocessHTML(opts PostprocessOptions) (PostprocessStats, error) {
	logger := log.GetDefault().With("component", "generator")
	stats := PostprocessStats{}

	rewriteSet := make(map[string]string, len(opts.LogicalToOutput))
	for logical, output := range opts.LogicalToOutput {
		if logical == output {
			continue
		}
		rewriteSet[logical] = output
	}
	if len(rewriteSet) == 0 {
		logger.Debug("postprocess: no rewrite entries, skipping")
		return stats, nil
	}

	// Pre-build a single regex that matches href= or src= against any of
	// the rewrite keys. Keys are pre-escaped and sorted by descending
	// length so that /css/site.css matches before /css/site.
	rewriteKeys := make([]string, 0, len(rewriteSet))
	for k := range rewriteSet {
		rewriteKeys = append(rewriteKeys, "/"+k)
	}
	sort.Slice(rewriteKeys, func(i, j int) bool {
		return len(rewriteKeys[i]) > len(rewriteKeys[j])
	})

	pattern := buildHTMLRefPattern(rewriteKeys)
	if pattern == nil {
		return stats, nil
	}

	err := filepath.WalkDir(opts.OutputDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".html" {
			return nil
		}

		stats.HTMLFilesScanned++

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		rewritten, found, replaced := rewriteHTMLReferences(string(content), pattern, rewriteSet)
		stats.ReferencesFound += found
		stats.ReferencesRewritten += replaced

		if replaced == 0 {
			return nil
		}

		if writeErr := os.WriteFile(path, []byte(rewritten), 0644); writeErr != nil {
			return writeErr
		}
		stats.HTMLFilesChanged++

		rel, _ := filepath.Rel(opts.OutputDir, path)
		logger.Debug("postprocess: rewrote HTML references",
			"file", rel,
			"rewrites", replaced,
		)
		return nil
	})
	if err != nil {
		return stats, fmt.Errorf("postprocess: %w", err)
	}
	return stats, nil
}

func buildHTMLRefPattern(keys []string) *regexp.Regexp {
	if len(keys) == 0 {
		return nil
	}
	escaped := make([]string, 0, len(keys))
	for _, k := range keys {
		escaped = append(escaped, regexp.QuoteMeta(k))
	}
	// (?i) case-insensitive for the attribute name (HTML allows HREF=).
	// Group 1 = full attribute (href="..." or src="..."), group 2 = value.
	src := `(?i)(href|src)\s*=\s*"(` + strings.Join(escaped, "|") + `)"`
	return regexp.MustCompile(src)
}

func rewriteHTMLReferences(html string, pattern *regexp.Regexp, rewriteSet map[string]string) (string, int, int) {
	matches := pattern.FindAllStringSubmatchIndex(html, -1)
	if len(matches) == 0 {
		return html, 0, 0
	}

	var buf bytes.Buffer
	buf.Grow(len(html))

	last := 0
	found := 0
	replaced := 0

	for _, m := range matches {
		// m[0:2] full match, m[2:4] capture 1 (attr name), m[4:6] capture 2 (value)
		if len(m) < 6 {
			continue
		}
		valueStart, valueEnd := m[4], m[5]
		value := html[valueStart:valueEnd]

		// Preserve everything between the last match and this one.
		buf.WriteString(html[last:valueStart])

		// value starts with "/" (rewrite keys are stored as "/css/site.css").
		lookupKey := strings.TrimPrefix(value, "/")
		if target, ok := rewriteSet[lookupKey]; ok {
			buf.WriteString("/")
			buf.WriteString(target)
			replaced++
		} else {
			// Defensive: regex matched a key that is not in the set.
			// Should not happen because we built the pattern from the
			// set, but keep behavior safe.
			buf.WriteString(value)
		}
		found++

		last = valueEnd
	}
	buf.WriteString(html[last:])

	return buf.String(), found, replaced
}

// collectAssetRewriteEntries builds the logical → output map used by
// PostprocessHTML. Only assets whose on-disk path differs from the logical
// path (i.e. filename-level fingerprinting rewrote them) are returned.
func collectAssetRewriteEntries(assets []staticAssetFile, fingerprinter *assetFingerprinter) (map[string]string, error) {
	entries := make(map[string]string, len(assets))
	for _, asset := range assets {
		rewritten, err := fingerprinter.resolveFingerprintOutput(asset)
		if err != nil {
			return nil, err
		}
		rewrittenSlash := filepath.ToSlash(rewritten)
		logicalSlash := filepath.ToSlash(asset.OutputPath)
		if rewrittenSlash != logicalSlash {
			entries[logicalSlash] = rewrittenSlash
		}
	}
	return entries, nil
}

// VerifyAssetHashes walks the static asset manifest and confirms that each
// fingerprinted file's on-disk content hash matches the hash embedded in its
// filename. It is exposed for the `gobin check --assets` sub-mode.
//
// Returns the list of mismatches (empty on success) and the count verified.
func VerifyAssetHashes(outputDir string, fingerprinter *assetFingerprinter) ([]AssetHashMismatch, int, error) {
	manifest, err := readStaticAssetManifest(outputDir)
	if err != nil {
		return nil, 0, err
	}

	var mismatches []AssetHashMismatch
	verified := 0

	paths := make([]string, 0, len(manifest))
	for p := range manifest {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, manifestPath := range paths {
		if !fingerprinter.shouldFingerprintFilename(manifestPath) {
			continue
		}
		embedded, ok := extractEmbeddedHash(manifestPath)
		if !ok {
			continue
		}

		absPath := filepath.Join(outputDir, filepath.FromSlash(manifestPath))
		actual, err := hashFile(absPath)
		if err != nil {
			return mismatches, verified, fmt.Errorf("hash %s: %w", manifestPath, err)
		}
		verified++

		// assetFingerprinter uses the first 8 hex chars of sha1 (4 bytes
		// from hashFile). Compare prefix.
		if actual == "" || !strings.HasPrefix(actual, embedded) {
			mismatches = append(mismatches, AssetHashMismatch{
				OutputPath:      manifestPath,
				ExpectedHash:    embedded,
				ActualHash:      actual,
			})
		}
	}
	return mismatches, verified, nil
}

// AssetHashMismatch is one row of the verify-assets report.
type AssetHashMismatch struct {
	OutputPath   string
	ExpectedHash string
	ActualHash   string
}

func (m AssetHashMismatch) String() string {
	return fmt.Sprintf("%s: hash mismatch (embedded=%s, actual=%s)",
		m.OutputPath, m.ExpectedHash, m.ActualHash)
}

// extractEmbeddedHash parses the hash out of "name.<hash>.ext" where hash is
// 12 lowercase hex chars (sha256 truncated to 12 by hashBytes). Returns
// ("", false) for paths without an embedded hash.
func extractEmbeddedHash(manifestPath string) (string, bool) {
	manifestPath = filepath.ToSlash(manifestPath)
	dir, file := filepath.Split(manifestPath)
	_ = dir

	ext := filepath.Ext(file)
	if ext == "" {
		return "", false
	}
	stem := strings.TrimSuffix(file, ext)
	// stem = "name.<hash>"
	idx := strings.LastIndex(stem, ".")
	if idx < 0 {
		return "", false
	}
	candidate := stem[idx+1:]
	if len(candidate) != 12 {
		return "", false
	}
	for _, r := range candidate {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return "", false
		}
	}
	return candidate, true
}

// CategorizeAsset classifies an asset by extension. v1.5.0 uses this to
// default the filename-fingerprint extension set when the user does not
// override it.
func CategorizeAsset(ext string) assetCategory {
	switch strings.ToLower(ext) {
	case ".css":
		return categoryCSS
	case ".js", ".mjs":
		return categoryJS
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg":
		return categoryImage
	default:
		return categoryOther
	}
}
