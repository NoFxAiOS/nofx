# Repository Guidelines

## Project Structure & Modules
Backend Go code sits at the root: `main.go` wires `manager/`, `market/`, `trader/`, `decision/`, and `pool/` for orchestration and execution. HTTP, auth, and shared helpers live in `api/`, `auth/`, `config/`, `logger/`, and `mcp/`. The React dashboard is in `web/` (`src/components`, `src/pages`, `src/hooks`, `public/`); docs, scripts, and ops assets live under `docs/`, `scripts/`, `docker/`, `nginx/`. Generated artifacts (`logs/`, `decision_logs/`, `web/dist/`, `config.db`) stay out of commits.

## Build & Test Commands
- `go run ./main.go` bootstraps the backend with your local `config.json`.
- `go build -o bin/nofx ./...` and `go test ./...` must succeed before any PR.
- `npm install` in `web/` installs frontend deps; `npm run dev` serves Vite on `localhost:5173`.
- `npm run build` (TypeScript check + Vite build) and `npm run lint` must pass before pushing changes.

## Style & Naming
Run `gofmt` (tabs, camelCase) and wrap errors via `fmt.Errorf("...: %w", err)`. Keep package naming aligned with existing adapters (`trader/binance_futures.go`, `trader/hyperliquid_trader.go`). Frontend code follows ESLint + Prettier (2 spaces, explicit typing); colocate components under `web/src/components/FeatureName`. Avoid suppressing lint rules without reviewer approval. Commit subjects should mirror the repo pattern (`fix(prompts): …`, `feat(trader): …`).

## Issues, Bounties & Support
Use the `.github/ISSUE_TEMPLATE` forms: `bug_report.md` (logs, console output, environment), `feature_request.md` (problem, solution, acceptance criteria), and `bounty_claim.md` (plan, timeline, deliverables). Bug reporters should attach `decision_logs/{trader_id}/` snippets; bounty claimants acknowledge the Code of Conduct and AGPL terms.

## Commit & PR Workflow
Follow Conventional Commits (`type(scope): summary`) and reference issues with `Closes #ID`. Open PRs against `dev`; CODEOWNERS auto-request reviewers for Go, web, config, and docs. Choose the PR template that matches the change (`backend.md`, `frontend.md`, `docs.md`, `general.md`). The template suggester labels the PR and auto-fills content if the body is empty, so update the bilingual sections, attach UI screenshots, and note commands run (`go test`, `npm run build`, etc.).

## CI Automation & Review Gates
`pr-checks.yml` blocks merges if Go fmt/vet/build, frontend build, Trivy, or TruffleHog fail—fix locally before re-requesting review. Advisory workflows (`pr-checks-run.yml` + `pr-checks-comment.yml`) post hints for fork PRs. Size labels and the labeler (`area: backend`, `area: frontend`, `dependencies`, etc.) apply automatically—split oversized diffs.

## Security & Configuration
Never commit API keys or trader secrets; derive local settings from `config.json.example` or environment variables. Keep deploy automation inside `docker/`, `pm2.config.js`, and `nginx/` and document any config migrations in the PR. Scrub order IDs, wallet addresses, and `.db` dumps from shared logs before uploading.
