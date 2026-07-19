package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/amin/web-based-ai-project-advisor/internal/models"
	"github.com/amin/web-based-ai-project-advisor/internal/services/architecture"
	"github.com/amin/web-based-ai-project-advisor/internal/services/embedding"
	githubsvc "github.com/amin/web-based-ai-project-advisor/internal/services/githubsvc"
	"github.com/amin/web-based-ai-project-advisor/internal/services/huggingface"
	"github.com/amin/web-based-ai-project-advisor/internal/services/llm"
	"github.com/amin/web-based-ai-project-advisor/internal/services/ranking"
	"github.com/amin/web-based-ai-project-advisor/internal/services/requirements"
	"github.com/amin/web-based-ai-project-advisor/internal/services/search"
)

type Service struct {
	reqs   *requirements.Extractor
	gh     *githubsvc.Service
	hf     *huggingface.Service
	web    *search.Service
	embed  *embedding.Service
	ranker *ranking.Ranker
	llm    *llm.Client
}

func New(
	reqs *requirements.Extractor,
	gh *githubsvc.Service,
	hf *huggingface.Service,
	web *search.Service,
	embed *embedding.Service,
	ranker *ranking.Ranker,
	llmClient *llm.Client,
) *Service {
	return &Service{reqs: reqs, gh: gh, hf: hf, web: web, embed: embed, ranker: ranker, llm: llmClient}
}

func (s *Service) Recommend(ctx context.Context, idea string, prefs *models.AnalyzeRequest) (*models.Recommendation, error) {
	req, err := s.reqs.Extract(ctx, idea, prefs)
	if err != nil {
		return nil, err
	}

	var (
		repos   []models.Repository
		mods    []models.Model
		webHits []models.SearchResult
		wg      sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		repos, _ = s.gh.Search(ctx, req.Query, 10)
	}()
	go func() {
		defer wg.Done()
		mods, _ = s.hf.Search(ctx, req.Query+" "+req.Task, req.Task, 10)
	}()
	go func() {
		defer wg.Done()
		webHits, _ = s.web.Search(ctx, req.Query, 5)
	}()
	wg.Wait()

	vecs, _ := s.embed.Embed(ctx, []string{req.Query + " " + req.Domain + " " + req.Task})
	queryVec := vecs[0]
	embedOne := func(text string) []float32 {
		v, _ := s.embed.Embed(ctx, []string{text})
		return v[0]
	}

	repos = s.ranker.RankRepos(queryVec, repos, embedOne)
	mods = s.ranker.RankModels(queryVec, mods, embedOne)
	if len(repos) > 5 {
		repos = repos[:5]
	}
	if len(mods) > 5 {
		mods = mods[:5]
	}

	// Best-effort vector store upsert for semantic memory
	_ = s.embed.EnsureCollection(ctx, len(queryVec))
	_ = s.embed.Upsert(ctx, hashID(idea), queryVec, map[string]any{
		"idea": idea, "domain": req.Domain, "task": req.Task,
	})

	arch, mermaid, stack, roadmap, deploy, hardware, cost := architecture.Generate(req)

	rec := &models.Recommendation{
		Requirements:   req,
		Repositories:   repos,
		Models:         mods,
		SearchResults:  webHits,
		Architecture:   arch,
		MermaidDiagram: mermaid,
		TechStack:      stack,
		Roadmap:        roadmap,
		Deployment:     deploy,
		Hardware:       hardware,
		CostEstimate:   cost,
		Summary:        fmt.Sprintf("Plan for a %s %s system (%s modality).", req.Domain, req.Task, req.Modality),
	}

	if s.llm != nil && s.llm.Available() {
		s.enrichWithLLM(ctx, idea, rec)
	} else {
		rec.Summary = s.localSummary(rec)
	}
	return rec, nil
}

