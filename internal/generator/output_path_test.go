package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

func TestRenderPageSpecsRejectsPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "public")
	tmpl := mustParseTemplate(t, `{{ define "page" }}ok{{ end }}`)

	err := renderPageSpecs(tmpl, outputDir, []PageSpec{
		{
			TemplateCandidates: []string{"page"},
			OutputPath:         "../escaped.html",
		},
	})
	if err == nil {
		t.Fatal("expected path traversal output path to fail")
	}
	if !strings.Contains(err.Error(), "escapes output directory") {
		t.Fatalf("expected output directory escape error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "escaped.html")); !os.IsNotExist(statErr) {
		t.Fatalf("expected escaped file to be absent, got err=%v", statErr)
	}
}

func TestGenerateAliasPagesRejectsPathTraversal(t *testing.T) {
	outputDir := t.TempDir()
	post := &parser.Post{
		Title:   "Unsafe Alias",
		URL:     "/posts/safe/",
		Aliases: []string{"../escaped/"},
	}

	err := generateAliasPages([]*parser.Post{post}, &config.Config{}, outputDir)
	if err == nil {
		t.Fatal("expected path traversal alias to fail")
	}
	if !strings.Contains(err.Error(), "escapes output directory") {
		t.Fatalf("expected output directory escape error, got %v", err)
	}
}
