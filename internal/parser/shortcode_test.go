package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/shortcode"
)

func writePost(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "2026-01-01-shortcode.md")
	content := "---\ntitle: \"Shortcode Post\"\ndate: 2026-01-01T10:00:00+08:00\n---\n\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write post: %v", err)
	}
	return path
}

func TestParsePost_ShortcodeExpanded(t *testing.T) {
	reg, err := shortcode.LoadRegistry(&config.Config{})
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	path := writePost(t, "intro\n\n{{< youtube dQw4w9WgXcQ >}}\n\noutro")
	opts := RenderOptions{Shortcodes: reg} // AllowUnsafeHTML stays false

	post, err := ParsePostWithOptions(path, opts)
	if err != nil {
		t.Fatalf("ParsePostWithOptions: %v", err)
	}

	if !strings.Contains(post.ContentHTML, "https://www.youtube.com/embed/dQw4w9WgXcQ") {
		t.Errorf("shortcode HTML not present in content (escaped?):\n%s", post.ContentHTML)
	}
}

func TestParsePost_NilRegistryLeavesShortcodeLiteral(t *testing.T) {
	path := writePost(t, "before {{< youtube abc >}} after")

	post, err := ParsePostWithOptions(path, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("ParsePostWithOptions: %v", err)
	}

	// With no registry, the parser must behave exactly as before: the shortcode
	// syntax is passed through to the rendered output unchanged.
	if !strings.Contains(post.ContentHTML, "{{&lt; youtube abc &gt;}}") &&
		!strings.Contains(post.ContentHTML, "{{< youtube abc >}}") {
		t.Errorf("nil registry should not expand shortcodes:\n%s", post.ContentHTML)
	}
	if strings.Contains(post.ContentHTML, "youtube.com/embed") {
		t.Errorf("nil registry expanded a shortcode:\n%s", post.ContentHTML)
	}
}
