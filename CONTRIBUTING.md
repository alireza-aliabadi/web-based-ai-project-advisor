# Contributing to AI Solution Architect

Thank you for your interest in contributing!

## Development Setup

1. Clone the repository
2. Copy `.env.example` to `.env` and fill in API keys
3. Start infrastructure: `docker compose up -d postgres redis qdrant`
4. Run the API: `cd web-advisor-api && go run ./cmd/server`

## Branch Workflow

- `main` — stable releases
- `develop` — integration branch
- Feature branches: `feature/<short-description>`
- Bugfix: `fix/<short-description>`

Open PRs against `develop`. Include a clear description and test plan.

## Code Style

### Go

- Follow standard Go formatting (`gofmt`, `go vet`)
- Use meaningful package names under `internal/`
- Prefer table-driven tests

## Commit Messages

Use concise, imperative messages:

```
add github repository search service
fix jwt expiry validation
```

## Pull Requests

- Keep PRs focused and reviewable
- Update `PROGRESS.md` if you complete a milestone
- Ensure CI passes before requesting review
