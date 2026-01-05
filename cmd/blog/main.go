package main

import (
	"fmt"
	"os"

	"github.com/mengbin92/blog/internal/config"
	"github.com/mengbin92/blog/internal/generator"
	"github.com/mengbin92/blog/internal/parser"
)

func main() {
	// 显示版本信息
	fmt.Println("Blog Static Site Generator v1.0.0")
	fmt.Println("===================================")

	// 加载配置
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 解析内容
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

	// 生成站点
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
}
