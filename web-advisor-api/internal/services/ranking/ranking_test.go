package ranking_test

import (
	"testing"

	"github.com/amin/web-based-ai-project-advisor/internal/models"
	"github.com/amin/web-based-ai-project-advisor/internal/services/llm"
	"github.com/amin/web-based-ai-project-advisor/internal/services/ranking"
)

func TestRankReposOrdersByScore(t *testing.T) {
	ranker := ranking.New()
	queryVec := []float32{1, 0, 0, 0}
	embedFn := func(text string) []float32 {
		_ = text
		return []float32{0.9, 0.1, 0, 0}
	}
	repos := []models.Repository{
		{FullName: "a/low", Stars: 10, Description: "x"},
		{FullName: "b/high", Stars: 50000, Description: "popular project", Topics: []string{"llm"}, Language: "Python", UpdatedAt: "2025-01-01T00:00:00Z"},
	}
	out := ranker.RankRepos(queryVec, repos, embedFn)
	if out[0].FullName != "b/high" {
		t.Fatalf("expected high-star repo first, got %s score=%f vs %f", out[0].FullName, out[0].Score, out[1].Score)
	}
	_ = llm.Cosine
}
