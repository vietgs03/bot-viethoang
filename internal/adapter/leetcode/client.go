package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"bot-viethoang/internal/domain/model"
	"bot-viethoang/internal/domain/ports"
)

const (
	graphQLEndpoint    = "https://leetcode.com/graphql"
	problemsetEndpoint = "https://leetcode.com/api/problems/all/"
)

// Client implements ProblemProvider using LeetCode public endpoints.
type Client struct {
	httpClient *http.Client
	logger     ports.Logger
	rnd        *rand.Rand
}

// New creates a new LeetCode client.
func New(timeout time.Duration, logger ports.Logger) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetDailyChallenge retrieves the daily LeetCode challenge.
func (c *Client) GetDailyChallenge(ctx context.Context) (*model.Problem, error) {
	payload := map[string]string{
		"query": `query questionOfToday { activeDailyCodingChallengeQuestion { link question { questionFrontendId title titleSlug difficulty content topicTags { name } } } }`,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal graphql payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphQLEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://leetcode.com")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(data))
	}

	var gqlResp struct {
		Data struct {
			ActiveDailyCodingChallengeQuestion struct {
				Link     string `json:"link"`
				Question struct {
					QuestionFrontendID string `json:"questionFrontendId"`
					Title              string `json:"title"`
					TitleSlug          string `json:"titleSlug"`
					Difficulty         string `json:"difficulty"`
					Content            string `json:"content"`
					TopicTags          []struct {
						Name string `json:"name"`
					} `json:"topicTags"`
				} `json:"question"`
			} `json:"activeDailyCodingChallengeQuestion"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	q := gqlResp.Data.ActiveDailyCodingChallengeQuestion
	if q.Question.TitleSlug == "" {
		return nil, fmt.Errorf("empty daily challenge data")
	}

	topics := make([]string, 0, len(q.Question.TopicTags))
	for _, tag := range q.Question.TopicTags {
		topics = append(topics, tag.Name)
	}

	return &model.Problem{
		ID:         parseInt(q.Question.QuestionFrontendID),
		Title:      q.Question.Title,
		Slug:       q.Question.TitleSlug,
		Difficulty: q.Question.Difficulty,
		Link:       resolveLink(q.Question.TitleSlug, q.Link),
		Content:    strings.TrimSpace(htmlToText(q.Question.Content)),
		Topics:     topics,
	}, nil
}

// GetRandomProblems returns a random subset of problems from the global problemset.
func (c *Client) GetRandomProblems(ctx context.Context, count int) ([]model.Problem, error) {
	if count <= 0 {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, problemsetEndpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Referer", "https://leetcode.com")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(data))
	}

	var payload struct {
		StatStatusPairs []struct {
			Stat struct {
				FrontendQuestionID int    `json:"frontend_question_id"`
				QuestionID         int    `json:"question_id"`
				QuestionTitle      string `json:"question__title"`
				QuestionTitleSlug  string `json:"question__title_slug"`
			} `json:"stat"`
			Difficulty struct {
				Level int `json:"level"`
			} `json:"difficulty"`
			PaidOnly bool `json:"paid_only"`
		} `json:"stat_status_pairs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	candidates := make([]model.Problem, 0, len(payload.StatStatusPairs))
	for _, item := range payload.StatStatusPairs {
		if item.PaidOnly || item.Stat.QuestionTitleSlug == "" {
			continue
		}

		candidates = append(candidates, model.Problem{
			ID:         item.Stat.QuestionID,
			Title:      item.Stat.QuestionTitle,
			Slug:       item.Stat.QuestionTitleSlug,
			Difficulty: difficultyToString(item.Difficulty.Level),
			Link:       resolveLink(item.Stat.QuestionTitleSlug, ""),
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no problems available")
	}

	if count > len(candidates) {
		count = len(candidates)
	}

	c.rnd.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	return candidates[:count], nil
}

func difficultyToString(level int) string {
	switch level {
	case 1:
		return "Easy"
	case 2:
		return "Medium"
	case 3:
		return "Hard"
	default:
		return "Unknown"
	}
}

func parseInt(val string) int {
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return n
}

func resolveLink(slug, fallback string) string {
	if fallback != "" {
		return "https://leetcode.com" + fallback
	}
	return fmt.Sprintf("https://leetcode.com/problems/%s/", slug)
}

func htmlToText(input string) string {
	if input == "" {
		return ""
	}

	node, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return input
	}

	var builder strings.Builder
	extractText(node, &builder)
	return builder.String()
}

func extractText(node *html.Node, builder *strings.Builder) {
	switch node.Type {
	case html.TextNode:
		builder.WriteString(node.Data)
	case html.ElementNode:
		if node.Data == "br" || node.Data == "p" || node.Data == "li" {
			builder.WriteRune('\n')
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		extractText(child, builder)
	}

	if node.Type == html.ElementNode && (node.Data == "p" || node.Data == "li") {
		builder.WriteRune('\n')
	}
}
