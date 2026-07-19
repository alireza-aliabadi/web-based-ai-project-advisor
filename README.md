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

## Tech Stack

**Backend:** Go, Fiber, PostgreSQL, Redis, Qdrant  
**AI:** OpenAI-compatible LLM + embeddings  
**External:** GitHub, Hugging Face, Tavily  
**Ops:** Docker Compose, Prometheus, Grafana

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md). Track milestones in [PROGRESS.md](PROGRESS.md).

## License

[MIT](LICENSE)
