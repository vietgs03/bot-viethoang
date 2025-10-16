package articles

import (
	"context"
	"strings"

	"bot-viethoang/internal/domain/model"
	"bot-viethoang/internal/domain/ports"
)

// CompositeProvider merges multiple article providers together.
type CompositeProvider struct {
	logger    ports.Logger
	providers []ports.ArticleProvider
}

// NewCompositeProvider constructs a provider that queries the given providers sequentially.
func NewCompositeProvider(logger ports.Logger, providers ...ports.ArticleProvider) *CompositeProvider {
	active := make([]ports.ArticleProvider, 0, len(providers))
	for _, p := range providers {
		if p != nil {
			active = append(active, p)
		}
	}
	return &CompositeProvider{
		logger:    logger,
		providers: active,
	}
}

// GetRecommendedArticles returns up to count articles, de-duplicated by link/title.
func (c *CompositeProvider) GetRecommendedArticles(ctx context.Context, count int) ([]model.Article, error) {
	if count <= 0 {
		return nil, nil
	}

	results := make([]model.Article, 0, count)
	seen := make(map[string]struct{})
	var firstErr error

	for _, provider := range c.providers {
		if len(results) >= count {
			break
		}

		items, err := provider.GetRecommendedArticles(ctx, count-len(results))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if c.logger != nil {
				c.logger.Error(ctx, "article provider failed", "error", err)
			}
			continue
		}

		for _, item := range items {
			key := canonicalArticleKey(item)
			if key == "" {
				continue
			}

			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			results = append(results, item)

			if len(results) >= count {
				break
			}
		}
	}

	if len(results) == 0 && firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

func canonicalArticleKey(article model.Article) string {
	if article.Link != "" {
		return strings.ToLower(strings.TrimSpace(article.Link))
	}
	if article.Title != "" {
		return strings.ToLower(strings.TrimSpace(article.Title))
	}
	return ""
}
