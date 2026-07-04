package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// writePostFile writes a markdown post under dir with the given filename and
// front matter + body.
func writePostFile(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// postFixtures creates a content directory with several posts (including a
// nested subdir and mixed .md/.markdown extensions) and returns the dir path.
func postFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writePostFile(t, dir, "2026-01-01-alpha.md", `---
title: "Alpha"
date: 2026-01-01T10:00:00+08:00
draft: false
tags: ["go"]
---

Alpha body.`)
	writePostFile(t, dir, "2026-01-02-beta.markdown", `---
title: "Beta"
date: 2026-01-02T10:00:00+08:00
draft: true
---

Beta draft body.`)
	writePostFile(t, dir, "nested/2026-01-03-gamma.md", `---
title: "Gamma"
date: 2026-01-03T10:00:00+08:00
tags: ["go", "web"]
categories: ["tech"]
---

Gamma body.`)
	writePostFile(t, dir, "2026-01-04-delta.md", `---
title: "Delta"
date: 2026-01-04T10:00:00+08:00
---

Delta body.`)
	return dir
}

// pageFixtures creates a page directory with several standalone markdown pages.
func pageFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writePostFile(t, dir, "about.md", `---
title: "About"
permalink: "/about/"
---

About content.`)
	writePostFile(t, dir, "docs/nested.md", `---
title: "Nested"
description: "nested page"
---

Nested content.`)
	writePostFile(t, dir, "contact.markdown", `---
title: "Contact"
---

Contact content.`)
	return dir
}

