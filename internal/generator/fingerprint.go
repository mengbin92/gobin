package generator

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mengbin92/gobin/internal/config"
)

// assetFingerprinter centralizes hash computation and fingerprint path
// rewriting for the static asset pipeline. It is built per build so callers
// (copy planner and asset URL resolver) share a hash cache and the same
// configured fingerprint policy.
//
// The resolver is exposed as a template function (assetURLResolver), so its
// memo caches are read and written from concurrent page-render workers during
// a parallel build. mu guards hashByPath and fingerprintPath; strategy and
// enabledExt are set once at construction and only read afterwards.
type assetFingerprinter struct {
	strategy        string
	enabledExt      map[string]struct{}
	mu              sync.Mutex
	hashByPath      map[string]string
	fingerprintPath map[string]string
}

func newAssetFingerprinter(cfg *config.Config) *assetFingerprinter {
	fp := &assetFingerprinter{
		strategy:        config.AssetsFingerprintStrategyQuery,
		hashByPath:      make(map[string]string),
		fingerprintPath: make(map[string]string),
	}

	if cfg == nil || cfg.Assets == nil || cfg.Assets.Fingerprint == nil {
		return fp
	}
	fp.strategy = cfg.Assets.Fingerprint.Strategy
	if fp.strategy != config.AssetsFingerprintStrategyFilename {
		return fp
	}

	extensions := cfg.Assets.Fingerprint.Extensions
	if len(extensions) == 0 {
		extensions = config.DefaultAssetsFingerprintExtensions()
	}
	fp.enabledExt = make(map[string]struct{}, len(extensions))
	for _, ext := range extensions {
		fp.enabledExt[normalizeExtension(ext)] = struct{}{}
	}
	return fp
}

// shouldFingerprintFilename reports whether the given logical output path is
// eligible for filename-level fingerprinting under the current strategy.
func (f *assetFingerprinter) shouldFingerprintFilename(outputPath string) bool {
	if f == nil || f.strategy != config.AssetsFingerprintStrategyFilename {
		return false
	}
	ext := normalizeExtension(filepath.Ext(outputPath))
	if ext == "" {
		return false
	}
	_, ok := f.enabledExt[ext]
	return ok
}

// hash returns the short content hash for sourcePath, caching the result by
// source path so repeated callers do not re-read the file.
func (f *assetFingerprinter) hash(sourcePath string) (string, error) {
	f.mu.Lock()
	cached, ok := f.hashByPath[sourcePath]
	f.mu.Unlock()
	if ok {
		return cached, nil
	}
	digest, err := hashFile(sourcePath)
	if err != nil {
		return "", err
	}
	f.mu.Lock()
	f.hashByPath[sourcePath] = digest
	f.mu.Unlock()
	return digest, nil
}

// resolveFingerprintOutput returns the on-disk output path for an asset,
// inserting a content hash into the filename when filename-level
// fingerprinting applies. When the asset is not eligible it returns the
// original output path unchanged.
func (f *assetFingerprinter) resolveFingerprintOutput(asset staticAssetFile) (string, error) {
	if !f.shouldFingerprintFilename(asset.OutputPath) {
		return asset.OutputPath, nil
	}
	f.mu.Lock()
	cached, ok := f.fingerprintPath[asset.OutputPath]
	f.mu.Unlock()
	if ok {
		return cached, nil
	}
	// f.hash locks internally; do not hold mu across this call.
	digest, err := f.hash(asset.SourcePath)
	if err != nil {
		return "", err
	}
	rewritten := insertFingerprint(asset.OutputPath, digest)
	f.mu.Lock()
	f.fingerprintPath[asset.OutputPath] = rewritten
	f.mu.Unlock()
	return rewritten, nil
}

func insertFingerprint(outputPath, digest string) string {
	dir, name := filepath.Split(outputPath)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", base, digest, ext))
}

func normalizeExtension(ext string) string {
	ext = strings.TrimSpace(ext)
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return strings.ToLower(ext)
}
