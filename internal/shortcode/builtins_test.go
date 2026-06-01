package shortcode

import (
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
)

// renderShortcode executes a single registered shortcode with the given args
// and inner body, returning its output. Test-only helper.
func renderShortcode(t *testing.T, reg *Registry, name string, a *args, inner string) string {
	t.Helper()
	tmpl, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("shortcode %q not registered", name)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, newContext(name, a, inner)); err != nil {
		t.Fatalf("execute %q: %v", name, err)
	}
	return buf.String()
}

func builtinRegistry(t *testing.T) *Registry {
	t.Helper()
	reg, err := LoadRegistry(&config.Config{})
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	return reg
}

func TestBuiltinFigure(t *testing.T) {
	reg := builtinRegistry(t)
	out := renderShortcode(t, reg, "figure", &args{named: map[string]string{
		"src":     "/img/a.png",
		"alt":     "an image",
		"caption": "a caption",
	}}, "")

	for _, want := range []string{`<figure>`, `<img src="/img/a.png"`, `alt="an image"`, `<figcaption>a caption</figcaption>`, `</figure>`} {
		if !strings.Contains(out, want) {
			t.Errorf("figure output missing %q\ngot: %s", want, out)
		}
	}
	if strings.Contains(out, "<a href") {
		t.Errorf("figure should not emit a link without link arg: %s", out)
	}
}

func TestBuiltinYoutubePositional(t *testing.T) {
	reg := builtinRegistry(t)
	out := renderShortcode(t, reg, "youtube", &args{positional: []string{"dQw4w9WgXcQ"}}, "")
	if !strings.Contains(out, "https://www.youtube.com/embed/dQw4w9WgXcQ") {
		t.Errorf("youtube embed missing id: %s", out)
	}
}

func TestBuiltinGistNamed(t *testing.T) {
	reg := builtinRegistry(t)
	out := renderShortcode(t, reg, "gist", &args{named: map[string]string{"user": "octocat", "id": "abc123"}}, "")
	if !strings.Contains(out, "https://gist.github.com/octocat/abc123.js") {
		t.Errorf("gist script missing: %s", out)
	}
}

func TestBuiltinHighlightEscapesInner(t *testing.T) {
	reg := builtinRegistry(t)
	out := renderShortcode(t, reg, "highlight", &args{positional: []string{"go"}}, `if a < b { return }`)
	if !strings.Contains(out, `class="language-go"`) {
		t.Errorf("highlight missing language class: %s", out)
	}
	// Inner is rendered as escaped text, so '<' must be escaped.
	if !strings.Contains(out, "a &lt; b") {
		t.Errorf("highlight inner should be HTML-escaped: %s", out)
	}
}
