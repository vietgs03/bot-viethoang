package app

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"

	"bot-viethoang/internal/domain/ports"
	"bot-viethoang/internal/usecase"
)

// App manages the lifecycle of the daily digest scheduler.
type App struct {
	cron     *cron.Cron
	usecase  *usecase.DailyDigest
	logger   ports.Logger
	schedule string
}

// New constructs an App instance.
func New(digest *usecase.DailyDigest, logger ports.Logger, schedule string) *App {
	return &App{
		cron:     cron.New(),
		usecase:  digest,
		logger:   logger,
		schedule: schedule,
	}
}

// Run executes the use case once immediately and then according to the cron schedule.
func (a *App) Run(ctx context.Context) error {
	if err := a.scheduleJob(); err != nil {
		return err
	}

	a.logger.Info(ctx, "running first digest immediately")
	if err := a.usecase.Run(ctx); err != nil {
		a.logger.Error(ctx, "initial digest run failed", "error", err)
	}

	a.logger.Info(ctx, "starting scheduler", "cron", a.schedule)
	a.cron.Start()

	<-ctx.Done()
	stopCtx := a.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-time.After(5 * time.Second):
	}
	a.logger.Info(context.Background(), "scheduler stopped")
	return nil
}

func (a *App) scheduleJob() error {
	_, err := a.cron.AddFunc(a.schedule, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := a.usecase.Run(ctx); err != nil {
			a.logger.Error(ctx, "scheduled digest run failed", "error", err)
		}
	})
	if err != nil {
		return err
	}
	return nil
}
