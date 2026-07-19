package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiKey     string
	baseURL    string
	model      string
	embedModel string
	http       *http.Client
}

func New(apiKey, baseURL, model, embedModel string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		embedModel: embedModel,
		http:       &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *Client) Available() bool {
	return c.apiKey != ""
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *Client) Chat(ctx context.Context, system string, messages []Message) (string, error) {
	if !c.Available() {
		return "", fmt.Errorf("LLM API key not configured")
	}
	payload := map[string]any{
		"model":       c.model,
		"messages":    append([]Message{{Role: "system", Content: system}}, messages...),
		"temperature": 0.3,
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty llm response")
	}
	return parsed.Choices[0].Message.Content, nil
}

func (c *Client) ChatJSON(ctx context.Context, system, user string, dest any) error {
	content, err := c.Chat(ctx, system+"\nRespond with valid JSON only.", []Message{{Role: "user", Content: user}})
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(stripCodeFence(content)), dest)
}

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if !c.Available() {
		return heuristicEmbeddings(texts), nil
	}
	payload := map[string]any{"model": c.embedModel, "input": texts}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return heuristicEmbeddings(texts), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return heuristicEmbeddings(texts), nil
	}
	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return heuristicEmbeddings(texts), nil
	}
	out := make([][]float32, len(texts))
	for _, d := range parsed.Data {
		if d.Index >= 0 && d.Index < len(out) {
			out[d.Index] = d.Embedding
		}
	}
	for i, v := range out {
		if v == nil {
			out[i] = heuristicEmbeddings([]string{texts[i]})[0]
		}
	}
	return out, nil
}

func Cosine(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func heuristicEmbeddings(texts []string) [][]float32 {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		vec := make([]float32, 64)
		for _, tok := range strings.Fields(strings.ToLower(t)) {
			h := 0
			for _, ch := range tok {
				h = (h*31 + int(ch)) % 64
			}
			vec[h]++
		}
		out[i] = l2Normalize(vec)
	}
	return out
}

func l2Normalize(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	if sum == 0 {
		return vec
	}
	inv := float32(1.0 / math.Sqrt(sum))
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = v * inv
	}
	return out
}
