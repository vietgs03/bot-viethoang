package writing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bot-viethoang/internal/domain/model"
	"bot-viethoang/internal/domain/ports"
)

const (
	geminiWriteEndpointTemplate = "https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s"
	maxDiscordDescription       = 2000
)

// GeminiWriter composes academic-style insights using Gemini.
type GeminiWriter struct {
	httpClient *http.Client
	apiKey     string
	model      string
	logger     ports.Logger
}

// NewGeminiWriter constructs a GeminiWriter.
func NewGeminiWriter(apiKey, model string, timeout time.Duration, logger ports.Logger) *GeminiWriter {
	return &GeminiWriter{
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     apiKey,
		model:      model,
		logger:     logger,
	}
}

// var _ ports.ArticleWriter = (*GeminiWriter)(nil)

// Compose generates a structured narrative referencing problems and articles.
func (g *GeminiWriter) Compose(ctx context.Context, daily *model.Problem, random []model.Problem, articles []model.Article) (string, error) {
	if g.apiKey == "" || g.model == "" {
		return "", fmt.Errorf("gemini writer not configured")
	}

	prompt := g.buildPrompt(daily, random, articles)

	body, err := g.buildRequestBody(prompt)
	if err != nil {
		return "", err
	}

	modelsToTry := []string{g.model}
	var lastErr error
	for idx, modelName := range modelsToTry {
		text, status, err := g.generate(ctx, modelName, body)
		if err == nil {
			if idx > 0 && g.logger != nil {
				g.logger.Info(ctx, "gemini writer fallback succeeded", "model", modelName)
			}
			return trimText(text, maxDiscordDescription), nil
		}
		lastErr = err
		if status == http.StatusNotFound || status == http.StatusTooManyRequests {
			continue
		}
		break
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("gemini writer failed without detailed error")
	}
	return "", lastErr
}

func (g *GeminiWriter) buildPrompt(daily *model.Problem, random []model.Problem, articles []model.Article) string {
	var builder strings.Builder
	builder.WriteString("Write Vietnamese algorithm notes with this exact structure:\n\n")
	builder.WriteString("## 🎯 **Phân Tích Bài Toán**\n")
	builder.WriteString("Main approach + why effective (2 sentences)\n\n")
	builder.WriteString("## 📚 **Concept từ Grokking Algorithms**\n")
	builder.WriteString("Quote concept + connection to problem (2 sentences)\n\n")
	builder.WriteString("## 💡 **Study Plan**\n")
	builder.WriteString("2-3 practice steps\n\n")
	builder.WriteString("Rules: English tech terms, Markdown links, max 600 words total.\n\n")

	if daily != nil {
		builder.WriteString("Daily LeetCode Challenge:\n")
		builder.WriteString(fmt.Sprintf("- %s (%s) – %s\n", daily.Title, daily.Difficulty, daily.Link))
		if len(daily.Topics) > 0 {
			builder.WriteString(fmt.Sprintf("  Topics: %s\n", strings.Join(daily.Topics, ", ")))
		}
	}

	if len(random) > 0 {
		builder.WriteString("\nAdditional Practice Problems:\n")
		for _, p := range random {
			builder.WriteString(fmt.Sprintf("- %s (%s) – %s\n", p.Title, p.Difficulty, p.Link))
		}
	}

	if len(articles) > 0 {
		builder.WriteString("\nBackground Reading:\n")
		for _, a := range articles {
			builder.WriteString(fmt.Sprintf("- %s – %s\n", a.Title, a.Link))
		}
	}

	builder.WriteString("\nWrite the 3 sections above. Be concise.\n")
	return builder.String()
}

func (g *GeminiWriter) buildRequestBody(prompt string) ([]byte, error) {
	payload := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0.3,
			"topP":            0.8,
			"maxOutputTokens": 2500,
		},
		"safetySettings": []map[string]any{
			{"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_NONE"},
			{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_NONE"},
			{"category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "BLOCK_NONE"},
			{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_NONE"},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini writer payload: %w", err)
	}
	return body, nil
}

func (g *GeminiWriter) generate(ctx context.Context, model string, body []byte) (string, int, error) {
	endpoint := fmt.Sprintf(geminiWriteEndpointTemplate, model, g.apiKey)

	// Log request details for debugging
	if g.logger != nil {
		g.logger.Info(ctx, "calling gemini API",
			"model", model,
			"endpoint", strings.Split(endpoint, "?")[0], // hide API key
			"requestSize", len(body))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("create gemini writer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("call gemini writer: %w", err)
	}
	defer resp.Body.Close()

	// Read full response body for better debugging
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", resp.StatusCode, fmt.Errorf("gemini writer status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var payload struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason,omitempty"`
		} `json:"promptFeedback,omitempty"`
	}

	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		if g.logger != nil {
			g.logger.Error(ctx, "failed to decode gemini response", "error", err, "body", string(bodyBytes[:min(len(bodyBytes), 500)]))
		}
		return "", resp.StatusCode, fmt.Errorf("decode gemini writer response: %w", err)
	}

	// Log response structure for debugging
	if g.logger != nil && len(payload.Candidates) > 0 {
		partsCount := len(payload.Candidates[0].Content.Parts)
		g.logger.Info(ctx, "gemini response received",
			"candidates", len(payload.Candidates),
			"finishReason", payload.Candidates[0].FinishReason,
			"blockReason", payload.PromptFeedback.BlockReason,
			"partsCount", partsCount)
	}

	text := extractCandidateText(payload.Candidates)
	if text == "" {
		// Log full response when empty
		if g.logger != nil {
			finishReason := ""
			if len(payload.Candidates) > 0 {
				finishReason = payload.Candidates[0].FinishReason
			}
			g.logger.Error(ctx, "gemini returned empty text",
				"rawResponse", string(bodyBytes[:min(len(bodyBytes), 800)]),
				"candidatesCount", len(payload.Candidates),
				"finishReason", finishReason)
		}

		// Better error message
		if len(payload.Candidates) > 0 && payload.Candidates[0].FinishReason == "MAX_TOKENS" {
			return "", resp.StatusCode, fmt.Errorf("gemini hit token limit before generating output (try increasing maxOutputTokens)")
		}

		return "", resp.StatusCode, fmt.Errorf("gemini writer returned empty text (candidates: %d)", len(payload.Candidates))
	}

	return text, resp.StatusCode, nil
}

func extractCandidateText(candidates []struct {
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}) string {
	for _, candidate := range candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return strings.TrimSpace(part.Text)
			}
		}
	}
	return ""
}

func trimText(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return strings.TrimSpace(text[:max-3]) + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
