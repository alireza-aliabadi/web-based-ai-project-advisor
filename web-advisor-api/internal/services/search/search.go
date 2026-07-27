package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/amin/web-based-ai-project-advisor/internal/cache"
	"github.com/amin/web-based-ai-project-advisor/internal/models"
)

type Service struct {
	tavilyKey     string
	tavilyBaseURL string
	client        *http.Client
	cache         *cache.Cache
}

func New(tavilyKey, tavilyBaseURL string, c *cache.Cache) *Service {
	return &Service{
		tavilyKey:     tavilyKey,
		tavilyBaseURL: tavilyBaseURL,
		client:        &http.Client{Timeout: 20 * time.Second},
		cache:         c,
	}
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]models.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	cacheKey := cache.Key("web", query, fmt.Sprintf("%d", limit))
	var cached []models.SearchResult
	if ok, _ := s.cache.GetJSON(ctx, cacheKey, &cached); ok {
		return cached, nil
	}

	var results []models.SearchResult
	var err error
	if s.tavilyKey != "" {
		results, err = s.searchTavily(ctx, query, limit)
	} else {
		results = fallbackSearch(query)
	}
	if err != nil || len(results) == 0 {
		results = fallbackSearch(query)
	}
	_ = s.cache.SetJSON(ctx, cacheKey, results, 20*time.Minute)
	return results, nil
}

func (s *Service) searchTavily(ctx context.Context, query string, limit int) ([]models.SearchResult, error) {
	payload := map[string]any{
		"api_key":      s.tavilyKey,
		"query":        query + " AI framework architecture 2025 2026",
		"search_depth": "basic",
		"max_results":  limit,
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tavilyBaseURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tavily status %d", resp.StatusCode)
	}
	var parsed struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]models.SearchResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		out = append(out, models.SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Content, Score: r.Score})
	}
	return out, nil
}

func fallbackSearch(query string) []models.SearchResult {
	return []models.SearchResult{
		{Title: "Building production RAG systems", URL: "https://www.anthropic.com/research", Snippet: "Patterns for retrieval-augmented generation around: " + query},
		{Title: "Hugging Face Tasks Guide", URL: "https://huggingface.co/tasks", Snippet: "Model tasks and pipelines relevant to AI project design"},
		{Title: "Awesome LLM Apps", URL: "https://github.com/Shubhamsaboo/awesome-llm-apps", Snippet: "Curated list of LLM applications and architectures"},
	}
}
