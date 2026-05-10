package generator

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type renderer interface {
	Lookup(name string) *template.Template
	ExecuteTemplate(wr io.Writer, name string, data interface{}) error
}

type PageSpec struct {
	TemplateCandidates []string
	OutputPath         string
	Title              string
	Data               interface{}
}

func renderPageSpecs(tmpl renderer, outputDir string, pages []PageSpec) error {
	for _, page := range pages {
		templateName, err := resolveTemplateName(tmpl, page.TemplateCandidates)
		if err != nil {
			return pageRenderError(page, "", err)
		}

		outputPath, err := safeOutputPath(outputDir, page.OutputPath)
		if err != nil {
			return pageRenderError(page, templateName, err)
		}
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return pageRenderError(page, templateName, err)
		}
		if err := renderTemplate(tmpl, templateName, outputPath, page.Data); err != nil {
			return pageRenderError(page, templateName, err)
		}
	}

	return nil
}

func resolveTemplateName(tmpl renderer, candidates []string) (string, error) {
	for _, candidate := range candidates {
		if tmpl.Lookup(candidate) != nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no template found for candidates: %s", strings.Join(candidates, ", "))
}

func pageRenderError(page PageSpec, templateName string, err error) error {
	parts := []string{"render page"}
	if page.OutputPath != "" {
		parts = append(parts, fmt.Sprintf("output=%q", page.OutputPath))
	}
	if page.Title != "" {
		parts = append(parts, fmt.Sprintf("title=%q", page.Title))
	}
	if templateName != "" {
		parts = append(parts, fmt.Sprintf("template=%q", templateName))
	} else if len(page.TemplateCandidates) > 0 {
		parts = append(parts, fmt.Sprintf("templates=%q", strings.Join(page.TemplateCandidates, ",")))
	}

	return fmt.Errorf("%s: %w", strings.Join(parts, " "), err)
}