func (s *Service) enrichWithLLM(ctx context.Context, idea string, rec *models.Recommendation) {
	contextBlob, _ := json.Marshal(map[string]any{
		"idea":         idea,
		"requirements": rec.Requirements,
		"repositories": summarizeRepos(rec.Repositories),
		"models":       summarizeModels(rec.Models),
		"search":       rec.SearchResults,
	})
	var enriched struct {
		Summary        string   `json:"summary"`
		Architecture   string   `json:"architecture"`
		MermaidDiagram string   `json:"mermaid_diagram"`
		TechStack      []string `json:"tech_stack"`
		Roadmap        []string `json:"roadmap"`
		Deployment     string   `json:"deployment"`
		Hardware       string   `json:"hardware"`
		CostEstimate   string   `json:"cost_estimate"`
		RepoWhy        []string `json:"repo_why"`
		ModelWhy       []string `json:"model_why"`
	}
	err := s.llm.ChatJSON(ctx,
		`You are an AI solution architect. Given context JSON, produce a practical recommendation.
Return JSON keys: summary, architecture, mermaid_diagram, tech_stack, roadmap, deployment, hardware, cost_estimate, repo_why, model_why.
Keep advice concrete and implementation-oriented.`,
		string(contextBlob),
		&enriched,
	)
	if err != nil {
		rec.Summary = s.localSummary(rec)
		return
	}
	if enriched.Summary != "" {
		rec.Summary = enriched.Summary
	}
	if enriched.Architecture != "" {
		rec.Architecture = enriched.Architecture
	}
	if enriched.MermaidDiagram != "" {
		rec.MermaidDiagram = enriched.MermaidDiagram
	}
	if len(enriched.TechStack) > 0 {
		rec.TechStack = enriched.TechStack
	}
	if len(enriched.Roadmap) > 0 {
		rec.Roadmap = enriched.Roadmap
	}
	if enriched.Deployment != "" {
		rec.Deployment = enriched.Deployment
	}
	if enriched.Hardware != "" {
		rec.Hardware = enriched.Hardware
	}
	if enriched.CostEstimate != "" {
		rec.CostEstimate = enriched.CostEstimate
	}
	for i := range rec.Repositories {
		if i < len(enriched.RepoWhy) && enriched.RepoWhy[i] != "" {
			rec.Repositories[i].Why = enriched.RepoWhy[i]
		}
	}
	for i := range rec.Models {
		if i < len(enriched.ModelWhy) && enriched.ModelWhy[i] != "" {
			rec.Models[i].Why = enriched.ModelWhy[i]
		}
	}
}

func (s *Service) localSummary(rec *models.Recommendation) string {
	var b strings.Builder
	b.WriteString(rec.Summary)
	b.WriteString("\n\n## Recommended repositories\n")
	for _, r := range rec.Repositories {
		b.WriteString(fmt.Sprintf("- [%s](%s) ⭐ %d — %s\n", r.FullName, r.URL, r.Stars, r.Why))
	}
	b.WriteString("\n## Recommended models\n")
	for _, m := range rec.Models {
		b.WriteString(fmt.Sprintf("- [%s](%s) — %s (%s)\n", m.Name, m.URL, m.Task, m.Why))
	}
	b.WriteString("\n## Architecture\n")
	b.WriteString(rec.Architecture)
	b.WriteString("\n\n```mermaid\n")
	b.WriteString(rec.MermaidDiagram)
	b.WriteString("```\n\n## Tech stack\n")
	for _, t := range rec.TechStack {
		b.WriteString("- " + t + "\n")
	}
	b.WriteString("\n## Roadmap\n")
	for _, step := range rec.Roadmap {
		b.WriteString("1. " + step + "\n")
	}
	b.WriteString("\n## Deployment\n")
	b.WriteString(rec.Deployment)
	b.WriteString("\n\n## Hardware\n")
	b.WriteString(rec.Hardware)
	b.WriteString("\n\n## Cost estimate\n")
	b.WriteString(rec.CostEstimate)
	return b.String()
}

func (s *Service) FormatMarkdown(rec *models.Recommendation) string {
	return s.localSummary(rec)
}

func summarizeRepos(repos []models.Repository) []map[string]any {
	out := make([]map[string]any, 0, len(repos))
	for _, r := range repos {
		out = append(out, map[string]any{
			"full_name": r.FullName, "stars": r.Stars, "language": r.Language,
			"description": r.Description, "url": r.URL, "why": r.Why,
		})
	}
	return out
}

func summarizeModels(mods []models.Model) []map[string]any {
	out := make([]map[string]any, 0, len(mods))
	for _, m := range mods {
		out = append(out, map[string]any{
			"name": m.Name, "task": m.Task, "downloads": m.Downloads,
			"likes": m.Likes, "url": m.URL, "hardware": m.Hardware, "why": m.Why,
		})
	}
	return out
}

func hashID(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
