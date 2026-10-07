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

func TestLoadTemplatesSkipsUnparseableLayoutWithoutRegisteringName(t *testing.T) {
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

	mustWriteFile(t, filepath.Join(tmpDir, "templates", "_default", "single.html"), `{{ define "singlePage" }}single{{ end }}`)
	// Liquid syntax that Go templates cannot parse.
	mustWriteFile(t, filepath.Join(tmpDir, "_layouts", "post.html"), `{% if page.title %}{{ site.title | escape }}{% endif %}`)
	mustWriteFile(t, filepath.Join(tmpDir, "_layouts", "page.html"), `page layout body`)

	tmpl, err := loadTemplates(&config.Config{})
	if err != nil {
		t.Fatalf("loadTemplates failed: %v", err)
	}
	if tmpl.Lookup("post") != nil {
		t.Fatal("unparseable _layouts/post.html must not leave an incomplete template registered")
	}
	if tmpl.Lookup("page") == nil {
		t.Fatal("parseable _layouts/page.html should be registered")
	}
}
