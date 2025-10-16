package articles

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"bot-viethoang/internal/domain/model"
	"bot-viethoang/internal/domain/ports"
)

const devtoEndpoint = "https://dev.to/api/articles?tag=algorithms&per_page=50"

// DevToProvider fetches algorithm-related articles from dev.to.
type DevToProvider struct {
	httpClient *http.Client
	logger     ports.Logger
	rnd        *rand.Rand
}

// NewDevToProvider builds a DevToProvider with the supplied timeout.
func NewDevToProvider(timeout time.Duration, logger ports.Logger) *DevToProvider {
	return &DevToProvider{
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano() + 1)),
	}
}

// GetRecommendedArticles fetches and randomly selects algorithm-related articles.
func (d *DevToProvider) GetRecommendedArticles(ctx context.Context, count int) ([]model.Article, error) {
	if count <= 0 {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, devtoEndpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var payload []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
		User  struct {
			Name     string `json:"name"`
			Username string `json:"username"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(payload) == 0 {
		return nil, fmt.Errorf("no articles returned")
	}

	if count > len(payload) {
		count = len(payload)
	}

	d.rnd.Shuffle(len(payload), func(i, j int) {
		payload[i], payload[j] = payload[j], payload[i]
	})

	articles := make([]model.Article, 0, count)
	for i := 0; i < count; i++ {
		item := payload[i]
		source := item.User.Name
		if source == "" {
			source = item.User.Username
		}
		if source == "" {
			source = "dev.to"
		}
		articles = append(articles, model.Article{
			Title:  item.Title,
			Link:   item.URL,
			Source: source,
		})
	}

	return articles, nil
}
