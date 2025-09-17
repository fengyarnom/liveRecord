# Repository Guidelines

## Project Structure & Module Organization
- Current minimal layout: `main.go`, `go.mod`.
- Target layout as the project grows:
  - `cmd/blog/main.go` – app entrypoint
  - `internal/` – app code grouped by package
    - `config/`, `db/`, `model/`, `repo/`, `service/`, `web/handler/`, `web/middleware/`
    - `web/templates/` (Go `html/template`), `web/static/` (CSS/JS/assets)
  - `migrations/` – SQL migrations for PostgreSQL

## Build, Test, and Development Commands
- Run locally: `go run .` (or `go run ./cmd/blog` after moving entrypoint)
- Build binary: `go build -o bin/blog .`
- Run tests: `go test ./...`
- Vet static checks: `go vet ./...`
- Format code: `gofmt -s -w .`

## Coding Style & Naming Conventions
- Use `gofmt` formatting; 1 tab indentation (default Go style).
- Package names are lower_snake, short, and meaningful (e.g., `repo`, `service`).
- Exported identifiers: `CamelCase`; unexported: `camelCase`.
- File naming:
  - HTTP handlers in `internal/web/handler/` with feature-based files (e.g., `post.go`).
  - Templates use `.tmpl` and group under `layout/`, `partials/`, `pages/`.
  - SQL migrations use incremental prefixes (e.g., `0001_init.sql`).

## Testing Guidelines
- Framework: standard `testing` package.
- Test files: `*_test.go`; table-driven tests preferred for repos/services.
- Coverage: `go test ./... -cover -coverprofile=cover.out` then `go tool cover -html=cover.out`.
- Keep tests deterministic; use in-memory fakes or controlled fixtures for DB where possible.

## Commit & Pull Request Guidelines
- Commits are small and focused with imperative mood (e.g., "add repo for posts").
- Recommended prefixes: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`.
- PRs include:
  - Purpose and scope, linked issue if any
  - How to run/test locally (commands)
  - DB migration notes (`migrations/`) and rollback considerations
  - Screenshots for UI-impacting changes (templates/ CSS)

## Security & Configuration Tips
- Config via env vars: `DATABASE_URL`, `PORT`, `GIN_MODE`.
- Do not commit secrets; use `.env` only for local dev and add to `.gitignore`.
- Validate inputs in handlers; escape outputs via templates; sanitize rendered Markdown.

## 规范 (Norms)
- Be ashamed of guessing interfaces; be proud of carefully consulting documentation.
- Be ashamed of ambiguous execution; be proud of seeking confirmation.
- Be ashamed of speculating about business; be proud of confirming with stakeholders.
- Be ashamed of inventing interfaces; be proud of reusing existing ones.
- Be ashamed of skipping validation; be proud of proactive testing.
- Be ashamed of breaking the architecture; be proud of following conventions.
- Be ashamed of pretending to understand; be proud of being honest about not knowing.
- Be ashamed of blind edits; be proud of cautious refactoring.
