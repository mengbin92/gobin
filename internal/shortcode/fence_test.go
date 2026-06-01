package shortcode

import "testing"

// expandedNames scans src and returns the names of shortcodes that are NOT
// inside a protected (code) region — i.e. the ones that would expand.
func expandedNames(t *testing.T, src string) []string {
	t.Helper()
	tokens, err := scanTokens(src, protectedRanges(src))
	if err != nil {
		t.Fatalf("scanTokens: %v", err)
	}
	names := make([]string, 0, len(tokens))
	for _, tk := range tokens {
		names = append(names, tk.name)
	}
	return names
}

func TestProtectedRanges_FencedCodeBlock(t *testing.T) {
	src := "before {{< a >}}\n" +
		"```\n" +
		"{{< b >}}\n" +
		"```\n" +
		"after {{< c >}}\n"

	got := expandedNames(t, src)
	want := []string{"a", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestProtectedRanges_TildeFence(t *testing.T) {
	src := "~~~\n{{< inside >}}\n~~~\n{{< outside >}}\n"
	got := expandedNames(t, src)
	if len(got) != 1 || got[0] != "outside" {
		t.Fatalf("got %v, want [outside]", got)
	}
}

func TestProtectedRanges_InlineCode(t *testing.T) {
	src := "use `{{< notexpanded >}}` but {{< expanded >}} here"
	got := expandedNames(t, src)
	if len(got) != 1 || got[0] != "expanded" {
		t.Fatalf("got %v, want [expanded]", got)
	}
}

func TestProtectedRanges_UnterminatedFence(t *testing.T) {
	// An opening fence with no close protects everything to end of source.
	src := "intro {{< a >}}\n```\n{{< b >}}\nno close"
	got := expandedNames(t, src)
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v, want [a]", got)
	}
}
