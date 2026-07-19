package githubsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/amin/web-based-ai-project-advisor/internal/cache"
	"github.com/amin/web-based-ai-project-advisor/internal/models"
)

// Infra/tooling repos that are useful for building, but are not "similar projects".
var toolingRepos = map[string]struct{}{
	"langchain-ai/langchain": {},
	"langchain-ai/langgraph": {},
	"run-llama/llama_index":  {},
	"huggingface/transformers": {},
	"huggingface/diffusers": {},
	"ollama/ollama": {},
	"vllm-project/vllm": {},
	"openai/openai-python": {},
	"openai/openai-node": {},
	"grpc/grpc": {},
	"pytorch/pytorch": {},
	"tensorflow/tensorflow": {},
	"microsoft/autogen": {},
	"crewAIInc/crewAI": {},
}

var ideaFillers = []string{
	"i want to", "i'd like to", "i would like to", "we want to",
	"build a", "build an", "create a", "create an", "make a", "make an",
	"develop a", "develop an", "implement a", "implement an",
	"looking for", "help me", "please",
}

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

func (s *Service) Search(ctx context.Context, query string, limit int) ([]models.Repository, error) {
	if limit <= 0 {
		limit = 8
	}
	cacheKey := cache.Key("github", query, fmt.Sprintf("%d", limit))
	var cached []models.Repository
	if ok, _ := s.cache.GetJSON(ctx, cacheKey, &cached); ok && len(cached) > 0 {
		return filterTooling(cached), nil
	}

	q := url.Values{}
	q.Set("q", buildQuery(query))
	q.Set("sort", "stars")
	q.Set("order", "desc")
	q.Set("per_page", fmt.Sprintf("%d", limit*2)) // over-fetch so tooling filter still leaves enough

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
	out = filterTooling(out)
	if len(out) > limit {
		out = out[:limit]
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

// buildQuery turns a natural-language idea into a GitHub search that prefers
// similar end-user projects over generic AI infrastructure libraries.
func buildQuery(idea string) string {
	keywords := extractKeywords(idea)
	lower := strings.ToLower(keywords)

	// Light domain hints — avoid framework terms like "RAG" / "LLM" that
	// dominate results with infra repos.
	switch {
	case strings.Contains(lower, "chat") || strings.Contains(lower, "bot") || strings.Contains(lower, "assistant"):
		if !strings.Contains(lower, "chatbot") {
			keywords += " chatbot"
		}
	case strings.Contains(lower, "vision") || strings.Contains(lower, "image") || strings.Contains(lower, "detect"):
		keywords += " computer-vision"
	case strings.Contains(lower, "agent"):
		keywords += " agent"
	}

	names := make([]string, 0, len(toolingRepos))
	for fullName := range toolingRepos {
		names = append(names, fullName)
	}
	sort.Strings(names)
	exclusions := make([]string, 0, len(names))
	for _, fullName := range names {
		exclusions = append(exclusions, "-repo:"+fullName)
	}
	return strings.TrimSpace(keywords + " " + strings.Join(exclusions, " "))
}

func extractKeywords(idea string) string {
	s := strings.ToLower(strings.TrimSpace(idea))
	for _, filler := range ideaFillers {
		s = strings.ReplaceAll(s, filler, " ")
	}
	// Drop punctuation, keep words that carry meaning.
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	parts := strings.Fields(b.String())
	stop := map[string]struct{}{
		"a": {}, "an": {}, "the": {}, "to": {}, "for": {}, "and": {}, "or": {},
		"with": {}, "using": {}, "my": {}, "our": {}, "that": {}, "this": {},
		"app": {}, "application": {}, "system": {}, "project": {}, "tool": {},
	}
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if _, skip := stop[p]; skip || len(p) < 2 {
			continue
		}
		kept = append(kept, p)
	}
	if len(kept) == 0 {
		return strings.TrimSpace(idea)
	}
	// Cap length so GitHub search stays focused.
	if len(kept) > 6 {
		kept = kept[:6]
	}
	return strings.Join(kept, " ")
}

func filterTooling(repos []models.Repository) []models.Repository {
	out := make([]models.Repository, 0, len(repos))
	for _, r := range repos {
		key := strings.ToLower(r.FullName)
		if _, isTool := toolingRepos[key]; isTool {
			continue
		}
		// Also drop obvious infra names even if not in the exact map.
		name := strings.ToLower(r.Name)
		if name == "langchain" || name == "llama_index" || name == "transformers" ||
			name == "ollama" || name == "vllm" || name == "pytorch" || name == "tensorflow" {
			continue
		}
		out = append(out, r)
	}
	return out
}

func fallbackRepos(query string, limit int) []models.Repository {
	lower := strings.ToLower(query)
	now := time.Now().Format(time.RFC3339)

	var demos []models.Repository
	switch {
	case containsAny(lower, "medical", "health", "clinic", "patient"):
		demos = []models.Repository{
			{Name: "Multi-Agent-Medical-Assistant", FullName: "souvikmajumder26/Multi-Agent-Medical-Assistant", Description: "GenAI multi-agent medical diagnostics and healthcare research assistant", URL: "https://github.com/souvikmajumder26/Multi-Agent-Medical-Assistant", Stars: 930, Forks: 200, Language: "Python", Topics: []string{"medical", "chatbot", "agents"}, UpdatedAt: now},
			{Name: "Health-Care-Chatbot", FullName: "amberkakkar01/Health-Care-Chatbot", Description: "Medical chatbot that answers healthcare FAQs", URL: "https://github.com/amberkakkar01/Health-Care-Chatbot", Stars: 431, Forks: 300, Language: "Python", Topics: []string{"medical", "chatbot"}, UpdatedAt: now},
			{Name: "Llama2-Medical-Chatbot", FullName: "AIAnytime/Llama2-Medical-Chatbot", Description: "Medical bot built with Llama2 and sentence transformers", URL: "https://github.com/AIAnytime/Llama2-Medical-Chatbot", Stars: 352, Forks: 150, Language: "Python", Topics: []string{"medical", "llm", "chatbot"}, UpdatedAt: now},
			{Name: "MedicalGPT", FullName: "shibing624/MedicalGPT", Description: "Train your own medical GPT model", URL: "https://github.com/shibing624/MedicalGPT", Stars: 5600, Forks: 800, Language: "Python", Topics: []string{"medical", "gpt"}, UpdatedAt: now},
		}
	case containsAny(lower, "agent", "coding", "code", "developer"):
		demos = []models.Repository{
			{Name: "open-interpreter", FullName: "OpenInterpreter/open-interpreter", Description: "Natural language interface for computers", URL: "https://github.com/OpenInterpreter/open-interpreter", Stars: 55000, Forks: 4800, Language: "Python", Topics: []string{"agent", "code"}, UpdatedAt: now},
			{Name: "gpt-engineer", FullName: "AntonOsika/gpt-engineer", Description: "Specify what you want, get code", URL: "https://github.com/AntonOsika/gpt-engineer", Stars: 52000, Forks: 6800, Language: "Python", Topics: []string{"code", "agent"}, UpdatedAt: now},
			{Name: "aider", FullName: "Aider-AI/aider", Description: "AI pair programming in your terminal", URL: "https://github.com/Aider-AI/aider", Stars: 30000, Forks: 2800, Language: "Python", Topics: []string{"coding", "agent"}, UpdatedAt: now},
		}
	case containsAny(lower, "vision", "image", "detect", "ocr"):
		demos = []models.Repository{
			{Name: "ultralytics", FullName: "ultralytics/ultralytics", Description: "YOLO vision models for detection and segmentation", URL: "https://github.com/ultralytics/ultralytics", Stars: 35000, Forks: 6800, Language: "Python", Topics: []string{"vision", "yolo"}, UpdatedAt: now},
			{Name: "detectron2", FullName: "facebookresearch/detectron2", Description: "Facebook AI Research's detection platform", URL: "https://github.com/facebookresearch/detectron2", Stars: 30000, Forks: 7500, Language: "Python", Topics: []string{"detection", "vision"}, UpdatedAt: now},
		}
	default:
		demos = []models.Repository{
			{Name: "privateGPT", FullName: "zylon-ai/private-gpt", Description: "Interact privately with your documents using LLMs", URL: "https://github.com/zylon-ai/private-gpt", Stars: 55000, Forks: 7400, Language: "Python", Topics: []string{"rag", "chatbot"}, UpdatedAt: now},
			{Name: "dify", FullName: "langgenius/dify", Description: "Production-ready LLM app development platform", URL: "https://github.com/langgenius/dify", Stars: 80000, Forks: 11000, Language: "TypeScript", Topics: []string{"llm", "rag", "agent"}, UpdatedAt: now},
			{Name: "open-webui", FullName: "open-webui/open-webui", Description: "User-friendly AI chat interface", URL: "https://github.com/open-webui/open-webui", Stars: 80000, Forks: 10000, Language: "JavaScript", Topics: []string{"chatbot", "llm"}, UpdatedAt: now},
		}
	}

	if limit > len(demos) {
		limit = len(demos)
	}
	return demos[:limit]
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
