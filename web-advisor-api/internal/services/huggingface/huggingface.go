package huggingface

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
		token:  token,
		client: &http.Client{Timeout: 20 * time.Second},
		cache:  c,
	}
}

func (s *Service) Search(ctx context.Context, query, task string, limit int) ([]models.Model, error) {
	if limit <= 0 {
		limit = 8
	}
	cacheKey := cache.Key("hf", query, task, fmt.Sprintf("%d", limit))
	var cached []models.Model
	if ok, _ := s.cache.GetJSON(ctx, cacheKey, &cached); ok && len(cached) > 0 {
		return cached, nil
	}

	q := url.Values{}
	q.Set("search", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("sort", "downloads")
	q.Set("direction", "-1")
	if task != "" {
		q.Set("pipeline_tag", mapTask(task))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://huggingface.co/api/models?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ai-solution-architect")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fallbackModels(query, limit), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fallbackModels(query, limit), nil
	}

	var items []struct {
		ID           string `json:"id"`
		PipelineTag  string `json:"pipeline_tag"`
		Downloads    int    `json:"downloads"`
		Likes        int    `json:"likes"`
		LibraryName  string `json:"library_name"`
		ModelID      string `json:"modelId"`
		Tags         []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return fallbackModels(query, limit), nil
	}

	out := make([]models.Model, 0, len(items))
	for _, item := range items {
		name := item.ID
		if name == "" {
			name = item.ModelID
		}
		arch := ""
		license := ""
		for _, t := range item.Tags {
			if strings.HasPrefix(t, "license:") {
				license = strings.TrimPrefix(t, "license:")
			}
			if strings.Contains(t, "bert") || strings.Contains(t, "llama") || strings.Contains(t, "mistral") || strings.Contains(t, "gpt") {
				arch = t
			}
		}
		out = append(out, models.Model{
			Name:         name,
			Task:         item.PipelineTag,
			Downloads:    item.Downloads,
			Likes:        item.Likes,
			Architecture: arch,
			License:      license,
			Framework:    item.LibraryName,
			URL:          "https://huggingface.co/" + name,
			Hardware:     estimateHardware(item.Downloads, item.PipelineTag),
		})
	}
	if len(out) == 0 {
		out = fallbackModels(query, limit)
	} else {
		_ = s.cache.SetJSON(ctx, cacheKey, out, 30*time.Minute)
	}
	return out, nil
}

func mapTask(task string) string {
	t := strings.ToLower(task)
	switch {
	case strings.Contains(t, "question") || strings.Contains(t, "qa") || strings.Contains(t, "chat"):
		return "text-generation"
	case strings.Contains(t, "embed") || strings.Contains(t, "retrieval") || strings.Contains(t, "rag"):
		return "feature-extraction"
	case strings.Contains(t, "classif"):
		return "text-classification"
	case strings.Contains(t, "vision") || strings.Contains(t, "image"):
		return "image-classification"
	case strings.Contains(t, "speech") || strings.Contains(t, "audio"):
		return "automatic-speech-recognition"
	default:
		return "text-generation"
	}
}

func estimateHardware(downloads int, task string) string {
	_ = downloads
	if strings.Contains(strings.ToLower(task), "text-generation") {
		return "GPU recommended (8–24 GB VRAM) or managed API"
	}
	return "CPU feasible; GPU optional for speed"
}

func fallbackModels(query string, limit int) []models.Model {
	_ = query
	demos := []models.Model{
		{Name: "meta-llama/Meta-Llama-3.1-8B-Instruct", Task: "text-generation", Downloads: 2000000, Likes: 5000, Architecture: "llama", License: "llama3.1", Framework: "transformers", URL: "https://huggingface.co/meta-llama/Meta-Llama-3.1-8B-Instruct", Hardware: "GPU 16+ GB VRAM"},
		{Name: "mistralai/Mistral-7B-Instruct-v0.3", Task: "text-generation", Downloads: 1500000, Likes: 4000, Architecture: "mistral", License: "apache-2.0", Framework: "transformers", URL: "https://huggingface.co/mistralai/Mistral-7B-Instruct-v0.3", Hardware: "GPU 12+ GB VRAM"},
		{Name: "sentence-transformers/all-MiniLM-L6-v2", Task: "feature-extraction", Downloads: 5000000, Likes: 2000, Architecture: "bert", License: "apache-2.0", Framework: "sentence-transformers", URL: "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2", Hardware: "CPU friendly"},
		{Name: "BAAI/bge-small-en-v1.5", Task: "feature-extraction", Downloads: 1000000, Likes: 800, Architecture: "bert", License: "mit", Framework: "sentence-transformers", URL: "https://huggingface.co/BAAI/bge-small-en-v1.5", Hardware: "CPU friendly"},
		{Name: "openai/whisper-large-v3", Task: "automatic-speech-recognition", Downloads: 800000, Likes: 1500, Architecture: "whisper", License: "apache-2.0", Framework: "transformers", URL: "https://huggingface.co/openai/whisper-large-v3", Hardware: "GPU recommended"},
	}
	if limit > len(demos) {
		limit = len(demos)
	}
	return demos[:limit]
}
