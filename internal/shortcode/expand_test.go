package shortcode

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/yuin/goldmark"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
)

// goldmarkConvert returns a convert func mirroring the parser's renderer.
// unsafe=false is the default policy (raw HTML in markdown is escaped), which
// is the important case for verifying shortcode HTML survives via sentinels.
func goldmarkConvert(unsafe bool) func(string) (string, error) {
	return func(md string) (string, error) {
		var opts []goldmark.Option
		if unsafe {
			opts = append(opts, goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()))
		}
		g := goldmark.New(opts...)
		var b strings.Builder
		if err := g.Convert([]byte(md), &b); err != nil {
			return "", err
		}
		return b.String(), nil
	}
}

// registryWithSite loads a registry rooted at a temp dir with the given
// templates/shortcodes files (name -> body), and chdirs into it.
func registryWithSite(t *testing.T, files map[string]string) *Registry {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		writeShortcode(t, filepath.Join(root, "templates", "shortcodes"), name, body)
	}
	chdir(t, root)
	reg, err := LoadRegistry(&config.Config{})
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	return reg
}

func TestRender_HTMLFormSurvivesEscaping(t *testing.T) {
	reg := builtinRegistry(t)
	src := "intro paragraph\n\n{{< youtube dQw4w9WgXcQ >}}\n\nclosing paragraph"

	out, err := reg.Render(src, goldmarkConvert(false))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, `https://www.youtube.com/embed/dQw4w9WgXcQ`) {
		t.Fatalf("youtube HTML not emitted (escaped?):\n%s", out)
	}
	// Block shortcode must not remain wrapped in a stray paragraph.
	if strings.Contains(out, "<p><div") {
		t.Errorf("block shortcode left inside a <p>:\n%s", out)
	}
	if strings.Contains(out, "gobinsc") {
		t.Errorf("sentinel token leaked into output:\n%s", out)
	}
}

func TestRender_InlineShortcodeStaysInline(t *testing.T) {
	reg := registryWithSite(t, map[string]string{"b.html": `<b>{{ .Get 0 }}</b>`})
	out, err := reg.Render(`text {{< b hi >}} more text`, goldmarkConvert(false))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "<p>text <b>hi</b> more text</p>") {
		t.Errorf("inline shortcode not substituted in place:\n%s", out)
	}
}

func TestRender_MarkdownFormRenderedAsMarkdown(t *testing.T) {
	reg := registryWithSite(t, map[string]string{"strong.html": `**{{ .Get 0 }}**`})
	out, err := reg.Render(`{{% strong loud %}}`, goldmarkConvert(false))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "<strong>loud</strong>") {
		t.Errorf("markdown-form output not rendered as markdown:\n%s", out)
	}
}

func TestRender_NestedShortcodes(t *testing.T) {
	reg := registryWithSite(t, map[string]string{
		"wrap.html": `<div class="wrap">{{ .Inner | safeHTML }}</div>`,
		"b.html":    `<b>{{ .Get 0 }}</b>`,
	})
	out, err := reg.Render(`{{< wrap >}}{{< b x >}}{{< /wrap >}}`, goldmarkConvert(false))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, `<div class="wrap"><b>x</b></div>`) {
		t.Errorf("nested shortcode not resolved inner-first:\n%s", out)
	}
}

func TestRender_UnknownShortcodeErrors(t *testing.T) {
	reg := builtinRegistry(t)
	if _, err := reg.Render(`{{< nope >}}`, goldmarkConvert(false)); err == nil {
		t.Fatal("expected error for unknown shortcode")
	}
}

func TestRender_DanglingClosingErrors(t *testing.T) {
	reg := builtinRegistry(t)
	if _, err := reg.Render(`{{< /highlight >}}`, goldmarkConvert(false)); err == nil {
		t.Fatal("expected error for closing tag with no opening")
	}
}

func TestRender_CodeFenceNotExpanded(t *testing.T) {
	reg := builtinRegistry(t)
	src := "```\n{{< youtube abc >}}\n```"
	out, err := reg.Render(src, goldmarkConvert(false))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "youtube.com/embed") {
		t.Errorf("shortcode inside code fence was expanded:\n%s", out)
	}
	if !strings.Contains(out, "youtube abc") {
		t.Errorf("literal shortcode text missing from code block:\n%s", out)
	}
}

func TestRender_MultipleSentinelsNoPrefixCollision(t *testing.T) {
	reg := registryWithSite(t, map[string]string{"b.html": `<b>{{ .Get 0 }}</b>`})
	// Force >=11 invocations so sequence numbers like 1 and 10 coexist.
	var sb strings.Builder
	for i := range 12 {
		sb.WriteString("para ")
		sb.WriteString(strings.Repeat("a", 1))
		sb.WriteString(" {{< b ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" >}}\n\n")
	}
	out, err := reg.Render(sb.String(), goldmarkConvert(false))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "gobinsc") {
		t.Errorf("sentinel leaked (prefix collision?):\n%s", out)
	}
	for i := range 12 {
		if !strings.Contains(out, "<b>"+strconv.Itoa(i)+"</b>") {
			t.Errorf("missing expansion for invocation %d:\n%s", i, out)
		}
	}
}
