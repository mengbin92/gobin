package parser

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGenerateSummaryTruncatesUTF8Safely(t *testing.T) {
	summary := generateSummary(strings.Repeat("界", 201))

	if !utf8.ValidString(summary) {
		t.Fatalf("expected valid UTF-8 summary, got %q", summary)
	}
	if !strings.HasSuffix(summary, "...") {
		t.Fatalf("expected truncated summary to end with ellipsis, got %q", summary)
	}
}
