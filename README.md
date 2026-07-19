# AI Solution Architect

API service that turns application ideas into actionable AI project plans.

Describe an idea — get recommended GitHub repositories, Hugging Face models, architecture, tech stack, implementation roadmap, deployment strategy, and hardware requirements.

## Features

- Conversation and one-shot analysis APIs
- GitHub repository discovery and ranking
- Hugging Face model recommendations
- Web search for recent frameworks and papers
- Requirement extraction and semantic ranking
- Architecture diagrams (Mermaid)
- JWT auth and API key support

## Quick Start

```bash
cp .env.example .env
# Add LLM_API_KEY and optional GITHUB_TOKEN / HUGGINGFACE_TOKEN / TAVILY_API_KEY

docker compose up -d postgres redis qdrant
make api        # http://localhost:8080
```

Or run the full API stack:

```bash
docker compose up -d --build
```

## Architecture

```
Client → API Gateway
            ├─ GitHub Service
            ├─ Hugging Face Service
            ├─ Web Search Service
            └─ Context Builder → Reranker → LLM Agent → Recommendations
```

## API Overview

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/api/v1/auth/register` | Register |
| POST | `/api/v1/auth/login` | Login |
| GET | `/api/v1/conversations` | List conversations |
| POST | `/api/v1/conversations` | Create conversation |
| POST | `/api/v1/conversations/:id/messages` | Send message (recommend) |
| POST | `/api/v1/analyze` | One-shot project analysis |

### Example

```bash
curl -s -X POST http://localhost:8080/api/v1/analyze \
  -H 'Content-Type: application/json' \
  -d '{"idea":"I want to build a medical chatbot"}'
```

### Response shape (`POST /api/v1/analyze`)

```json
{
  "requirements": {
    "domain": "string",
    "task": "string",
    "modality": "string",
    "requirements": ["string"],
    "query": "string",
    "skill_level": "string",
    "languages": ["string"],
    "hardware": "string"
  },
  "repositories": [
    {
      "name": "string",
      "full_name": "owner/repo",
      "description": "string",
      "url": "https://github.com/...",
      "stars": 0,
      "forks": 0,
      "language": "string",
      "topics": ["string"],
      "last_update": "RFC3339 timestamp",
      "score": 0.0,
      "why": "string"
    }
  ],
  "models": [
    {
      "name": "org/model",
      "task": "string",
      "downloads": 0,
      "likes": 0,
      "architecture": "string",
      "license": "string",
      "framework": "string",
      "url": "https://huggingface.co/...",
      "score": 0.0,
      "hardware": "string",
      "why": "string"
    }
  ],
  "search_results": [
    {
      "title": "string",
      "url": "string",
      "snippet": "string",
      "score": 0.0
    }
  ],
  "architecture": "string",
  "mermaid_diagram": "string",
  "tech_stack": ["string"],
  "roadmap": ["string"],
  "deployment": "string",
  "hardware": "string",
  "cost_estimate": "string",
  "summary": "string"
}
```

`repositories` is capped at the top 2 similar open-source projects (not infra/tooling libraries). `models` returns up to 5 Hugging Face recommendations.

Full sample payload: [examples/medical/response.json](examples/medical/response.json).

## Tech Stack

**Backend:** Go, Fiber, PostgreSQL, Redis, Qdrant  
**AI:** OpenAI-compatible LLM + embeddings  
**External:** GitHub, Hugging Face, Tavily  
**Ops:** Docker Compose (Prometheus/Grafana run on a separate observability host)

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md). Track milestones in [PROGRESS.md](PROGRESS.md).

## License

[MIT](LICENSE)
