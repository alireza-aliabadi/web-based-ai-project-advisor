package githubsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amin/web-based-ai-project-advisor/internal/cache"
	"github.com/amin/web-based-ai-project-advisor/internal/models"
)

type Service struct {
	token  string
	client *http.Client
	cache  *cache.Cache
}

func New(token string, c *cache.Cache) *Service {
	return &Service{
		token: token,
		client: &http.Client{Timeout: 20 * time.Second},
		cache: c,
	}
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]models.Repository, error) {
	if limit <= 0 {
		limit = 8
	}
	cacheKey := cache.Key("github", query, fmt.Sprintf("%d", limit))
	var cached []models.Repository
	if ok, _ := s.cache.GetJSON(ctx, cacheKey, &cached); ok && len(cached) > 0 {
		return cached, nil
	}

	q := url.Values{}
	q.Set("q", buildQuery(query))
	q.Set("sort", "stars")
	q.Set("order", "desc")
	q.Set("per_page", fmt.Sprintf("%d", limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/search/repositories?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ai-solution-architect")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fallbackRepos(query, limit), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fallbackRepos(query, limit), nil
	}

	var parsed struct {
		Items []struct {
			Name            string   `json:"name"`
			FullName        string   `json:"full_name"`
			Description     string   `json:"description"`
			HTMLURL         string   `json:"html_url"`
			StargazersCount int      `json:"stargazers_count"`
			ForksCount      int      `json:"forks_count"`
			Language        string   `json:"language"`
			Topics          []string `json:"topics"`
			UpdatedAt       string   `json:"updated_at"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fallbackRepos(query, limit), nil
	}

	out := make([]models.Repository, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		repo := models.Repository{
			Name:        item.Name,
			FullName:    item.FullName,
			Description: item.Description,
			URL:         item.HTMLURL,
			Stars:       item.StargazersCount,
			Forks:       item.ForksCount,
			Language:    item.Language,
			Topics:      item.Topics,
			UpdatedAt:   item.UpdatedAt,
		}
		if repo.Topics == nil {
			repo.Topics = []string{}
		}
		out = append(out, repo)
	}
	if len(out) == 0 {
		out = fallbackRepos(query, limit)
	} else {
		_ = s.cache.SetJSON(ctx, cacheKey, out, 30*time.Minute)
	}
	return out, nil
}

func (s *Service) FetchREADME(ctx context.Context, fullName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://api.github.com/repos/%s/readme", fullName), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.raw")
	req.Header.Set("User-Agent", "ai-solution-architect")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("readme status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 50_000))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildQuery(idea string) string {
	idea = strings.TrimSpace(idea)
	lower := strings.ToLower(idea)
	extra := " AI LLM"
	if strings.Contains(lower, "chat") || strings.Contains(lower, "bot") {
		extra = " chatbot RAG"
	}
	if strings.Contains(lower, "vision") || strings.Contains(lower, "image") {
		extra = " computer vision"
	}
	if strings.Contains(lower, "agent") {
		extra = " agent LLM"
	}
	// Keep query simple so GitHub search returns matches without a token.
	return strings.TrimSpace(idea + extra)
}

func fallbackRepos(query string, limit int) []models.Repository {
	demos := []models.Repository{
		{Name: "langchain", FullName: "langchain-ai/langchain", Description: "Build context-aware reasoning applications", URL: "https://github.com/langchain-ai/langchain", Stars: 90000, Forks: 14000, Language: "Python", Topics: []string{"llm", "agents", "rag"}, UpdatedAt: time.Now().Format(time.RFC3339)},
		{Name: "llama_index", FullName: "run-llama/llama_index", Description: "Data framework for LLM applications", URL: "https://github.com/run-llama/llama_index", Stars: 35000, Forks: 5000, Language: "Python", Topics: []string{"rag", "llm"}, UpdatedAt: time.Now().Format(time.RFC3339)},
		{Name: "transformers", FullName: "huggingface/transformers", Description: "State-of-the-art Machine Learning for PyTorch, TensorFlow, and JAX", URL: "https://github.com/huggingface/transformers", Stars: 130000, Forks: 26000, Language: "Python", Topics: []string{"nlp", "transformers"}, UpdatedAt: time.Now().Format(time.RFC3339)},
		{Name: "ollama", FullName: "ollama/ollama", Description: "Get up and running with large language models locally", URL: "https://github.com/ollama/ollama", Stars: 90000, Forks: 7000, Language: "Go", Topics: []string{"llm", "local"}, UpdatedAt: time.Now().Format(time.RFC3339)},
		{Name: "vllm", FullName: "vllm-project/vllm", Description: "High-throughput and memory-efficient inference engine", URL: "https://github.com/vllm-project/vllm", Stars: 30000, Forks: 4500, Language: "Python", Topics: []string{"inference", "llm"}, UpdatedAt: time.Now().Format(time.RFC3339)},
	}
	_ = query
	if limit > len(demos) {
		limit = len(demos)
	}
	return demos[:limit]
}
