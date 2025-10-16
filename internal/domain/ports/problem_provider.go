package ports

import (
	"context"

	"bot-viethoang/internal/domain/model"
)

// ProblemProvider defines access to LeetCode problems.
type ProblemProvider interface {
	GetDailyChallenge(ctx context.Context) (*model.Problem, error)
	GetRandomProblems(ctx context.Context, count int) ([]model.Problem, error)
}
