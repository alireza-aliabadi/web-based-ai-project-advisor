package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/amin/web-based-ai-project-advisor/internal/services/llm"
)

type Service struct {
	llm        *llm.Client
	qdrantURL  string
	collection string
	http       *http.Client
}

func New(llmClient *llm.Client, qdrantURL, collection string) *Service {
	return &Service{
		llm:        llmClient,
		qdrantURL:  strings.TrimRight(qdrantURL, "/"),
		collection: collection,
		http:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Service) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return s.llm.Embed(ctx, texts)
}

func (s *Service) EnsureCollection(ctx context.Context, dim int) error {
	payload := map[string]any{
		"vectors": map[string]any{
			"size":     dim,
			"distance": "Cosine",
		},
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/collections/%s", s.qdrantURL, s.collection), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (s *Service) Upsert(ctx context.Context, id uint64, vector []float32, payload map[string]any) error {
	body := map[string]any{
		"points": []map[string]any{
			{"id": id, "vector": vector, "payload": payload},
		},
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/collections/%s/points?wait=true", s.qdrantURL, s.collection), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (s *Service) Search(ctx context.Context, vector []float32, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}
	body := map[string]any{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/collections/%s/points/search", s.qdrantURL, s.collection), bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("qdrant search status %d", resp.StatusCode)
	}
	var parsed struct {
		Result []struct {
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(parsed.Result))
	for _, r := range parsed.Result {
		p := r.Payload
		if p == nil {
			p = map[string]any{}
		}
		p["_score"] = r.Score
		out = append(out, p)
	}
	return out, nil
}

func (s *Service) Similarity(a, b []float32) float64 {
	return llm.Cosine(a, b)
}
