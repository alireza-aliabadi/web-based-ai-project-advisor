package requirements

import (
	"context"
	"strings"

	"github.com/amin/web-based-ai-project-advisor/internal/models"
	"github.com/amin/web-based-ai-project-advisor/internal/services/llm"
)

type Extractor struct {
	llm *llm.Client
}

func New(client *llm.Client) *Extractor {
	return &Extractor{llm: client}
}

func (e *Extractor) Extract(ctx context.Context, idea string, prefs *models.AnalyzeRequest) (models.Requirements, error) {
	idea = strings.TrimSpace(idea)
	req := heuristicExtract(idea)

	if prefs != nil {
		if prefs.SkillLevel != "" {
			req.SkillLevel = prefs.SkillLevel
		}
		if len(prefs.PreferredLangs) > 0 {
			req.Languages = prefs.PreferredLangs
		}
		if prefs.HardwareLimit != "" {
			req.Hardware = prefs.HardwareLimit
		}
	}

	if e.llm != nil && e.llm.Available() {
		var out models.Requirements
		err := e.llm.ChatJSON(ctx,
			`You extract structured AI project requirements.
Return JSON with keys: domain, task, modality, requirements (string array), query.`,
			"Idea: "+idea,
			&out,
		)
		if err == nil && out.Domain != "" {
			out.Query = idea
			out.SkillLevel = req.SkillLevel
			out.Languages = req.Languages
			out.Hardware = req.Hardware
			if len(out.Requirements) == 0 {
				out.Requirements = req.Requirements
			}
			return out, nil
		}
	}
	return req, nil
}

func heuristicExtract(idea string) models.Requirements {
	lower := strings.ToLower(idea)
	req := models.Requirements{
		Domain:       "general",
		Task:         "text generation",
		Modality:     "text",
		Requirements: []string{"LLM"},
		Query:        idea,
		SkillLevel:   "intermediate",
	}

	domainRules := []struct {
		keys   []string
		domain string
	}{
		{[]string{"medical", "health", "clinic", "patient"}, "healthcare"},
		{[]string{"legal", "law", "contract"}, "legal"},
		{[]string{"finance", "bank", "trading", "fintech"}, "finance"},
		{[]string{"edu", "tutor", "learn", "student"}, "education"},
		{[]string{"code", "coding", "developer", "software"}, "software engineering"},
		{[]string{"retail", "shop", "ecommerce", "commerce"}, "retail"},
	}
	for _, r := range domainRules {
		for _, k := range r.keys {
			if strings.Contains(lower, k) {
				req.Domain = r.domain
				break
			}
		}
	}

	switch {
	case containsAny(lower, "chat", "bot", "assistant", "qa", "question"):
		req.Task = "question answering"
		req.Requirements = append(req.Requirements, "RAG", "vector database")
	case containsAny(lower, "agent", "autonomous", "tool use"):
		req.Task = "agentic workflow"
		req.Requirements = append(req.Requirements, "tool calling", "memory", "orchestration")
	case containsAny(lower, "vision", "image", "ocr", "detect"):
		req.Task = "computer vision"
		req.Modality = "image"
		req.Requirements = append(req.Requirements, "vision model")
	case containsAny(lower, "speech", "voice", "audio", "transcri"):
		req.Task = "speech processing"
		req.Modality = "audio"
		req.Requirements = append(req.Requirements, "ASR")
	case containsAny(lower, "recommend", "personaliz"):
		req.Task = "recommendation"
		req.Requirements = append(req.Requirements, "embeddings", "ranking")
	}

	req.Requirements = unique(req.Requirements)
	return req
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func unique(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
