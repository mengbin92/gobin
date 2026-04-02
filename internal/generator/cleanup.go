package generator

import (
	"fmt"
	"os"
	"path/filepath"
)

func prepareOutputDir(outputDir string, clean bool) error {
	if clean {
		if err := cleanOutputDir(outputDir); err != nil {
			return err
		}
	}

	return os.MkdirAll(outputDir, 0755)
}

func cleanOutputDir(outputDir string) error {
	if outputDir == "" {
		return fmt.Errorf("output directory cannot be empty")
	}

	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	if absOutputDir == string(filepath.Separator) {
		return fmt.Errorf("refusing to clean filesystem root")
	}
	if absOutputDir == absCWD {
		return fmt.Errorf("refusing to clean working directory: %s", absOutputDir)
	}

	info, err := os.Stat(absOutputDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat output directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output path is not a directory: %s", absOutputDir)
	}

	entries, err := os.ReadDir(absOutputDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(absOutputDir, entry.Name())); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entry.Name(), err)
		}
	}

	return nil
}
