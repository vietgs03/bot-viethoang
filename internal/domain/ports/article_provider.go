package ports

import (
	"context"

	"bot-viethoang/internal/domain/model"
)

// ArticleProvider fetches algorithm-related reading materials.
type ArticleProvider interface {
	GetRecommendedArticles(ctx context.Context, count int) ([]model.Article, error)
}
