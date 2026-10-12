# Progress

| Phase | Status | Notes |
|---|---|---|
| P0 Research and decisions | **done** (2026-10-12) | R-001 to R-007, R-012, R-013; ADR 0001 accepted; BASELINE 0.2 applied |
| P1 Skeleton | **done** (2026-10-12) | `make check` green; CI runs on first push to `main` |
| P2 Kernel | not started | |
| P3 Security core | not started | needs R-008 to R-011, R-022 |
| P4 Data | not started | needs R-014, R-015 |
| P5 Views and Tailwind | not started | |
| P6 Auth module | not started | needs R-016 |
| P7 Blog module | not started | needs R-018, R-023 |
| P8 Hardening and DX | not started | needs R-017, R-019 to R-021 |

## P0 log
- 2026-10-05: R-001 (fiber v3.5.0, advisories), R-002 (go 1.27.1), R-003 (Config fields), R-004 (proxy scheme/IP), R-005 (helmet), R-006 (session + Postgres storage), R-007 (csrf).
- 2026-10-12: R-012 (templ render/nonce/URL sanitization), R-013 (Tailwind v4.3.3 standalone), ADR 0001 (pins + 14-day soak policy). Approved; BASELINE 0.2, CLAUDE.md (`c.Context()` rule) and RESEARCH §6 (CVE fix versions) updated.
- Spikes live in `spikes/` (gitignored): `a` (config/proxy/govulncheck), `b` (helmet/session), `c` (csrf), `d` (templ/tailwind, plus early sqlc/golangci-lint downloads), `d-tools` (`tool` directive).

## P1 log
- 2026-10-12: module `github.com/ahsanmubariz/bolerplet`; Makefile (`tools` with committed SHA-256, `generate`, `css`, `lint`, `vuln`, `test`, `check`, `build`, `dev`); `.golangci.yml` (v2: standard + bodyclose, depguard, gosec, gofmt); CI workflow (actions pinned by SHA, `contents: read`, `GOTOOLCHAIN=local`, weekly schedule); `framework/config` with all §6.13 guards; `framework/kernel` `NewApp`/`ListenConfig` with the §5.1 table and a non-leaking stand-in ErrorHandler; `cmd/server` with `/healthz`, `/readyz`, graceful shutdown.
- Tests: `TestProdGuards` (+ allowed cases, all-violations, fail-closed default env, every-env proxy guards, no secret echo), `TestNoServerHeader`, `TestAppConfig`, `TestListenConfig`, `TestErrorHandlerDoesNotLeak`.
- `make vuln` caught go1.27.1 stdlib vulns → moved to go1.27.2 + x/net v0.60.0 (R-002 addendum).
- Temporary deviation: `/healthz` and `/readyz` are registered on the Fiber app in `cmd/server`; P2 moves them behind `kernel.Router` + `policy.Public`.

## Open decisions
- fasthttp override to v1.75.0: separate ADR after soak (R-001).
- `SESSION_KEY`: drop, or keep for HMAC'd storage keys (R-006); needs an ADR either way.
