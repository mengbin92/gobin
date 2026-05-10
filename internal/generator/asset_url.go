package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
)

type assetURLResolver struct {
	sourceByOutput  map[string]string
	versionByOutput map[string]string
}

func newAssetURLResolver(cfg *config.Config) (*assetURLResolver, error) {
	assets, err := collectStaticAssetFiles(cfg)
	if err != nil {
		return nil, err
	}

	sourceByOutput := make(map[string]string, len(assets))
	for _, asset := range assets {
		sourceByOutput[manifestAssetPath(asset.OutputPath)] = asset.SourcePath
	}

	return &assetURLResolver{
		sourceByOutput:  sourceByOutput,
		versionByOutput: make(map[string]string, len(assets)),
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
	version, ok, err := r.version(cleanPath)
	if err != nil {
		return "", err
	}
	if !ok {
		return raw, nil
	}

	query := parsed.Query()
	query.Set("v", version)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (r *assetURLResolver) version(outputPath string) (string, bool, error) {
	key := manifestAssetPath(outputPath)
	if version, ok := r.versionByOutput[key]; ok {
		return version, true, nil
	}

	sourcePath, ok := r.sourceByOutput[key]
	if !ok {
		return "", false, nil
	}

	version, err := hashAssetFile(sourcePath)
	if err != nil {
		return "", false, fmt.Errorf("hash asset %s: %w", outputPath, err)
	}
	r.versionByOutput[key] = version
	return version, true, nil
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
