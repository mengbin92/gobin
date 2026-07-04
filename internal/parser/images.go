package parser

import (
	"regexp"
	"strings"
)

// ImageRef is one image reference extracted from a post or page. It is
// used by the v1.7 image-optimization pipeline to discover which source
// images need to be transformed into responsive variants.
//
// The ref string is the path as written in the source — a relative URL
// (no scheme, no host). Callers are responsible for resolving it to an
// absolute on-disk path. The Source field records where the reference
// came from so callers can produce useful log lines.
type ImageRef struct {
	// Ref is the raw path as it appeared in the source.
	Ref string
	// Source is the FilePath of the post or page that referenced the
	// image, so callers can produce a helpful warning if a transform
	// fails.
	Source string
	// Kind is "frontmatter" or "body" and helps with diagnostics.
	Kind string
}

// markdownImageRegexp matches Markdown image references. The alt text
// and the URL are captured separately. We do not match reference-style
// images ([ref]: url) because the value they reference is still a URL
// string the user wrote; the simpler inline form covers >95% of blog
// posts and keeps the regex easy to reason about.
var markdownImageRegexp = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

// frontMatterImageKeys is the set of front matter keys we treat as
// image references. The values can be a string ("/img/cover.jpg") or a
// list of strings (["/img/a.jpg", "/img/b.jpg"]). All are normalized to
// a single string slice.
var frontMatterImageKeys = []string{"cover", "image", "thumbnail", "hero"}

// ExtractPostImageRefs returns the deduplicated list of image references
// in a post (front matter + body). The body scan looks at the raw
// Markdown source, not the rendered HTML, so it picks up references that
// goldmark may have re-written (e.g. escaped characters in URLs).
func ExtractPostImageRefs(p *Post) []ImageRef {
	if p == nil {
		return nil
	}
	refs := make([]ImageRef, 0, 4)
	refs = extractFrontMatterImageRefs(p.Params, p.FilePath, refs)
	refs = extractBodyImageRefs(p.Content, p.FilePath, refs)
	return dedupImageRefs(refs)
}

// ExtractPageImageRefs is the page analogue of ExtractPostImageRefs.
func ExtractPageImageRefs(p *Page) []ImageRef {
	if p == nil {
		return nil
	}
	refs := make([]ImageRef, 0, 4)
	refs = extractFrontMatterImageRefs(p.Params, p.FilePath, refs)
	refs = extractBodyImageRefs(p.Content, p.FilePath, refs)
	return dedupImageRefs(refs)
}

func extractFrontMatterImageRefs(params map[string]interface{}, source string, out []ImageRef) []ImageRef {
	if len(params) == 0 {
		return out
	}
	for _, key := range frontMatterImageKeys {
		v, ok := params[key]
		if !ok {
			continue
		}
		for _, s := range flattenStringValues(v) {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, ImageRef{Ref: s, Source: source, Kind: "frontmatter"})
		}
	}
	return out
}

func extractBodyImageRefs(content, source string, out []ImageRef) []ImageRef {
	if content == "" {
		return out
	}
	for _, m := range markdownImageRegexp.FindAllStringSubmatch(content, -1) {
		ref := strings.TrimSpace(m[1])
		if ref == "" {
			continue
		}
		out = append(out, ImageRef{Ref: ref, Source: source, Kind: "body"})
	}
	return out
}

// flattenStringValues turns a front matter value into a slice of
// strings. Strings are returned as a one-element slice; lists of
// strings are returned as-is. Anything else (maps, numbers) is
// returned as an empty slice — the front matter shape for image
// references is either a string or a list of strings, and we silently
// ignore anything else rather than fail the build.
func flattenStringValues(v interface{}) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []string:
		return x
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// dedupImageRefs preserves the first occurrence of each ref (case
// sensitive, path sensitive — we do not normalize trailing slashes
// because the v1.7 pipeline is path-exact) and the Source/Kind metadata
// from that first occurrence.
func dedupImageRefs(refs []ImageRef) []ImageRef {
	if len(refs) == 0 {
		return refs
	}
	seen := make(map[string]struct{}, len(refs))
	out := make([]ImageRef, 0, len(refs))
	for _, r := range refs {
		if _, ok := seen[r.Ref]; ok {
			continue
		}
		seen[r.Ref] = struct{}{}
		out = append(out, r)
	}
	return out
}
