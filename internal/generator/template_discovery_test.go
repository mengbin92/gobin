package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
)

func TestLoadTemplatesDiscoversCustomPartials(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singlePage" }}{{ template "customCard" . }}{{ end }}`)
	mustWriteFile(t, filepath.Join(tmpDir, "templates", "partials", "custom-card.html"), `{{ define "customCard" }}custom{{ end }}`)

	tmpl, err := loadTemplates(&config.Config{})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}
	if tmpl.Lookup("customCard") == nil {
		t.Fatal("expected custom partial to be loaded")
	}
}
