package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mengbin92/gobin/internal/config"
)

func fixedNow(t *testing.T, raw string) func() time.Time {
	t.Helper()
	when, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse fixedNow %q: %v", raw, err)
	}
	return func() time.Time { return when }
}

func TestScaffoldContent_Post_WritesDatedFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Normalize(&config.Config{
		ContentDir: filepath.Join(tmpDir, "_posts"),
		PageDir:    filepath.Join(tmpDir, "pages"),
	})

	var stdout bytes.Buffer
	now := fixedNow(t, "2026-05-21T09:30:00+08:00")
	if err := scaffoldContent(&stdout, cfg, newOptions{
		Kind:  "post",
		Title: "Hello World",
	}, now); err != nil {
		t.Fatalf("scaffoldContent failed: %v", err)
	}

	want := filepath.Join(cfg.ContentDir, "2026-05-21-hello-world.md")
	content, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected post at %s, got err=%v", want, err)
	}
	body := string(content)
	if !strings.Contains(body, `title: "Hello World"`) {
		t.Fatalf("expected title in front matter, got:\n%s", body)
	}
	if !strings.Contains(body, "date: 2026-05-21T09:30:00") {
		t.Fatalf("expected date in front matter, got:\n%s", body)
	}
	if !strings.Contains(body, "draft: true") {
		t.Fatalf("expected new posts to default to draft, got:\n%s", body)
	}
	if !strings.Contains(stdout.String(), "Created post hello-world") {
		t.Fatalf("expected success message, got:\n%s", stdout.String())
	}
}

func TestScaffoldContent_Post_RespectsDateFlag(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Normalize(&config.Config{
		ContentDir: filepath.Join(tmpDir, "_posts"),
		PageDir:    filepath.Join(tmpDir, "pages"),
	})

	now := fixedNow(t, "2026-05-21T09:30:00+08:00")
	if err := scaffoldContent(&bytes.Buffer{}, cfg, newOptions{
		Kind:  "post",
		Title: "Release Notes",
		Date:  "2026-01-15",
	}, now); err != nil {
		t.Fatalf("scaffoldContent failed: %v", err)
	}

	want := filepath.Join(cfg.ContentDir, "2026-01-15-release-notes.md")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected scaffold to honor --date, got err=%v", err)
	}
}

func TestScaffoldContent_Post_RejectsInvalidDate(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Normalize(&config.Config{
		ContentDir: filepath.Join(tmpDir, "_posts"),
		PageDir:    filepath.Join(tmpDir, "pages"),
	})
	now := fixedNow(t, "2026-05-21T09:30:00+08:00")

	err := scaffoldContent(&bytes.Buffer{}, cfg, newOptions{
		Kind:  "post",
		Title: "Broken",
		Date:  "May 1, 2026",
	}, now)
	if err == nil {
		t.Fatal("expected invalid date to error")
	}
	if !strings.Contains(err.Error(), "invalid --date") {
		t.Fatalf("expected invalid date error, got %v", err)
	}
}

func TestScaffoldContent_Page_WritesUndatedFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Normalize(&config.Config{
		ContentDir: filepath.Join(tmpDir, "_posts"),
		PageDir:    filepath.Join(tmpDir, "pages"),
	})

	now := fixedNow(t, "2026-05-21T09:30:00+08:00")
	if err := scaffoldContent(&bytes.Buffer{}, cfg, newOptions{
		Kind:  "page",
		Title: "About",
	}, now); err != nil {
		t.Fatalf("scaffoldContent failed: %v", err)
	}

	want := filepath.Join(cfg.PageDir, "about.md")
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected page at %s, got err=%v", want, err)
	}
	if !strings.Contains(string(body), `title: "About"`) {
		t.Fatalf("expected title in front matter, got:\n%s", string(body))
	}
}

func TestScaffoldContent_RefusesOverwriteWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Normalize(&config.Config{
		ContentDir: filepath.Join(tmpDir, "_posts"),
		PageDir:    filepath.Join(tmpDir, "pages"),
	})

	now := fixedNow(t, "2026-05-21T09:30:00+08:00")
	opts := newOptions{Kind: "post", Title: "Once"}
	if err := scaffoldContent(&bytes.Buffer{}, cfg, opts, now); err != nil {
		t.Fatalf("first scaffold failed: %v", err)
	}

	err := scaffoldContent(&bytes.Buffer{}, cfg, opts, now)
	if err == nil {
		t.Fatal("expected duplicate scaffold to error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}

	opts.Force = true
	if err := scaffoldContent(&bytes.Buffer{}, cfg, opts, now); err != nil {
		t.Fatalf("scaffold with --force failed: %v", err)
	}
}

func TestScaffoldContent_RejectsUnknownKind(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Normalize(&config.Config{ContentDir: tmpDir, PageDir: tmpDir})
	err := scaffoldContent(&bytes.Buffer{}, cfg, newOptions{Kind: "movie", Title: "x"}, fixedNow(t, "2026-05-21T09:30:00+08:00"))
	if err == nil || !strings.Contains(err.Error(), "unknown content kind") {
		t.Fatalf("expected unknown kind error, got %v", err)
	}
}

func TestScaffoldContent_RejectsEmptyTitle(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Normalize(&config.Config{ContentDir: tmpDir, PageDir: tmpDir})
	err := scaffoldContent(&bytes.Buffer{}, cfg, newOptions{Kind: "post", Title: "   "}, fixedNow(t, "2026-05-21T09:30:00+08:00"))
	if err == nil || !strings.Contains(err.Error(), "title must not be empty") {
		t.Fatalf("expected empty title error, got %v", err)
	}
}
