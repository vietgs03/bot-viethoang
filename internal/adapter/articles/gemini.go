package articles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bot-viethoang/internal/domain/model"
	"bot-viethoang/internal/domain/ports"
)

const geminiEndpointTemplate = "https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s"

// GeminiProvider uses Google Gemini to suggest algorithm reading topics.
type GeminiProvider struct {
	httpClient *http.Client
	apiKey     string
	model      string
	logger     ports.Logger
	topicLimit int
}

// NewGeminiProvider builds a Gemini-backed article provider.
func NewGeminiProvider(apiKey, model string, timeout time.Duration, topicLimit int, logger ports.Logger) *GeminiProvider {
	return &GeminiProvider{
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     apiKey,
		model:      model,
		logger:     logger,
		topicLimit: topicLimit,
	}
}

// GetRecommendedArticles asks Gemini to produce curated algorithm resources.
func (g *GeminiProvider) GetRecommendedArticles(ctx context.Context, count int) ([]model.Article, error) {
	if count <= 0 {
		return nil, nil
	}

	if g.apiKey == "" || g.model == "" {
		return nil, fmt.Errorf("gemini configuration missing")
	}

	requestCount := count
	if g.topicLimit > 0 && requestCount > g.topicLimit {
		requestCount = g.topicLimit
	}

	prompt := g.buildPrompt(requestCount)

	body, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0.4,
			"topP":            0.8,
			"maxOutputTokens": 768,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf(geminiEndpointTemplate, g.model, g.apiKey), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini returned status %d", resp.StatusCode)
	}

	var payload struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}

	raw := g.extractFirstText(payload.Candidates)
	if raw == "" {
		return nil, fmt.Errorf("gemini response is empty")
	}

	return g.parseArticles(raw, requestCount)
}

func (g *GeminiProvider) buildPrompt(count int) string {
	return fmt.Sprintf(`You are an expert algorithms mentor curating daily study material.
Provide a JSON array with exactly %d unique items.
Each item must have keys "title", "link", and "source".
- "title": concise topic or article title (max 80 characters).
- "link": valid URL to a high-quality free resource (official docs, reputable blogs, lectures).
- "source": the site or author name.
Do not include any additional text outside the JSON array.`, count)
}

func (g *GeminiProvider) extractFirstText(candidates []struct {
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}) string {
	for _, candidate := range candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return part.Text
			}
		}
	}
	return ""
}

func (g *GeminiProvider) parseArticles(raw string, limit int) ([]model.Article, error) {
	raw = strings.TrimSpace(raw)
	// Gemini may wrap JSON in markdown fences; strip them if present.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```JSON")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var items []struct {
		Title  string `json:"title"`
		Link   string `json:"link"`
		Source string `json:"source"`
	}

	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parse gemini JSON: %w", err)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("gemini returned no items")
	}

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	results := make([]model.Article, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)
		source := strings.TrimSpace(item.Source)
		if title == "" {
			continue
		}
		if link == "" {
			link = fmt.Sprintf("https://www.google.com/search?q=%s", strings.ReplaceAll(title, " ", "+"))
		}
		if source == "" {
			source = "Gemini Suggestion"
		}

		results = append(results, model.Article{
			Title:  title,
			Link:   link,
			Source: source,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("gemini suggestions were empty")
	}

	return results, nil
}
