package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/imaging"
	"github.com/mengbin92/gobin/internal/parser"
)

// ImageStats summarizes the v1.7 image-optimization pipeline output. It
// is populated by runImagePipeline and surfaced in GenerationResult.Images
// so the CLI can report "transformed N source(s) into M variant(s)" the
// same way it reports pages / artifacts / static assets.
type ImageStats struct {
	// Sources is the count of distinct source images discovered across
	// posts and pages (after dedup).
	Sources int
	// Variants is the count of output files written to publishDir on
	// this run. It excludes sources that were skipped because their
	// on-disk variants were already up-to-date (see Skipped).
	Variants int
	// Skipped counts sources that were already up-to-date on disk:
	// the source content hash, the transform options hash, and every
	// expected variant file all matched the previous run. v1.7.1
	// addition; v1.7.0 passthrough-wrote every variant on every build.
	Skipped int
	// Errors counts per-source transform failures. The build does not
	// abort on a single image failure; it logs a warning and falls
	// back to the original bytes copied into the publishDir.
	Errors int
}

// runImagePipeline scans posts and pages for image references, transforms
// each distinct source into the configured responsive variants, and
// writes the variants into publishDir under a layout that mirrors the
// source path. It is a no-op when Assets.Images.Enabled is false.
//
// On success, runImagePipeline also writes a per-site manifest
// (publishDir/.gobin-images.json) that the postprocess HTML rewriter
// consults to turn <img src="/img/cover.jpg"> into
// <picture><source srcset=...><img srcset=...></picture>.
//
// The pipeline is intentionally conservative:
//
//   - A source that fails to decode or transform is logged and skipped
//     (counted in ImageStats.Errors), and the original source bytes are
//     copied into publishDir so the build still has a usable asset.
//   - Sources that resolve to absolute URLs (http://, https://, //cdn)
//     are skipped — they are not local files. The regex in
//     parser.ExtractPostImageRefs returns them as-is; we filter here.
//   - Sources that resolve outside the site root (path traversal
//     attempts) are skipped to keep the build sandbox safe.
//
// v1.7.1 incremental: the manifest records a source-content hash and
// a transform-options hash per entry. When both match the previous
// run AND every expected variant file is still on disk, the source
// is skipped (counted in ImageStats.Skipped). This makes subsequent
// builds much cheaper when posts/pages change but their referenced
// images do not.
func runImagePipeline(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, outputDir string) (ImageStats, error) {
	var stats ImageStats

	if cfg == nil || cfg.Assets == nil || cfg.Assets.Images == nil || !cfg.Assets.Images.Enabled {
		return stats, nil
	}
	imgCfg := cfg.Assets.Images

	// Discover sources across posts and pages, dedup by (resolved, path)
	// so the same image referenced from N pages is transformed once.
	refs := collectAllImageRefs(posts, standalonePages)

	// Load the previous manifest so we can skip sources whose on-disk
	// variants are still up-to-date. A missing or invalid manifest
	// disables the skip path (we still write a fresh one); the
	// build is never blocked on the manifest.
	prevManifest, _ := loadImageManifest(outputDir)
	optsHash := hashTransformOptions(imgCfg)

	exec := imaging.NewDefaultExecutor()
	manifest := make(imageManifest, len(refs))

	for _, ref := range refs {
		srcPath, err := resolveSourcePath(ref.Ref, cfg)
		if err != nil {
			stats.Errors++
			continue
		}
		stats.Sources++

		srcBytes, err := os.ReadFile(srcPath)
		if err != nil {
			stats.Errors++
			continue
		}
		srcHash := hashContent(srcBytes)

		// Incremental skip: if the previous run saw the same source
		// bytes and the same transform options, and every variant
		// file the previous run wrote is still on disk, reuse it.
		if prev, ok := prevManifest[ref.Ref]; ok && prev.SourceHash == srcHash && prev.OptionsHash == optsHash {
			if allVariantsOnDisk(ref.Ref, prev.Outputs, outputDir) {
				manifest[ref.Ref] = prev
				stats.Skipped++
				continue
			}
		}

		variants, err := imaging.Transform(srcBytes, filepath.Ext(srcPath), imaging.TransformOptions{
			Widths:  imgCfg.Srcset,
			Formats: imgCfg.Formats,
			Quality: imgCfg.Quality,
		}, exec)
		if err != nil {
			stats.Errors++
			if copyErr := copyOriginalToOutput(srcBytes, ref.Ref, outputDir); copyErr != nil {
				stats.Errors++
			}
			continue
		}

		entry := imageManifestEntry{
			Widths:      []int{},
			Formats:     []string{},
			Outputs:     map[string]map[string]string{},
			Sizes:       imgCfg.Sizes,
			SourceHash:  srcHash,
			OptionsHash: optsHash,
		}
		widthsSet := map[int]bool{}
		formatsSet := map[string]bool{}

		for _, v := range variants {
			outRel, err := variantOutputPath(ref.Ref, v, outputDir)
			if err != nil {
				stats.Errors++
				continue
			}
			if err := os.MkdirAll(filepath.Dir(outRel), 0755); err != nil {
				stats.Errors++
				continue
			}
			if err := os.WriteFile(outRel, v.Bytes, 0644); err != nil {
				stats.Errors++
				continue
			}
			stats.Variants++

			// Record the output path relative to the outputDir, in URL
			// form (forward slashes) so the postprocess rewriter can
			// match it without a path-style conversion.
			urlPath := "/" + filepath.ToSlash(strings.TrimPrefix(outRel, outputDir+string(filepath.Separator)))
			if !strings.HasPrefix(urlPath, "/") {
				urlPath = "/" + urlPath
			}
			widthKey := fmt.Sprintf("%dw", v.Width)
			if entry.Outputs[widthKey] == nil {
				entry.Outputs[widthKey] = map[string]string{}
			}
			entry.Outputs[widthKey][v.Format] = urlPath
			widthsSet[v.Width] = true
			formatsSet[v.Format] = true
		}

		for w := range widthsSet {
			entry.Widths = append(entry.Widths, w)
		}
		for f := range formatsSet {
			entry.Formats = append(entry.Formats, f)
		}
		// Stable order: widths ascending, formats alphabetical.
		sort.Ints(entry.Widths)
		sort.Strings(entry.Formats)
		manifest[ref.Ref] = entry
	}

	if err := writeImageManifest(outputDir, manifest); err != nil {
		// Manifest write failure is non-fatal: the build still has the
		// variant files; the HTML rewriter just won't be able to find
		// them. Surface as a warning stat instead of aborting.
		stats.Errors++
	}
	return stats, nil
}

