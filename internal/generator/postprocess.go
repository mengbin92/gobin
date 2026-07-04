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
	// v1.7 image pipeline integration.
	ImageReferencesFound     int
	ImageReferencesRewritten int
}

// PostprocessOptions controls a single postprocess pass.
type PostprocessOptions struct {
	OutputDir string
	// LogicalToOutput maps logical asset paths (no leading slash, e.g.
	// "css/site.css") to their on-disk output paths (e.g.
	// "css/site.abc123.css"). Entries where the two values are equal are
	// skipped. Only filename-level fingerprinting produces entries.
	LogicalToOutput map[string]string
	// ImageSources, when non-empty, enables the v1.7 <img> -> <picture>
	// rewrite. The map is keyed by the original <img src> path (with a
	// leading slash) and carries the per-source variant set produced by
	// the image pipeline. The rewriter emits a <picture> element whose
	// <source> tags cover each requested format and whose <img> fallback
	// keeps the same src as the original.
	ImageSources map[string]ImageSourceRewrite
}

// ImageSourceRewrite is the per-source data the v1.7 postprocess step
// needs to emit a <picture> block. Widths is the list of source widths
// in ascending order; for each width there is one output path per
// format. Sizes is the `sizes` attribute that goes on the <img> element
// and on every <source>.
type ImageSourceRewrite struct {
	// Widths sorted ascending.
	Widths []int
	// Formats sorted alphabetically.
	Formats []string
	// Outputs[width][format] = URL path (e.g. "/img/cover-800w.jpg").
	Outputs map[string]map[string]string
	// Sizes is the sizes attribute value (e.g.
	// "(max-width: 800px) 100vw, 800px"). Empty is allowed and the
	// rewriter omits the attribute.
	Sizes string
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
	if len(rewriteSet) == 0 && len(opts.ImageSources) == 0 {
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
	// pattern == nil is fine when ImageSources alone are present; the
	// asset-rewrite block is skipped and the image-rewrite block runs
	// in the WalkDir below.
	_ = pattern

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

		var rewritten string
		var assetFound, assetReplaced int
		if pattern != nil {
			rewritten, assetFound, assetReplaced = rewriteHTMLReferences(string(content), pattern, rewriteSet)
			stats.ReferencesFound += assetFound
			stats.ReferencesRewritten += assetReplaced
		} else {
			rewritten = string(content)
		}

		// v1.7: also rewrite <img src="/img/cover.jpg"> to <picture>
		// when the image pipeline has produced variants for that path.
		var imgFound, imgReplaced int
		if len(opts.ImageSources) > 0 {
			rewritten, imgFound, imgReplaced = rewriteImageReferences(rewritten, opts.ImageSources)
			stats.ImageReferencesFound += imgFound
			stats.ImageReferencesRewritten += imgReplaced
		}
		found := assetFound + imgFound
		replaced := assetReplaced + imgReplaced

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
			"asset_rewrites", found,
			"image_rewrites", replaced,
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
				OutputPath:   manifestPath,
				ExpectedHash: embedded,
				ActualHash:   actual,
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

// rewriteImageReferences turns every <img src="..."> in html whose src
// matches a key in imageSources into a <picture> block with one
// <source type="image/<format>"> per format and a <img srcset> fallback.
//
// The rewriter is deliberately narrow: it only touches <img> elements
// whose src is one of the keys in imageSources. <a href>, <link
// rel="alternate">, external URLs, and unlisted <img> tags are left
// alone so the existing v1.5 asset-rewrite semantics (and its explicit
// "no-touch" list) carry over.
//
// Output shape (one per source width, all formats emitted):
//
//	<picture>
//	  <source type="image/avif" srcset="/img/cover-480w.avif 480w, /img/cover-800w.avif 800w" sizes="...">
//	  <source type="image/webp" srcset="/img/cover-480w.webp 480w, /img/cover-800w.webp 800w" sizes="...">
//	  <img src="/img/cover-480w.jpg" srcset="/img/cover-480w.jpg 480w, /img/cover-800w.jpg 800w" sizes="..." loading="lazy" decoding="async">
//	</picture>
//
// When the image has only one format, the rewriter elides the
// <source> tags and just emits a plain <img srcset> tag (no <picture>
// wrapper). This keeps the output lean for the common
// jpg-only-rewrites case.
func rewriteImageReferences(html string, imageSources map[string]ImageSourceRewrite) (string, int, int) {
	if len(imageSources) == 0 {
		return html, 0, 0
	}
	// Build a regex of all known <img src="..."> tags. The src value is
	// group 1. We rely on the caller to have already filtered to
	// <img>, so we only match that tag.
	keys := make([]string, 0, len(imageSources))
	for k := range imageSources {
		keys = append(keys, k)
	}
	// Longest first so e.g. /img/cover.png matches before /img/cover.
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	escaped := make([]string, len(keys))
	for i, k := range keys {
		escaped[i] = regexp.QuoteMeta(k)
	}
	pattern := regexp.MustCompile(`(?is)<img\s+[^>]*?\bsrc\s*=\s*"(` + strings.Join(escaped, "|") + `)"[^>]*?>`)

	matches := pattern.FindAllStringSubmatchIndex(html, -1)
	if len(matches) == 0 {
		return html, 0, 0
	}

	var out strings.Builder
	out.Grow(len(html) + 128*len(matches))
	last := 0
	found := 0
	replaced := 0

	for _, m := range matches {
		srcStart, srcEnd := m[2], m[3]
		tagStart, tagEnd := m[0], m[1]
		originalTag := html[tagStart:tagEnd]
		src := html[srcStart:srcEnd]
		entry := imageSources[src]
		if len(entry.Outputs) == 0 {
			continue
		}
		found++

		// Build the new tag. The fallback <img> tag reuses the
		// original tag's other attributes (alt, class, ...) by
		// stripping the src="..." chunk and appending srcset + sizes.
		fallback := buildPictureFallback(originalTag, src, entry)
		if fallback == "" {
			continue
		}

		out.WriteString(html[last:tagStart])
		out.WriteString(fallback)
		replaced++
		last = tagEnd
	}
	out.WriteString(html[last:])
	return out.String(), found, replaced
}

// buildPictureFallback renders a single image element from a v1.7
// rewrite. When there is only one format the rewriter emits a single
// <img> tag (no <picture> wrapper); when there are multiple, it emits
// a <picture> with one <source> per format and an <img> fallback.
//
// The "src" of the fallback <img> is the smallest width's <format>
// variant. That keeps non-source-aware browsers (legacy browsers that
// ignore <source>) on the smallest, fastest variant, which is the
// correct progressive-enhancement default.
func buildPictureFallback(originalTag, src string, entry ImageSourceRewrite) string {
	widths := entry.Widths
	if len(widths) == 0 {
		return ""
	}
	formats := entry.Formats
	if len(formats) == 0 {
		return ""
	}
	// Pick the fallback format as the first format (stable order
	// because we sort formats alphabetically upstream) and the
	// fallback width as the largest one (so the fallback has the
	// best quality if the browser ignores srcset).
	fallbackFormat := formats[0]
	fallbackWidth := widths[len(widths)-1]
	fallbackSrc, ok := entry.Outputs[fmt.Sprintf("%dw", fallbackWidth)][fallbackFormat]
	if !ok {
		return ""
	}

	srcset := buildSrcset(entry, fallbackFormat)
	sizesAttr := ""
	if entry.Sizes != "" {
		sizesAttr = ` sizes="` + entry.Sizes + `"`
	}

	// Strip src="..." from the original tag so we can reuse its
	// other attributes (alt, class, id, loading, decoding, ...).
	stripped := stripSrcAttr(originalTag, src)

	if len(formats) == 1 {
		// Single-format fast path: <img src="<smallest variant>"
		// srcset="..." sizes="..." loading="lazy" decoding="async">.
		// The src points at the smallest variant so non-srcset-aware
		// browsers stay on the cheapest download.
		var single strings.Builder
		single.WriteString(stripped)
		single.WriteString(` src="`)
		single.WriteString(fallbackSrc)
		single.WriteString(`"`)
		single.WriteString(` srcset="`)
		single.WriteString(srcset)
		single.WriteString(`"`)
		if sizesAttr != "" {
			single.WriteString(sizesAttr)
		}
		single.WriteString(` loading="lazy" decoding="async">`)
		return single.String()
	}

	// Multi-format path: build a <picture> with one <source> per
	// format and an <img> fallback. The <source> tags are ordered
	// by format alphabetically (matches entry.Formats).
	var picture strings.Builder
	picture.WriteString("<picture>")
	for _, f := range formats {
		// Build a per-format srcset.
		psrcset := buildSrcset(entry, f)
		if psrcset == "" {
			continue
		}
		picture.WriteString(`<source type="image/`)
		picture.WriteString(f)
		picture.WriteString(`" srcset="`)
		picture.WriteString(psrcset)
		picture.WriteString(`"`)
		if sizesAttr != "" {
			picture.WriteString(sizesAttr)
		}
		picture.WriteString(`>`)
	}
	// Fallback <img>: keep the original src (smallest variant of
	// the first format) so the element is well-formed.
	picture.WriteString(stripped)
	picture.WriteString(` src="`)
	picture.WriteString(fallbackSrc)
	picture.WriteString(`"`)
	picture.WriteString(` srcset="`)
	picture.WriteString(srcset)
	picture.WriteString(`"`)
	if sizesAttr != "" {
		picture.WriteString(sizesAttr)
	}
	picture.WriteString(` loading="lazy" decoding="async">`)
	picture.WriteString("</picture>")
	return picture.String()
}

// buildSrcset renders a `srcset` attribute value for a given format.
// Format: "<url> <width>w, <url> <width>w, ..." (width descriptor).
func buildSrcset(entry ImageSourceRewrite, format string) string {
	parts := make([]string, 0, len(entry.Widths))
	for _, w := range entry.Widths {
		key := fmt.Sprintf("%dw", w)
		outs, ok := entry.Outputs[key]
		if !ok {
			continue
		}
		path, ok := outs[format]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf(`%s %dw`, path, w))
	}
	return strings.Join(parts, ", ")
}

// stripSrcAttr removes the first src="..." attribute (with the value
// passed in) from an <img> tag, returning the rest of the tag's
// attributes without the trailing `>`. Callers are responsible for
// closing the tag (`>`) once they have appended the rewritten
// attributes. The value is matched exactly so the rewriter does not
// accidentally strip a sibling attribute that happens to contain the
// same URL.
func stripSrcAttr(tag, src string) string {
	needle := `src="` + src + `"`
	idx := strings.Index(tag, needle)
	if idx < 0 {
		// Already stripped or not present. Strip the trailing > so the
		// returned fragment is uniformly a list of attributes.
		return strings.TrimRight(tag, ">")
	}
	before := tag[:idx]
	after := tag[idx+len(needle):]
	// after may start with `>` (when src was the last attribute) or
	// with ` ...>` (when there were trailing attributes). Trim both.
	after = strings.TrimLeft(after, " ")
	after = strings.TrimPrefix(after, ">")
	return strings.TrimRight(before, " ") + " " + after
}
