package ports

import (
	"context"

	"bot-viethoang/internal/domain/model"
)

// ArticleWriter synthesizes narrative content from problems and articles.
type ArticleWriter interface {
	Compose(ctx context.Context, daily *model.Problem, random []model.Problem, articles []model.Article) (string, error)
}
