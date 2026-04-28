package generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

func safeOutputPath(outputDir, outputPath string) (string, error) {
	baseAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return "", err
	}

	cleanOutputPath := filepath.FromSlash(strings.TrimPrefix(outputPath, "/"))
	destAbs, err := filepath.Abs(filepath.Join(baseAbs, cleanOutputPath))
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(baseAbs, destAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("output path %q escapes output directory %q", outputPath, outputDir)
	}

	return destAbs, nil
}
