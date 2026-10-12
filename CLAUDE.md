# CLAUDE.md

Secure-by-default **HMVC web framework inspired by CodeIgniter 3**, built on **Go Fiber v3 + templ + Tailwind CSS v4 (standalone CLI)**, shipped with a reference app.

Before any work, read:
- `docs/BASELINE.md`: architecture, invariants, phase plan (the spec; it wins over your assumptions)
- `docs/RESEARCH.md`: how to verify versions and APIs before coding
- `docs/PROGRESS.md`: current phase and what's done (create it in Phase 0 if missing)

## Golden rules

1. **Verify, don't recall.** Fiber v3 changed many APIs in 2026. Before using any third-party API, confirm the exact signature against the version pinned in `go.mod` (`go doc <pkg>.<Symbol>`, or read the source in the module cache). If docs and your memory disagree, the pinned source wins. Anything marked ⟨verify⟩ in BASELINE.md must be resolved and recorded in `docs/research/` before it is implemented.
2. **Spec first.** If a task conflicts with `docs/BASELINE.md`, stop and say so. Don't silently deviate. Propose a change as an ADR in `docs/adr/`.
3. **Small, tested steps.** Each change comes with tests. `make check` must be green before you report a step as done.
4. **No new dependency** without an ADR (why, alternatives, maintainer health, `govulncheck` result). Prefer the standard library.

## Security invariants (never violate; if a task seems to require it, stop and ask)

- Every route is registered through `kernel.Router` with an explicit `policy.Policy`. Never call `fiber.App`/`fiber.Router` route methods from modules. No auto-routing.
- SQL only through sqlc-generated queries with placeholders. No string concatenation or `fmt.Sprintf` into SQL. Dynamic sort/filter columns go through an allowlist.
- HTML only through templ components. Never `templ.Raw`, `templ.SafeURL`, or an equivalent on user-controlled data. Rich text goes through the bluemonday policy in `framework/pkg/sanitize`.
- Never send `err.Error()`, stack traces, SQL errors, or internal paths to clients. Errors go to the central ErrorHandler.
- Never call `c.AutoFormat()`. The response format is decided by the route group, not by the `Accept` header.
- Security randomness comes from `crypto/rand` only. Passwords use argon2id via `framework/security/password` only.
- Cookies: `__Host-` prefix, `Secure`, `HttpOnly`, `SameSite=Lax` (or Strict), `Path=/`, no Domain.
- Regenerate the session ID on login, logout, and privilege change.
- No inline `<script>` without the per-request CSP nonce. No inline `style=` attributes. No `on*=` handler attributes. No third-party CDN assets.
- Outbound HTTP only through `httpx.SafeClient`. Filesystem access to user-influenced paths only through `os.Root`.
- Object-level authorization lives in the query (`WHERE ... AND owner_id = $n`). Not-owned resources return 404, not 403.
- Modules import only `framework/...` and other modules' `contract` subpackages, never another module's internals.
- Never log passwords, tokens, session IDs, cookies, `Authorization` headers, or CSRF tokens.

## Commands (Makefile; create in Phase 1)

```
make dev        # templ watch + tailwind watch + air/go run
make generate   # templ generate && sqlc generate
make css        # tailwind standalone build (pinned binary, sha256-verified)
make test       # go test -race -cover ./...
make lint       # gofmt check, go vet, staticcheck, golangci-lint
make vuln       # govulncheck ./...
make check      # generate (diff must be clean) + lint + vuln + test
make build      # CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"
```

## Conventions

- Go: `gofmt`, small packages, explicit wiring in `cmd/server/main.go` (no `init()` registration magic), pass `c.Context()` (not `c`) wrapped in a timeout to DB and other cancellation-aware calls; never let `c` escape the handler.
- Handlers are thin: bind → validate → service → render. Business rules live in `service.go`.
- Errors: wrap with `%w`; return `httpx.AppError` for user-facing messages.
- Logs: `log/slog` JSON, always include `request_id`; security events use `event=<domain>.<action>`.
- Tests: table-driven; HTTP tests via `app.Test`; integration tests use a real Postgres (CI service container).
- Commit after each green step with a conventional message (`feat(kernel): ...`, `sec(csrf): ...`).

## Workflow per phase

Read the phase in BASELINE.md §10 → resolve its research items (RESEARCH.md §5) into `docs/research/` → present a short plan → implement → write the acceptance tests listed for that phase → `make check` → update `docs/PROGRESS.md` → commit.
