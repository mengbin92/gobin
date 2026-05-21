package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
)

// assetFingerprinter centralizes hash computation and fingerprint path
// rewriting for the static asset pipeline. It is built per build so callers
// (copy planner and asset URL resolver) share a hash cache and the same
// configured fingerprint policy.
type assetFingerprinter struct {
	strategy        string
	enabledExt      map[string]struct{}
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
	if cached, ok := f.hashByPath[sourcePath]; ok {
		return cached, nil
	}
	digest, err := hashAssetFile(sourcePath)
	if err != nil {
		return "", err
	}
	f.hashByPath[sourcePath] = digest
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
	if cached, ok := f.fingerprintPath[asset.OutputPath]; ok {
		return cached, nil
	}
	digest, err := f.hash(asset.SourcePath)
	if err != nil {
		return "", err
	}
	rewritten := insertFingerprint(asset.OutputPath, digest)
	f.fingerprintPath[asset.OutputPath] = rewritten
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

func hashAssetFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil))[:12], nil
}
