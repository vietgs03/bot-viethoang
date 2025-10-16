package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bot-viethoang/internal/domain/model"
	"bot-viethoang/internal/domain/ports"
)

// DailyDigest orchestrates fetching problems and articles, then sending a notification.
type DailyDigest struct {
	problems     ports.ProblemProvider
	articles     ports.ArticleProvider
	writer       ports.ArticleWriter
	notifier     ports.Notifier
	logger       ports.Logger
	randomCount  int
	articleCount int
}

// DailyDigestConfig controls optional behaviours for the digest.
type DailyDigestConfig struct {
	RandomCount  int
	ArticleCount int
}

// NewDailyDigest constructs a DailyDigest use case.
func NewDailyDigest(
	problems ports.ProblemProvider,
	articles ports.ArticleProvider,
	writer ports.ArticleWriter,
	notifier ports.Notifier,
	logger ports.Logger,
	cfg DailyDigestConfig,
) *DailyDigest {
	return &DailyDigest{
		problems:     problems,
		articles:     articles,
		writer:       writer,
		notifier:     notifier,
		logger:       logger,
		randomCount:  cfg.RandomCount,
		articleCount: cfg.ArticleCount,
	}
}

// Run executes the daily digest workflow.
func (d *DailyDigest) Run(ctx context.Context) error {
	start := time.Now()
	d.logger.Info(ctx, "starting daily digest")

	daily, err := d.problems.GetDailyChallenge(ctx)
	if err != nil {
		d.logger.Error(ctx, "failed to fetch daily challenge", "error", err)
		return err
	}

	randomProblems := d.fetchRandomProblems(ctx, daily)
	articles := d.fetchArticles(ctx)
	insight := d.composeInsight(ctx, daily, randomProblems, articles)

	notification := d.buildNotification(daily, randomProblems, articles, insight)
	if err := d.notifier.Send(ctx, notification); err != nil {
		d.logger.Error(ctx, "failed to send notification", "error", err)
		return err
	}

	d.logger.Info(ctx, "daily digest completed", "duration", time.Since(start))
	return nil
}

func (d *DailyDigest) fetchRandomProblems(ctx context.Context, daily *model.Problem) []model.Problem {
	if d.randomCount <= 0 {
		return nil
	}

	problems, err := d.problems.GetRandomProblems(ctx, d.randomCount)
	if err != nil {
		d.logger.Error(ctx, "failed to fetch random problems", "error", err)
		return nil
	}

	dailySlug := ""
	if daily != nil {
		dailySlug = daily.Slug
	}

	unique := make([]model.Problem, 0, len(problems))
	seen := map[string]struct{}{}
	if dailySlug != "" {
		seen[dailySlug] = struct{}{}
	}

	for _, p := range problems {
		if _, exists := seen[p.Slug]; exists {
			continue
		}
		seen[p.Slug] = struct{}{}
		unique = append(unique, p)
	}

	return unique
}

func (d *DailyDigest) fetchArticles(ctx context.Context) []model.Article {
	if d.articles == nil || d.articleCount <= 0 {
		return nil
	}

	articles, err := d.articles.GetRecommendedArticles(ctx, d.articleCount)
	if err != nil {
		d.logger.Error(ctx, "failed to fetch articles", "error", err)
		return nil
	}

	return articles
}

func (d *DailyDigest) composeInsight(ctx context.Context, daily *model.Problem, random []model.Problem, articles []model.Article) string {
	if d.writer == nil {
		return ""
	}

	insight, err := d.writer.Compose(ctx, daily, random, articles)
	if err != nil {
		d.logger.Error(ctx, "failed to compose insight", "error", err)
		return ""
	}

	return trimForDiscord(insight, 1900)
}

func (d *DailyDigest) buildNotification(daily *model.Problem, random []model.Problem, articles []model.Article, insight string) model.Notification {
	var fields []model.NotificationField

	if daily != nil {
		fields = append(fields, model.NotificationField{
			Name:   "Daily Challenge",
			Value:  formatProblemDetail(daily),
			Inline: false,
		})
	}

	if len(random) > 0 {
		fields = append(fields, model.NotificationField{
			Name:   "Practice Queue",
			Value:  formatPracticeQueue(random),
			Inline: false,
		})
	}

	if len(articles) > 0 {
		fields = append(fields, model.NotificationField{
			Name:   "Algorithm Reading List",
			Value:  formatArticleList(articles),
			Inline: false,
		})
	}

	description := "Here is your curated practice plan for today."
	if insight != "" {
		description = insight
	} else {
		description = fallbackDescription(daily)
	}

	return model.Notification{
		Title:       "Daily LeetCode & Algorithms Digest",
		Description: description,
		Fields:      fields,
	}
}

func formatProblemDetail(p *model.Problem) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**Link:** [%s](%s)\n", p.Title, p.Link))
	builder.WriteString(fmt.Sprintf("**Difficulty:** %s\n", p.Difficulty))
	if len(p.Topics) > 0 {
		builder.WriteString(fmt.Sprintf("**Topics:** %s\n", strings.Join(p.Topics, ", ")))
	}
	if p.Content != "" {
		summary := summarizeText(p.Content, 420)
		if summary != "" {
			builder.WriteString("\n> _")
			builder.WriteString(summary)
			builder.WriteString("_")
		}
	}
	return builder.String()
}

func trimForDiscord(content string, limit int) string {
	if len(content) <= limit {
		return content
	}
	trimmed := content[:limit]
	lastSpace := strings.LastIndex(trimmed, " ")
	if lastSpace > 0 {
		trimmed = trimmed[:lastSpace]
	}
	return trimmed + "..."
}

func summarizeText(content string, limit int) string {
	clean := strings.Join(strings.Fields(content), " ")
	if clean == "" {
		return ""
	}

	if len(clean) <= limit {
		return clean
	}

	trimmed := clean[:limit]
	lastSpace := strings.LastIndex(trimmed, " ")
	if lastSpace > 0 {
		trimmed = trimmed[:lastSpace]
	}

	return trimmed + "..."
}

func formatPracticeQueue(problems []model.Problem) string {
	lines := make([]string, 0, len(problems))
	for i, p := range problems {
		line := fmt.Sprintf("**%d. [%s](%s)**\n   Difficulty: *%s*", i+1, p.Title, p.Link, p.Difficulty)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n\n")
}

func formatArticleList(articles []model.Article) string {
	lines := make([]string, 0, len(articles))
	for i, a := range articles {
		source := a.Source
		if source == "" {
			source = "Curated"
		}
		line := fmt.Sprintf("**%d.** [%s](%s)\n   _from %s_", i+1, a.Title, a.Link, source)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n\n")
}

func fallbackDescription(daily *model.Problem) string {
	if daily == nil {
		return "Curated plan for algorithms practice today."
	}

	description := fmt.Sprintf("**Daily Focus:** %s (%s)\n", daily.Title, daily.Difficulty)
	if len(daily.Topics) > 0 {
		description += fmt.Sprintf("Chủ đề trọng tâm: %s.", strings.Join(daily.Topics, ", "))
	} else {
		description += "Tập trung phân tích kỹ thuật lõi và cách tối ưu lời giải."
	}
	return description
}
