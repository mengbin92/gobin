package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/cmd/gobin/commands"
)

func TestRootCmd_DefaultsToBuild(t *testing.T) {
	tmpDir := t.TempDir()
	siteDir := filepath.Join(tmpDir, "site")

	if err := commands.InitializeSite(&bytes.Buffer{}, siteDir); err != nil {
		t.Fatalf("InitializeSite failed: %v", err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	defer os.Chdir(oldWd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{})
	if _, err := rootCmd.ExecuteC(); err != nil {
		t.Fatalf("rootCmd.ExecuteC failed: %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("Expected empty stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Site generated successfully in 'public' directory") {
		t.Fatalf("Expected default root command to build site, got %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(siteDir, "public", "index.html")); err != nil {
		t.Fatalf("Expected root command to generate public/index.html: %v", err)
	}
}
