package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contains runtime configuration values.
type Config struct {
	DiscordWebhookURL  string
	DiscordBotToken    string
	ScheduleCron       string
	RandomProblemCount int
	ArticleCount       int
	RequestTimeout     time.Duration
	GeminiAPIKey       string
	GeminiModel        string
	GeminiTopicLimit   int
}

const (
	defaultCron             = "0 9 * * *" // 09:00 every day
	defaultRandomCount      = 2
	defaultArticleCount     = 2
	defaultTimeout          = 30 * time.Second
	defaultWebhookURL       = ""
	defaultBotToken         = ""
	defaultGeminiAPIKey     = ""
	defaultGeminiModel      = "gemini-2.5-flash"
	defaultGeminiTopicLimit = 3
)

// Load builds a Config from environment variables with sane defaults.
func Load() (*Config, error) {
	cfg := &Config{
		DiscordWebhookURL:  getenvDefault("DISCORD_WEBHOOK_URL", defaultWebhookURL),
		DiscordBotToken:    getenvDefault("DISCORD_BOT_TOKEN", defaultBotToken),
		ScheduleCron:       getenvDefault("SCHEDULE_CRON", defaultCron),
		RandomProblemCount: parseIntDefault("RANDOM_PROBLEM_COUNT", defaultRandomCount),
		ArticleCount:       parseIntDefault("ARTICLE_COUNT", defaultArticleCount),
		RequestTimeout:     parseDurationDefault("REQUEST_TIMEOUT", defaultTimeout),
		GeminiAPIKey:       getenvDefault("GEMINI_API_KEY", defaultGeminiAPIKey),
		GeminiModel:        getenvDefault("GEMINI_MODEL", defaultGeminiModel),
		GeminiTopicLimit:   parseIntDefault("GEMINI_TOPIC_LIMIT", defaultGeminiTopicLimit),
	}

	if cfg.DiscordWebhookURL == "" {
		return nil, fmt.Errorf("DISCORD_WEBHOOK_URL is required")
	}

	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = defaultTimeout
	}

	if cfg.GeminiTopicLimit <= 0 {
		cfg.GeminiTopicLimit = defaultGeminiTopicLimit
	}

	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func parseIntDefault(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}

func parseDurationDefault(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
