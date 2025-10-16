package ports

import "context"

// Logger is an abstract logger so the domain can remain decoupled from concrete loggers.
type Logger interface {
	Info(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
}
