package commands

import (
	"fmt"
	"os"

	"github.com/mengbin92/blog/internal/config"
	"github.com/mengbin92/blog/internal/generator"
	"github.com/mengbin92/blog/internal/parser"
	"github.com/spf13/cobra"
)

// Version is the current version of the application
const Version = "1.0.0"

// BuildCmd is the build command
var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the static site",
	Long: `Build the static site by parsing markdown posts and generating HTML pages.

This command will:
1. Read the configuration from config.yaml
2. Parse all markdown posts from the content directory
3. Generate HTML pages using templates
4. Copy static assets to the output directory`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Blog Static Site Generator v%s\n", Version)
		fmt.Println("===================================")

		cfg, err := config.Load("config.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		contentDir := cfg.ContentDir
		if contentDir == "" {
			contentDir = "_posts"
		}

		posts, err := parser.ParsePosts(contentDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing posts: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Found %d posts\n", len(posts))

		publishDir := cfg.PublishDir
		if publishDir == "" {
			publishDir = "public"
		}

		err = generator.Generate(posts, cfg, publishDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating site: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Site generated successfully in '%s' directory\n", publishDir)
	},
}
