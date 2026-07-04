package parser

import (
	"reflect"
	"testing"
)

func TestExtractPostImageRefs_FrontMatter(t *testing.T) {
	p := &Post{
		FilePath: "/posts/foo.md",
		Params: map[string]interface{}{
			"cover":     "/img/cover.jpg",
			"image":     []string{"/img/a.png", "/img/b.png"},
			"thumbnail": "/img/thumb.jpg",
			"hero":      "/img/hero.jpg",
			"tags":      []string{"go", "web"}, // not an image key
		},
	}
	refs := ExtractPostImageRefs(p)
	want := []string{"/img/cover.jpg", "/img/a.png", "/img/b.png", "/img/thumb.jpg", "/img/hero.jpg"}
	if len(refs) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(refs), len(want), refs)
	}
	for i, w := range want {
		if refs[i].Ref != w {
			t.Errorf("[%d] ref=%q want %q", i, refs[i].Ref, w)
		}
		if refs[i].Kind != "frontmatter" {
			t.Errorf("[%d] kind=%q want frontmatter", i, refs[i].Kind)
		}
	}
}

func TestExtractPostImageRefs_Body(t *testing.T) {
	p := &Post{
		FilePath: "/posts/foo.md",
		Content: `Intro text.

![Cover](/img/cover.jpg)
![With title](/img/titled.png "Some title")
![](https://example.com/external.png)
![Empty]()
![](/img/just-slash.png)

End.`,
	}
	refs := ExtractPostImageRefs(p)
	// External URL is included by the regex (we don't filter scheme
	// here; the pipeline handles that). Empty alt with no URL is
	// dropped by the URL-trim check.
	want := []string{
		"/img/cover.jpg",
		"/img/titled.png",
		"https://example.com/external.png",
		"/img/just-slash.png",
	}
	if len(refs) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(refs), len(want), refs)
	}
	for i, w := range want {
		if refs[i].Ref != w {
			t.Errorf("[%d] ref=%q want %q", i, refs[i].Ref, w)
		}
		if refs[i].Kind != "body" {
			t.Errorf("[%d] kind=%q want body", i, refs[i].Kind)
		}
	}
}

func TestExtractPostImageRefs_Dedup(t *testing.T) {
	p := &Post{
		FilePath: "/posts/foo.md",
		Params: map[string]interface{}{
			"cover": "/img/cover.jpg",
		},
		Content: `![](/img/cover.jpg)
![](/img/cover.jpg)
![](/img/other.png)`,
	}
	refs := ExtractPostImageRefs(p)
	want := []string{"/img/cover.jpg", "/img/other.png"}
	if len(refs) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(refs), len(want), refs)
	}
	for i, w := range want {
		if refs[i].Ref != w {
			t.Errorf("[%d] ref=%q want %q", i, refs[i].Ref, w)
		}
	}
}

func TestExtractPostImageRefs_NilAndEmpty(t *testing.T) {
	if refs := ExtractPostImageRefs(nil); refs != nil {
		t.Errorf("nil post: got %v", refs)
	}
	p := &Post{}
	if refs := ExtractPostImageRefs(p); len(refs) != 0 {
		t.Errorf("empty post: got %v", refs)
	}
}

func TestExtractPageImageRefs(t *testing.T) {
	page := &Page{
		FilePath: "/pages/about.md",
		Params: map[string]interface{}{
			"cover": "/img/about.jpg",
		},
		Content: `![](/img/inline.png)`,
	}
	refs := ExtractPageImageRefs(page)
	want := []string{"/img/about.jpg", "/img/inline.png"}
	if len(refs) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(refs), len(want), refs)
	}
	if !reflect.DeepEqual(refs[0].Ref, want[0]) {
		t.Errorf("frontmatter ref=%q want %q", refs[0].Ref, want[0])
	}
}

func TestFlattenStringValues(t *testing.T) {
	cases := []struct {
		in  interface{}
		out []string
	}{
		{"hello", []string{"hello"}},
		{[]string{"a", "b"}, []string{"a", "b"}},
		{[]interface{}{"a", "b", 1, "c"}, []string{"a", "b", "c"}},
		{42, nil},
		{map[string]interface{}{"k": "v"}, nil},
	}
	for _, c := range cases {
		got := flattenStringValues(c.in)
		if !reflect.DeepEqual(got, c.out) {
			t.Errorf("flattenStringValues(%v) = %v, want %v", c.in, got, c.out)
		}
	}
}
