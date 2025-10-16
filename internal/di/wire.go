//go:build wireinject

package di

import (
	"log/slog"
	"os"

	"github.com/google/wire"

	"bot-viethoang/internal/adapter/articles"
	"bot-viethoang/internal/adapter/discord"
	"bot-viethoang/internal/adapter/leetcode"
	"bot-viethoang/internal/adapter/logging"
	"bot-viethoang/internal/adapter/writing"
	"bot-viethoang/internal/app"
	"bot-viethoang/internal/config"
	"bot-viethoang/internal/domain/ports"
	"bot-viethoang/internal/usecase"
)

// InitializeApp wires the application components together.
func InitializeApp() (*app.App, error) {
	wire.Build(
		config.Load,
		provideSlogLogger,
		logging.New,
		wire.Bind(new(ports.Logger), new(*logging.SLogger)),
		provideProblemProvider,
		provideArticleProvider,
		provideArticleWriter,
		provideNotifier,
		usecase.NewDailyDigest,
		provideDigestConfig,
		app.New,
		provideSchedule,
	)
	return nil, nil
}

func provideSlogLogger() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}

func provideProblemProvider(cfg *config.Config, logger ports.Logger) ports.ProblemProvider {
	return leetcode.New(cfg.RequestTimeout, logger)
}

func provideArticleProvider(cfg *config.Config, logger ports.Logger) ports.ArticleProvider {
	medium := articles.NewMediumProvider(cfg.RequestTimeout, logger)
	devto := articles.NewDevToProvider(cfg.RequestTimeout, logger)
	return articles.NewCompositeProvider(logger, medium, devto)
}

func provideArticleWriter(cfg *config.Config, logger ports.Logger) ports.ArticleWriter {
	if cfg.GeminiAPIKey == "" {
		return nil
	}
	return writing.NewGeminiWriter(cfg.GeminiAPIKey, cfg.GeminiModel, cfg.RequestTimeout, logger)
}

func provideNotifier(cfg *config.Config, logger ports.Logger) ports.Notifier {
	return discord.NewWebhook(cfg.DiscordWebhookURL, cfg.RequestTimeout, logger)
}

func provideDigestConfig(cfg *config.Config) usecase.DailyDigestConfig {
	return usecase.DailyDigestConfig{
		RandomCount:  cfg.RandomProblemCount,
		ArticleCount: cfg.ArticleCount,
	}
}

func provideSchedule(cfg *config.Config) string {
	return cfg.ScheduleCron
}
