package articles

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"bot-viethoang/internal/domain/model"
	"bot-viethoang/internal/domain/ports"
)

const (
	mediumFeedPrimary = "https://medium.com/feed/tag/algorithms"
	mediumFeedProxy   = "https://r.jina.ai/https://medium.com/feed/tag/algorithms"
)

// MediumProvider retrieves algorithm articles from Medium RSS feeds, with a proxy fallback.
type MediumProvider struct {
	httpClient *http.Client
	logger     ports.Logger
	rnd        *rand.Rand
}

// NewMediumProvider builds a Medium RSS provider.
func NewMediumProvider(timeout time.Duration, logger ports.Logger) *MediumProvider {
	return &MediumProvider{
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano() + 7)),
	}
}

// GetRecommendedArticles fetches and samples Medium posts tagged algorithms.
func (m *MediumProvider) GetRecommendedArticles(ctx context.Context, count int) ([]model.Article, error) {
	if count <= 0 {
		return nil, nil
	}

	aggregated := make([]model.Article, 0)
	var lastErr error

	if items, err := m.fetchPrimary(ctx); err == nil {
		aggregated = append(aggregated, items...)
	} else {
		lastErr = err
		if m.logger != nil {
			m.logger.Error(ctx, "medium primary feed failed", "error", err)
		}
	}

	if len(aggregated) < count {
		if items, err := m.fetchProxy(ctx); err == nil {
			aggregated = append(aggregated, items...)
		} else {
			lastErr = err
			if m.logger != nil {
				m.logger.Error(ctx, "medium proxy feed failed", "error", err)
			}
		}
	}

	if len(aggregated) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no medium articles available")
	}

	dedup := make(map[string]model.Article)
	for _, article := range aggregated {
		key := strings.ToLower(strings.TrimSpace(article.Link))
		if key == "" {
			continue
		}
		if _, exists := dedup[key]; exists {
			continue
		}
		dedup[key] = article
	}

	unique := make([]model.Article, 0, len(dedup))
	for _, article := range dedup {
		unique = append(unique, article)
	}

	if len(unique) == 0 {
		return nil, fmt.Errorf("medium feed returned no usable articles")
	}

	m.rnd.Shuffle(len(unique), func(i, j int) {
		unique[i], unique[j] = unique[j], unique[i]
	})

	if count > len(unique) {
		count = len(unique)
	}

	return unique[:count], nil
}

func (m *MediumProvider) fetchPrimary(ctx context.Context) ([]model.Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediumFeedPrimary, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create medium request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DailyBot/1.0; +https://github.com)")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch medium feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("medium feed status %d: %s", resp.StatusCode, string(body))
	}

	payload := struct {
		Channel struct {
			Items []struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
			} `xml:"item"`
		} `xml:"channel"`
	}{}

	if err := xml.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode medium feed: %w", err)
	}

	items := payload.Channel.Items
	articles := make([]model.Article, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		link := normalizeMediumLink(strings.TrimSpace(item.Link))
		if title == "" || link == "" {
			continue
		}
		articles = append(articles, model.Article{
			Title:  title,
			Link:   link,
			Source: "Medium",
		})
	}

	if len(articles) == 0 {
		return nil, fmt.Errorf("medium primary feed empty")
	}

	return articles, nil
}

func (m *MediumProvider) fetchProxy(ctx context.Context) ([]model.Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediumFeedProxy, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create medium proxy request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DailyBot/1.0; +https://github.com)")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch medium proxy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("medium proxy status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read medium proxy body: %w", err)
	}

	content := string(data)
	blocks := strings.Split(content, "===============")
	articles := make([]model.Article, 0, len(blocks))
	for _, block := range blocks {
		title := extractCDATA(block)
		link := extractFirstMediumURL(block)
		if title == "" || strings.EqualFold(title, "Algorithms on Medium") {
			continue
		}
		if link == "" {
			continue
		}

		articles = append(articles, model.Article{
			Title:  title,
			Link:   link,
			Source: "Medium",
		})
	}

	if len(articles) == 0 {
		return nil, fmt.Errorf("medium proxy returned no articles")
	}

	return articles, nil
}

func extractCDATA(block string) string {
	start := strings.Index(block, "<![CDATA[")
	end := strings.Index(block, "]]>")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	raw := block[start+9 : end]
	return strings.TrimSpace(html.UnescapeString(raw))
}

func extractFirstMediumURL(block string) string {
	tokens := strings.Fields(block)
	var candidate string
	for _, token := range tokens {
		if strings.HasPrefix(token, "https://medium.com/") {
			clean := normalizeMediumLink(token)
			if strings.Contains(clean, "/tag/") {
				continue
			}
			if candidate == "" || strings.Contains(token, "?source=") {
				candidate = clean
				if strings.Contains(token, "?source=") {
					break
				}
			}
		}
	}
	return candidate
}

func normalizeMediumLink(link string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}
	link = strings.TrimPrefix(link, "<")
	link = strings.TrimSuffix(link, ">")
	if idx := strings.Index(link, "?"); idx != -1 {
		link = link[:idx]
	}
	return link
}
