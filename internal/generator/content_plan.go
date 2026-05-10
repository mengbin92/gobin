package generator

import (
	"sort"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/parser"
)

type contentPlan struct {
	posts           []*parser.Post
	standalonePages []*parser.Page
}

func prepareContentPlan(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, buildDrafts bool) contentPlan {
	visiblePosts := preparePosts(posts, cfg, buildDrafts)
	sortPostsByDateDesc(visiblePosts)

	return contentPlan{
		posts:           visiblePosts,
		standalonePages: standalonePages,
	}
}

func sortPostsByDateDesc(posts []*parser.Post) {
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
}
