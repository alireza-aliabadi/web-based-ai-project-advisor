package architecture

import (
	"fmt"
	"strings"

	"github.com/amin/web-based-ai-project-advisor/internal/models"
)

func Generate(req models.Requirements) (architecture, mermaid string, stack, roadmap []string, deployment, hardware, cost string) {
	needsRAG := contains(req.Requirements, "RAG") || contains(req.Requirements, "vector database")
	needsAgent := contains(req.Requirements, "tool calling") || strings.Contains(strings.ToLower(req.Task), "agent")

	stack = []string{"Python", "FastAPI", "PostgreSQL", "Docker"}
	if needsRAG {
		stack = append(stack, "Qdrant / pgvector", "Embedding model", "LLM API or local runtime")
	}
	if needsAgent {
		stack = append(stack, "LangGraph / LlamaIndex Workflows")
	}
	if req.Modality == "image" {
		stack = append(stack, "Vision model (HF Transformers)")
	}
	if req.Modality == "audio" {
		stack = append(stack, "Whisper / ASR pipeline")
	}

	architecture = fmt.Sprintf(
		`System for %s (%s / %s):

1. Client — web or API consumers submit requests.
2. API Gateway — auth, rate limits, request routing.
3. Orchestrator — requirement-aware workflow (%s).
4. Model layer — LLM / task models from Hugging Face or managed APIs.
5. Knowledge layer — %s.
6. Persistence — PostgreSQL for sessions and audit; Redis for cache.
7. Observability — logs, metrics, traces.`,
		req.Domain, req.Task, req.Modality, req.Task,
		ternary(needsRAG, "vector store + document ingest + retrieval", "application database and prompt context"),
	)

	mermaid = buildMermaid(needsRAG, needsAgent)

	roadmap = []string{
		"Phase 1: Clarify requirements, data sources, and success metrics",
		"Phase 2: Prototype model + minimal API with evaluation set",
		"Phase 3: Add retrieval/tools, harden prompts, measure quality",
		"Phase 4: Productionize auth, caching, monitoring, and deployment",
		"Phase 5: Iterate from user feedback and cost/latency budgets",
	}

	deployment = "Docker Compose for MVP; migrate to Kubernetes or a managed container platform when traffic grows. Prefer managed LLM APIs early; add self-hosted inference (vLLM/Ollama) when cost or privacy requires it."

	hardware = req.Hardware
	if hardware == "" {
		if needsAgent || strings.Contains(req.Task, "generation") || strings.Contains(req.Task, "question") {
			hardware = "Dev: 16 GB RAM laptop + API credits. Prod inference: GPU with 16–24 GB VRAM or managed endpoints."
		} else {
			hardware = "CPU-friendly for embeddings/classification; GPU optional for throughput."
		}
	}

	cost = "MVP: $20–200/mo (API + small VPS). Scale: dominated by LLM tokens and GPU inference; cache aggressively and use smaller models for routing."
	return
}

func buildMermaid(rag, agent bool) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	b.WriteString("  U[User] --> FE[Web App]\n")
	b.WriteString("  FE --> API[API Gateway]\n")
	b.WriteString("  API --> ORCH[Orchestrator]\n")
	if agent {
		b.WriteString("  ORCH --> TOOLS[Tools / Agents]\n")
		b.WriteString("  TOOLS --> LLM[LLM]\n")
	} else {
		b.WriteString("  ORCH --> LLM[LLM]\n")
	}
	if rag {
		b.WriteString("  ORCH --> RET[Retriever]\n")
		b.WriteString("  RET --> VDB[(Vector DB)]\n")
	}
	b.WriteString("  ORCH --> DB[(PostgreSQL)]\n")
	b.WriteString("  ORCH --> CACHE[(Redis)]\n")
	return b.String()
}

func contains(xs []string, want string) bool {
	want = strings.ToLower(want)
	for _, x := range xs {
		if strings.EqualFold(x, want) || strings.Contains(strings.ToLower(x), want) {
			return true
		}
	}
	return false
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
