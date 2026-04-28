package generator

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mengbin92/gobin/internal/parser"
)

func TestBuildSearchDocumentTruncatesUTF8ContentSafely(t *testing.T) {
	post := &parser.Post{
		Title:   "UTF-8",
		URL:     "/utf8/",
		Content: strings.Repeat("界", 2001),
	}

	doc := buildSearchDocument(post, nil, true)

	if !utf8.ValidString(doc.Content) {
		t.Fatalf("expected valid UTF-8 content, got %q", doc.Content)
	}
	if !strings.HasSuffix(doc.Content, "...") {
		t.Fatalf("expected truncated content to end with ellipsis, got %q", doc.Content)
	}
}
