package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/log"
	"github.com/mengbin92/gobin/internal/textutil"
	"github.com/spf13/cobra"
)

const (
	newKindPost = "post"
	newKindPage = "page"
)

var (
	newCmdDate  string
	newCmdForce bool
)

// NewCmd scaffolds new content (posts or pages) with a Front Matter stub.
var NewCmd = &cobra.Command{
	Use:   "new <post|page> <title>",
	Short: "Scaffold a new post or page",
	Long: `Create a new content file with a Front Matter stub.

For "post" the file lands at <contentDir>/YYYY-MM-DD-<slug>.md
For "page" the file lands at <pageDir>/<slug>.md

The slug is derived from the title (lowercased, hyphenated). Use --date
to override the date used in the post filename.`,
	Example: `  gobin new post "我的第一篇文章"
  gobin new post --date 2026-05-01 "Release notes"
  gobin new page "About"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNew(cmd.OutOrStdout(), newOptions{
			Kind:  args[0],
			Title: args[1],
			Date:  newCmdDate,
			Force: newCmdForce,
		})
	},
}

func init() {
	NewCmd.Flags().StringVar(&newCmdDate, "date", "", "Override the post date (format: YYYY-MM-DD). Defaults to today.")
	NewCmd.Flags().BoolVar(&newCmdForce, "force", false, "Overwrite the target file if it already exists")
}

type newOptions struct {
	Kind  string
	Title string
	Date  string
	Force bool
}

func runNew(stdout io.Writer, opts newOptions) error {
	log.Info("scaffolding new content", "kind", opts.Kind, "title", opts.Title)
	cfg, err := config.LoadDefault()
	if err != nil {
		log.Error("failed to load config", "error", err)
		return fmt.Errorf("load config: %w", err)
	}
	cfg = config.Normalize(cfg)

	return scaffoldContent(stdout, cfg, opts, time.Now)
}

func scaffoldContent(stdout io.Writer, cfg *config.Config, opts newOptions, nowFn func() time.Time) error {
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		return errors.New("title must not be empty")
	}

	slug := textutil.Slug(title)

	var destPath, content string
	switch opts.Kind {
	case newKindPost:
		date, err := resolveNewDate(opts.Date, nowFn)
		if err != nil {
			return err
		}
		destPath = filepath.Join(cfg.ContentDir, fmt.Sprintf("%s-%s.md", date.Format("2006-01-02"), slug))
		content = renderPostFrontMatter(title, date)
	case newKindPage:
		destPath = filepath.Join(cfg.PageDir, slug+".md")
		content = renderPageFrontMatter(title, nowFn())
	default:
		return fmt.Errorf("unknown content kind %q: expected %q or %q", opts.Kind, newKindPost, newKindPage)
	}

	if !opts.Force {
		if _, err := os.Stat(destPath); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", destPath)
		} else if !os.IsNotExist(err) {
			log.Error("failed to check destination", "path", destPath, "error", err)
			return fmt.Errorf("check destination: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		log.Error("failed to create destination directory", "path", destPath, "error", err)
		return fmt.Errorf("create destination directory: %w", err)
	}
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		log.Error("failed to write content file", "path", destPath, "error", err)
		return fmt.Errorf("write content file: %w", err)
	}

	fmt.Fprintf(stdout, "Created %s %s at %s\n", opts.Kind, slug, destPath)
	return nil
}

func resolveNewDate(raw string, nowFn func() time.Time) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nowFn(), nil
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --date %q: expected YYYY-MM-DD", raw)
	}
	return t, nil
}

func renderPostFrontMatter(title string, date time.Time) string {
	return fmt.Sprintf(`---
title: %q
date: %s
description: ""
tags: []
categories: []
draft: true
---

`, title, date.Format("2006-01-02T15:04:05-07:00"))
}

func renderPageFrontMatter(title string, now time.Time) string {
	return fmt.Sprintf(`---
title: %q
date: %s
description: ""
draft: true
---

`, title, now.Format("2006-01-02T15:04:05-07:00"))
}
