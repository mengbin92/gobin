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
	Data               interface{}
}

func renderPageSpecs(tmpl renderer, outputDir string, pages []PageSpec) error {
	for _, page := range pages {
		templateName, err := resolveTemplateName(tmpl, page.TemplateCandidates)
		if err != nil {
			return err
		}

		outputPath := filepath.Join(outputDir, filepath.FromSlash(strings.TrimPrefix(page.OutputPath, "/")))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return err
		}
		if err := renderTemplate(tmpl, templateName, outputPath, page.Data); err != nil {
			return err
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