func TestNormalizeParseConcurrency(t *testing.T) {
	autoExpected := min(runtime.NumCPU(), parseConcurrencyCap)
	cases := []struct {
		in   int
		want int
	}{
		{-1, autoExpected},
		{0, autoExpected},
		{1, 1},
		{2, 2},
		{64, 64}, // explicit values are not capped
	}
	for _, c := range cases {
		if got := normalizeParseConcurrency(c.in); got != c.want {
			t.Errorf("normalizeParseConcurrency(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestParsePostsWithOptionsConcurrent_MatchesSerial asserts the parallel path
// produces byte-for-byte identical output to the serial path. Posts are
// compared field-by-field to catch any ordering divergence.
func TestParsePostsWithOptionsConcurrent_MatchesSerial(t *testing.T) {
	dir := postFixtures(t)
	opts := DefaultRenderOptions()

	serial, err := ParsePostsWithOptionsConcurrent(dir, opts, 1)
	if err != nil {
		t.Fatalf("serial parse: %v", err)
	}
	parallel, err := ParsePostsWithOptionsConcurrent(dir, opts, 4)
	if err != nil {
		t.Fatalf("parallel parse: %v", err)
	}

	if len(serial) != len(parallel) {
		t.Fatalf("count mismatch: serial=%d parallel=%d", len(serial), len(parallel))
	}
	for i := range serial {
		if !reflect.DeepEqual(*serial[i], *parallel[i]) {
			t.Errorf("post %d mismatch:\nserial=%+v\nparallel=%+v", i, *serial[i], *parallel[i])
		}
	}
}

// TestParsePostsWithOptionsConcurrent_PreservesOrder asserts results are
// returned in filepath.WalkDir's lexical order (which is what serial parsing
// yields and what downstream code expects).
func TestParsePostsWithOptionsConcurrent_PreservesOrder(t *testing.T) {
	dir := postFixtures(t)
	opts := DefaultRenderOptions()

	posts, err := ParsePostsWithOptionsConcurrent(dir, opts, 4)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(posts) != 4 {
		t.Fatalf("expected 4 posts, got %d", len(posts))
	}
	// filepath.WalkDir yields lexical order: 2026-01-01-alpha,
	// 2026-01-02-beta, 2026-01-04-delta, nested/2026-01-03-gamma.
	want := []string{"Alpha", "Beta", "Delta", "Gamma"}
	for i, p := range posts {
		if p.Title != want[i] {
			t.Errorf("post %d: got title %q, want %q", i, p.Title, want[i])
		}
	}
}

// TestParsePostsWithOptionsConcurrent_PropagatesError asserts a parse error in
// one file aborts the whole run and the error mentions the offending path.
func TestParsePostsWithOptionsConcurrent_PropagatesError(t *testing.T) {
	dir := t.TempDir()
	writePostFile(t, dir, "2026-01-01-good.md", `---
title: "Good"
date: 2026-01-01T10:00:00+08:00
---

Good body.`)
	// Missing closing front matter delimiter.
	writePostFile(t, dir, "2026-01-02-bad.md", `---
title: "Bad"
date: 2026-01-02T10:00:00+08:00

No closing delimiter.`)

	_, err := ParsePostsWithOptionsConcurrent(dir, DefaultRenderOptions(), 4)
	if err == nil {
		t.Fatal("expected error for malformed front matter, got nil")
	}
	if !strings.Contains(err.Error(), "2026-01-02-bad.md") {
		t.Fatalf("expected error to mention the bad file, got: %v", err)
	}
}

// TestParsePostsWithOptionsConcurrent_EmptyAndSingle covers the fallback paths.
func TestParsePostsWithOptionsConcurrent_EmptyAndSingle(t *testing.T) {
	// Empty dir.
	empty := t.TempDir()
	posts, err := ParsePostsWithOptionsConcurrent(empty, DefaultRenderOptions(), 4)
	if err != nil {
		t.Fatalf("empty dir: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("expected 0 posts, got %d", len(posts))
	}

	// Single file.
	single := t.TempDir()
	writePostFile(t, single, "2026-01-01-solo.md", `---
title: "Solo"
date: 2026-01-01T10:00:00+08:00
---

Solo body.`)
	posts, err = ParsePostsWithOptionsConcurrent(single, DefaultRenderOptions(), 4)
	if err != nil {
		t.Fatalf("single file: %v", err)
	}
	if len(posts) != 1 || posts[0].Title != "Solo" {
		t.Fatalf("expected single Solo post, got %+v", posts)
	}
}

// TestParsePagesWithOptionsConcurrent_MatchesSerial mirrors the post
// equivalence test for standalone pages.
func TestParsePagesWithOptionsConcurrent_MatchesSerial(t *testing.T) {
	dir := pageFixtures(t)
	opts := RenderOptions{AllowUnsafeHTML: true}

	serial, err := ParsePagesWithOptionsConcurrent(dir, opts, 1)
	if err != nil {
		t.Fatalf("serial parse: %v", err)
	}
	parallel, err := ParsePagesWithOptionsConcurrent(dir, opts, 4)
	if err != nil {
		t.Fatalf("parallel parse: %v", err)
	}

	if len(serial) != len(parallel) {
		t.Fatalf("count mismatch: serial=%d parallel=%d", len(serial), len(parallel))
	}
	for i := range serial {
		if !reflect.DeepEqual(*serial[i], *parallel[i]) {
			t.Errorf("page %d mismatch:\nserial=%+v\nparallel=%+v", i, *serial[i], *parallel[i])
		}
	}
}

// TestParsePagesWithOptionsConcurrent_PropagatesError asserts page parse
// errors abort the run.
func TestParsePagesWithOptionsConcurrent_PropagatesError(t *testing.T) {
	dir := t.TempDir()
	writePostFile(t, dir, "good.md", `---
title: "Good"
---

Good.`)
	writePostFile(t, dir, "bad.md", `---
title: "Bad"

No closing delimiter.`)

	_, err := ParsePagesWithOptionsConcurrent(dir, DefaultRenderOptions(), 4)
	if err == nil {
		t.Fatal("expected error for malformed front matter, got nil")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Fatalf("expected error to mention the bad file, got: %v", err)
	}
}
