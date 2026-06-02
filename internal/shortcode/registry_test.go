package shortcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
)

func writeShortcode(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// chdir switches to dir for the duration of the test, since LoadRegistry
// resolves the site templates/shortcodes path relative to the working dir.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestLoadRegistryIncludesBuiltins(t *testing.T) {
	reg, err := LoadRegistry(&config.Config{})
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	for _, name := range []string{"figure", "youtube", "gist", "highlight"} {
		if _, ok := reg.Lookup(name); !ok {
			t.Errorf("built-in %q not registered", name)
		}
	}
}

func TestLoadRegistrySiteOverridesBuiltin(t *testing.T) {
	root := t.TempDir()
	writeShortcode(t, filepath.Join(root, "templates", "shortcodes"), "figure.html", `SITE-FIGURE`)
	chdir(t, root)

	reg, err := LoadRegistry(&config.Config{})
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	out := renderShortcode(t, reg, "figure", &args{}, "")
	if !strings.Contains(out, "SITE-FIGURE") {
		t.Errorf("site shortcode should override built-in, got: %s", out)
	}
}

func TestLoadRegistrySiteOverridesTheme(t *testing.T) {
	root := t.TempDir()
	writeShortcode(t, filepath.Join(root, "themes", "mytheme", "layouts", "shortcodes"), "note.html", `THEME-NOTE`)
	writeShortcode(t, filepath.Join(root, "templates", "shortcodes"), "note.html", `SITE-NOTE`)
	chdir(t, root)

	reg, err := LoadRegistry(&config.Config{Theme: "mytheme", ThemesDir: "themes"})
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	out := renderShortcode(t, reg, "note", &args{}, "")
	if !strings.Contains(out, "SITE-NOTE") {
		t.Errorf("site shortcode should override theme, got: %s", out)
	}
}

func TestLoadRegistryThemeOverridesBuiltin(t *testing.T) {
	root := t.TempDir()
	writeShortcode(t, filepath.Join(root, "themes", "mytheme", "layouts", "shortcodes"), "figure.html", `THEME-FIGURE`)
	chdir(t, root)

	reg, err := LoadRegistry(&config.Config{Theme: "mytheme", ThemesDir: "themes"})
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	out := renderShortcode(t, reg, "figure", &args{}, "")
	if !strings.Contains(out, "THEME-FIGURE") {
		t.Errorf("theme shortcode should override built-in, got: %s", out)
	}
}

func TestLoadRegistryBadTemplateErrors(t *testing.T) {
	root := t.TempDir()
	writeShortcode(t, filepath.Join(root, "templates", "shortcodes"), "broken.html", `{{ .Get `)
	chdir(t, root)

	if _, err := LoadRegistry(&config.Config{}); err == nil {
		t.Fatal("expected error for malformed shortcode template")
	}
}
