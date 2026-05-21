package generator

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
)

type assetURLResolver struct {
	sourceByOutput map[string]string
	assetByOutput  map[string]staticAssetFile
	basePath       string
	fingerprinter  *assetFingerprinter
}

func newAssetURLResolver(cfg *config.Config) (*assetURLResolver, error) {
	assets, err := collectStaticAssetFiles(cfg)
	if err != nil {
		return nil, err
	}

	sourceByOutput := make(map[string]string, len(assets))
	assetByOutput := make(map[string]staticAssetFile, len(assets))
	for _, asset := range assets {
		key := manifestAssetPath(asset.OutputPath)
		sourceByOutput[key] = asset.SourcePath
		assetByOutput[key] = asset
	}

	return &assetURLResolver{
		sourceByOutput: sourceByOutput,
		assetByOutput:  assetByOutput,
		basePath:       assetURLBasePath(cfg.BaseURL),
		fingerprinter:  newAssetFingerprinter(cfg),
	}, nil
}

func (r *assetURLResolver) URL(raw string) (string, error) {
	if r == nil {
		return raw, nil
	}

	assetURL := strings.TrimSpace(raw)
	if assetURL == "" {
		return raw, nil
	}
	if strings.HasPrefix(assetURL, "//") {
		return raw, nil
	}

	parsed, err := url.Parse(assetURL)
	if err != nil {
		return "", fmt.Errorf("parse asset URL %q: %w", raw, err)
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return raw, nil
	}

	cleanPath, ok := cleanManifestAssetPath(strings.TrimPrefix(parsed.Path, "/"))
	if !ok {
		return raw, nil
	}
	key := manifestAssetPath(cleanPath)
	asset, ok := r.assetByOutput[key]
	if !ok {
		return raw, nil
	}

	if r.fingerprinter.shouldFingerprintFilename(asset.OutputPath) {
		return r.filenameURL(parsed, asset)
	}
	return r.queryURL(parsed, asset)
}

func (r *assetURLResolver) filenameURL(parsed *url.URL, asset staticAssetFile) (string, error) {
	rewritten, err := r.fingerprinter.resolveFingerprintOutput(asset)
	if err != nil {
		return "", fmt.Errorf("fingerprint asset %s: %w", asset.OutputPath, err)
	}
	parsed.Path = joinURLPath(r.basePath, "/"+manifestAssetPath(rewritten))
	return parsed.String(), nil
}

func (r *assetURLResolver) queryURL(parsed *url.URL, asset staticAssetFile) (string, error) {
	version, err := r.fingerprinter.hash(asset.SourcePath)
	if err != nil {
		return "", fmt.Errorf("hash asset %s: %w", asset.OutputPath, err)
	}
	query := parsed.Query()
	query.Set("v", version)
	parsed.Path = joinURLPath(r.basePath, parsed.Path)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func assetURLBasePath(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return ""
	}
	return "/" + strings.Trim(parsed.Path, "/")
}

func joinURLPath(basePath, assetPath string) string {
	assetPath = "/" + strings.TrimLeft(assetPath, "/")
	if basePath == "" {
		return assetPath
	}
	return strings.TrimRight(basePath, "/") + assetPath
}