// writeImageManifest serializes the per-site image manifest to
// publishDir/.gobin-images.json so the postprocess step can use it to
// rewrite <img src> attributes in rendered HTML.
func writeImageManifest(outputDir string, m imageManifest) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, imageManifestName), append(buf, "\n"...), 0644)
}

// loadImageManifest reads the on-disk image manifest so runImagePipeline
// can apply the v1.7.1 incremental skip. A missing or invalid manifest
// returns a nil map (and a nil error) so the caller can treat the build
// as a cold run without an explicit branch on the error.
//
// Exposed as a package-private helper because only runImagePipeline
// needs the full entry shape (SourceHash, OptionsHash). The
// postprocess-side helper loadImageManifestForPostprocess is a
// separate function with a stricter schema check.
func loadImageManifest(outputDir string) (imageManifest, error) {
	path := filepath.Join(outputDir, imageManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m imageManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// collectAllImageRefs aggregates image references from posts and pages
// and dedupes them by the ref path string.
func collectAllImageRefs(posts []*parser.Post, pages []*parser.Page) []parser.ImageRef {
	seen := make(map[string]struct{})
	var out []parser.ImageRef
	for _, p := range posts {
		for _, r := range parser.ExtractPostImageRefs(p) {
			if _, ok := seen[r.Ref]; ok {
				continue
			}
			seen[r.Ref] = struct{}{}
			out = append(out, r)
		}
	}
	for _, p := range pages {
		for _, r := range parser.ExtractPageImageRefs(p) {
			if _, ok := seen[r.Ref]; ok {
				continue
			}
			seen[r.Ref] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

// resolveSourcePath turns a ref string (e.g. "/img/cover.jpg" or
// "../assets/img/cover.jpg" relative to a post) into an absolute path on
// disk. It is conservative: absolute URLs are rejected, path-traversal
// attempts outside StaticDir are rejected, and unresolvable paths
// return an error so the caller can log and skip.
func resolveSourcePath(ref string, cfg *config.Config) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty image ref")
	}
	// Reject absolute URLs — we only transform local files.
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "//") {
		return "", fmt.Errorf("external url: %s", ref)
	}
	// Treat leading-slash refs as site-root paths under StaticDir.
	trimmed := strings.TrimPrefix(ref, "/")
	candidate := filepath.Join(cfg.StaticDir, trimmed)
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	// Reject path traversal: the resolved path must live under StaticDir.
	staticAbs, err := filepath.Abs(cfg.StaticDir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(staticAbs, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("ref resolves outside %s: %s", cfg.StaticDir, ref)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return abs, nil
}

// variantOutputPath composes the final output path for a single variant.
// Convention: <publishDir>/<ref-without-ext>-<width>w.<format>, with
// the leading slash preserved so the on-disk layout matches the URL
// layout (e.g. /img/cover.jpg -> public/img/cover-480w.webp).
func variantOutputPath(ref string, v imaging.Variant, outputDir string) (string, error) {
	ext := filepath.Ext(ref)
	base := strings.TrimSuffix(ref, ext)
	if !strings.HasPrefix(ref, "/") {
		// Relative refs (e.g. ../assets/img/cover.jpg) are rare in
		// Markdown and not currently wired up. Surface a clear error
		// so users know to use leading-slash refs.
		return "", fmt.Errorf("only /-rooted refs are supported, got %q", ref)
	}
	rel := base + "-" + v.OutputName[len("image-"):]
	return filepath.Join(outputDir, filepath.FromSlash(rel)), nil
}

// copyOriginalToOutput is the fallback when a per-source transform
// fails: copy the original bytes into publishDir so the build still
// has a usable asset.
func copyOriginalToOutput(src []byte, ref, outputDir string) error {
	trimmed := strings.TrimPrefix(ref, "/")
	out := filepath.Join(outputDir, filepath.FromSlash(trimmed))
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	return os.WriteFile(out, src, 0644)
}

// allVariantsOnDisk checks that every (width, format) -> URL entry
// from a previous manifest has a corresponding file on disk under
// outputDir. It is the second half of the incremental skip: even if
// the source hash and options hash match, a variant that the user
// has manually deleted should trigger a re-transform.
func allVariantsOnDisk(ref string, outputs map[string]map[string]string, outputDir string) bool {
	for _, byFormat := range outputs {
		for _, urlPath := range byFormat {
			diskPath, err := urlPathToDiskPath(urlPath, outputDir)
			if err != nil {
				return false
			}
			if _, err := os.Stat(diskPath); err != nil {
				return false
			}
		}
	}
	return true
}

// urlPathToDiskPath reverses the URL → disk conversion done in
// runImagePipeline: "/img/cover-480w.jpg" under outputDir becomes
// "<outputDir>/img/cover-480w.jpg" on the local filesystem.
func urlPathToDiskPath(urlPath, outputDir string) (string, error) {
	if !strings.HasPrefix(urlPath, "/") {
		return "", fmt.Errorf("manifest url not absolute: %s", urlPath)
	}
	rel := strings.TrimPrefix(urlPath, "/")
	return filepath.Join(outputDir, filepath.FromSlash(rel)), nil
}

// hashContent returns the lowercase hex SHA-256 of b. It is the
// "source bytes haven't changed" half of the incremental skip key.
func hashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hashTransformOptions returns a stable hash of the image pipeline's
// effective configuration. It is the "options haven't changed" half
// of the incremental skip key. Widths, Formats, Sizes, and Quality
// all participate; a user who tweaks any of them invalidates the
// skip and forces a re-transform.
func hashTransformOptions(imgCfg *config.AssetsImagesConfig) string {
	if imgCfg == nil {
		return ""
	}
	parts := []string{
		"srcset=" + joinInts(imgCfg.Srcset),
		"formats=" + strings.Join(imgCfg.Formats, ","),
		"sizes=" + imgCfg.Sizes,
		"quality=" + strconv.Itoa(imgCfg.Quality),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// joinInts renders []int as a stable, sorted "n,n,n" string. We sort
// here (instead of relying on the caller) so two configs that differ
// only in srcset order produce the same hash. The pipeline already
// de-duplicates and sorts widths at run time; this keeps the hash
// consistent with what Transform actually saw.
func joinInts(xs []int) string {
	cp := append([]int(nil), xs...)
	sort.Ints(cp)
	strs := make([]string, len(cp))
	for i, n := range cp {
		strs[i] = strconv.Itoa(n)
	}
	return strings.Join(strs, ",")
}

// imageManifestName is the on-disk filename of the v1.7 image manifest.
// It records which logical image paths have been transformed into
// responsive variants so the postprocess step can rewrite <img src>
// references in rendered HTML to <picture><source> blocks.
//
// Layout (per source):
//
//	{
//	  "/img/cover.jpg": {
//	    "widths": [480, 800, 1200],
//	    "formats": ["jpg", "png", "webp"],
//	    "outputs": {
//	      "480w": {"jpg": "/img/cover-480w.jpg", "png": "/img/cover-480w.png"},
//	      "800w": {"jpg": "/img/cover-800w.jpg", "png": "/img/cover-800w.png"},
//	      "1200w": {"jpg": "/img/cover-1200w.jpg", "png": "/img/cover-1200w.png"}
//	    },
//	    "sizes": "(max-width: 800px) 100vw, 800px",
//	    "source_hash": "<sha256>",
//	    "options_hash": "<sha256>"
//	  }
//	}
const imageManifestName = ".gobin-images.json"

// imageManifestEntry is one source image's variant set. Field tags
// match the on-disk JSON shape so callers can decode without a custom
// unmarshaler.
//
// SourceHash and OptionsHash are the v1.7.1 incremental-skip keys.
// Older manifests (written by v1.7.0, which did not record the
// hashes) decode cleanly with empty strings, and runImagePipeline
// treats a missing hash as "always re-transform" — a safe default
// that costs one extra build cycle when crossing from v1.7.0 to
// v1.7.1.
type imageManifestEntry struct {
	Widths      []int                        `json:"widths"`
	Formats     []string                     `json:"formats"`
	Outputs     map[string]map[string]string `json:"outputs"`
	Sizes       string                       `json:"sizes"`
	SourceHash  string                       `json:"source_hash,omitempty"`
	OptionsHash string                       `json:"options_hash,omitempty"`
}

// imageManifest is the top-level manifest document. Keys are logical
// image paths (the ref string as written in source).
type imageManifest map[string]imageManifestEntry

// loadImageManifestForPostprocess reads the on-disk image manifest
// produced by runImagePipeline and returns it in the shape
// PostprocessHTML expects (map[src]ImageSourceRewrite). It returns
// nil when the file is missing or invalid, so callers don't have to
// branch on "did the image artifact run?".
func loadImageManifestForPostprocess(outputDir string) map[string]ImageSourceRewrite {
	path := filepath.Join(outputDir, imageManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m imageManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	out := make(map[string]ImageSourceRewrite, len(m))
	for ref, entry := range m {
		out[ref] = ImageSourceRewrite{
			Widths:  entry.Widths,
			Formats: entry.Formats,
			Outputs: entry.Outputs,
			Sizes:   entry.Sizes,
		}
	}
	return out
}
