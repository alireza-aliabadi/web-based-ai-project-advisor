package githubsvc

import (
	"strings"
	"testing"

	"github.com/amin/web-based-ai-project-advisor/internal/models"
)

func TestExtractKeywordsStripsIdeaFillers(t *testing.T) {
	got := extractKeywords("I want to build a medical chatbot")
	if got != "medical chatbot" {
		t.Fatalf("expected 'medical chatbot', got %q", got)
	}
}

func TestBuildQueryPrefersProjectsNotTooling(t *testing.T) {
	q := buildQuery("I want to build a medical chatbot")
	if !strings.Contains(q, "medical chatbot") {
		t.Fatalf("expected medical chatbot keywords in %q", q)
	}
	if strings.Contains(strings.ToLower(q), " rag") || strings.HasSuffix(strings.ToLower(q), "rag") {
		t.Fatalf("should not inject RAG (biases toward frameworks): %q", q)
	}
	if !strings.Contains(q, "-repo:langchain-ai/langchain") {
		t.Fatalf("expected tooling exclusion in %q", q)
	}
	// Must not keep the natural-language filler phrase.
	if strings.Contains(q, "I want to") || strings.Contains(q, "build a") {
		t.Fatalf("fillers should be stripped: %q", q)
	}
}

func TestFilterToolingDropsInfraRepos(t *testing.T) {
	in := []models.Repository{
		{FullName: "langchain-ai/langchain", Name: "langchain"},
		{FullName: "amberkakkar01/Health-Care-Chatbot", Name: "Health-Care-Chatbot"},
		{FullName: "ollama/ollama", Name: "ollama"},
	}
	out := filterTooling(in)
	if len(out) != 1 || out[0].FullName != "amberkakkar01/Health-Care-Chatbot" {
		t.Fatalf("unexpected filter result: %+v", out)
	}
}

func TestFallbackReposAreDomainSimilar(t *testing.T) {
	repos := fallbackRepos("I want to build a medical chatbot", 2)
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	for _, r := range repos {
		if _, tool := toolingRepos[strings.ToLower(r.FullName)]; tool {
			t.Fatalf("fallback returned tooling repo %s", r.FullName)
		}
		blob := strings.ToLower(r.FullName + " " + r.Description + " " + strings.Join(r.Topics, " "))
		if !strings.Contains(blob, "medical") && !strings.Contains(blob, "health") {
			t.Fatalf("expected medical/health similar project, got %s", r.FullName)
		}
	}
}
