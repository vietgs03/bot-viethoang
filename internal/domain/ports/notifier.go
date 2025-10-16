package ports

import (
	"context"

	"bot-viethoang/internal/domain/model"
)

// Notifier sends notifications to downstream channels (e.g. Discord).
type Notifier interface {
	Send(ctx context.Context, notification model.Notification) error
}
