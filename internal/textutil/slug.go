package textutil

import "strings"

// Slug converts s to a URL-safe slug: lowercase ASCII letters / digits /
// hyphens, with CJK code blocks preserved. Spaces and underscores become
// hyphens, runs of hyphens collapse, and leading/trailing hyphens are
// trimmed. Returns "untitled" when the result would otherwise be empty.
func Slug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			(r >= 0x4e00 && r <= 0x9fff) ||
			(r >= 0x3400 && r <= 0x4dbf) ||
			(r >= 0x20000 && r <= 0x2a6df) {
			result.WriteRune(r)
		}
	}

	slug := result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "untitled"
	}

	return slug
}
