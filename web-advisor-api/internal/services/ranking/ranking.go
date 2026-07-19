package ranking

import (
	"math"
	"strings"
	"time"

	"github.com/amin/web-based-ai-project-advisor/internal/models"
	"github.com/amin/web-based-ai-project-advisor/internal/services/llm"
)

type Ranker struct{}

func New() *Ranker { return &Ranker{} }

func (r *Ranker) RankRepos(queryVec []float32, repos []models.Repository, embedFn func(string) []float32) []models.Repository {
	out := make([]models.Repository, len(repos))
	copy(out, repos)
	now := time.Now()
	for i := range out {
		text := out[i].FullName + " " + out[i].Description + " " + strings.Join(out[i].Topics, " ")
		sem := llm.Cosine(queryVec, embedFn(text))
		pop := math.Log10(float64(out[i].Stars)+1) / 6.0
		activity := 0.3
		if t, err := time.Parse(time.RFC3339, out[i].UpdatedAt); err == nil {
			days := now.Sub(t).Hours() / 24
			activity = math.Max(0, 1.0-days/365.0)
		}
		quality := 0.2
		if out[i].Description != "" {
			quality += 0.2
		}
		if len(out[i].Topics) > 0 {
			quality += 0.2
		}
		if out[i].Language != "" {
			quality += 0.1
		}
		out[i].Score = 0.45*sem + 0.25*pop + 0.15*activity + 0.15*math.Min(quality, 1)
		out[i].Why = explainRepo(out[i], sem, pop)
	}
	sortRepos(out)
	return out
}

func (r *Ranker) RankModels(queryVec []float32, modelsIn []models.Model, embedFn func(string) []float32) []models.Model {
	out := make([]models.Model, len(modelsIn))
	copy(out, modelsIn)
	for i := range out {
		text := out[i].Name + " " + out[i].Task + " " + out[i].Architecture + " " + out[i].Framework
		sem := llm.Cosine(queryVec, embedFn(text))
		downloads := math.Log10(float64(out[i].Downloads)+1) / 7.0
		likes := math.Log10(float64(out[i].Likes)+1) / 5.0
		out[i].Score = 0.5*sem + 0.3*downloads + 0.2*likes
		out[i].Why = explainModel(out[i], sem)
	}
	sortModels(out)
	return out
}

func explainRepo(r models.Repository, sem, pop float64) string {
	parts := []string{}
	if sem > 0.4 {
		parts = append(parts, "strong semantic match to your idea")
	}
	if r.Stars > 1000 {
		parts = append(parts, "high community adoption")
	}
	if r.Language != "" {
		parts = append(parts, "written in "+r.Language)
	}
	if len(parts) == 0 {
		return "relevant open-source reference"
	}
	_ = pop
	return strings.Join(parts, "; ")
}

func explainModel(m models.Model, sem float64) string {
	parts := []string{}
	if m.Task != "" {
		parts = append(parts, "task: "+m.Task)
	}
	if m.Downloads > 100000 {
		parts = append(parts, "widely downloaded")
	}
	if sem > 0.35 {
		parts = append(parts, "fits extracted requirements")
	}
	if len(parts) == 0 {
		return "candidate model for your use case"
	}
	return strings.Join(parts, "; ")
}

func sortRepos(repos []models.Repository) {
	for i := 0; i < len(repos); i++ {
		for j := i + 1; j < len(repos); j++ {
			if repos[j].Score > repos[i].Score {
				repos[i], repos[j] = repos[j], repos[i]
			}
		}
	}
}

func sortModels(models []models.Model) {
	for i := 0; i < len(models); i++ {
		for j := i + 1; j < len(models); j++ {
			if models[j].Score > models[i].Score {
				models[i], models[j] = models[j], models[i]
			}
		}
	}
}
