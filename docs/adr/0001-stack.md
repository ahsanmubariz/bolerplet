# 0001 Pinned stack and version policy
- Status: accepted (2026-10-12)
- Context: BASELINE §1 names the stack but leaves exact versions open. P0 research (R-001 to R-007, R-012, R-013) resolved them on 2026-10-05 and 2026-10-12. CLAUDE.md requires exact pins and an ADR for every dependency.
- Options considered (with pros/cons):
  - Always take the newest release. Pro: newest fixes. Con: brand-new releases have the highest supply-chain risk (compromised maintainer, bad release) and zero field time.
  - **Pin the newest release that is at least 14 days old, unless a newer one fixes a security issue that affects us.** Pro: a short soak catches yanked or bad releases. Con: we lag up to two weeks on non-security fixes.
- Decision:
  - Version policy: exact pins only. Newest release ≥ 14 days old, unless a newer release fixes an advisory reachable from our code. Upgrades go through Renovate/Dependabot PRs with `make check`.
  - **Go modules (direct `require`, 8 of the 12 budget):**

    | Module | Version | Released | Source note |
    |---|---|---|---|
    | `go` directive | `go 1.27.1` (no `toolchain` line) | 2026-09-01 | R-002 |
    | `github.com/gofiber/fiber/v3` | v3.5.0 | 2026-08-13 | R-001, R-003 to R-007 |
    | `github.com/a-h/templ` | v0.3.1020 | 2026-05-10 | R-012 (v0.3.1070 of 2026-10-04 is in soak; no security content) |
    | `github.com/jackc/pgx/v5` | v5.11.0 | 2026-09-07 | OSV: all pgx advisories fixed ≤ 5.9.2; API verified in R-014 (P4) |
    | `github.com/pressly/goose/v3` | v3.28.0 | 2026-09-02 | v3.29.0 (2026-10-11) is in soak; API verified in R-015 (P4) |
    | `github.com/go-playground/validator/v10` | v10.30.5 | 2026-09-20 | OSV: 0 vulns |
    | `golang.org/x/crypto` | v0.57.0 | — | R-001 (lifts fiber's v0.54.0 past GO-2026-6303/6354/6355); v0.58.0 (2026-10-09) is in soak |
    | `github.com/microcosm-cc/bluemonday` | v1.0.27 | 2024-07-04 | OSV: all advisories fixed ≤ 1.0.16 |
    | `github.com/gofiber/storage/postgres/v3` | v3.7.1 | 2026-09-22 | R-006 |

    Transitive `github.com/valyala/fasthttp` stays at fiber's v1.73.0. Overriding to v1.75.0 is a separate ADR after its soak (R-001 option (c)).
  - **Tools:**

    | Tool | Version | Delivery |
    |---|---|---|
    | templ CLI | v0.3.1020 | go.mod `tool` directive (same version as the lib) |
    | staticcheck | v0.8.1 (2026.2.1) | go.mod `tool` directive |
    | govulncheck | v1.8.0 | go.mod `tool` directive (v1.9.0 of 2026-10-11 is in soak) |
    | sqlc | v1.31.1 | release binary into `./bin`, SHA-256 committed in the Makefile |
    | golangci-lint | v2.14.0 | release binary into `./bin`, SHA-256 committed in the Makefile |
    | tailwindcss | v4.3.3 | standalone binary into `./bin`, SHA-256 committed in the Makefile (R-013) |
    | htmx | — | decided in P7 (R-018) |

    The `tool` directive keeps tool versions in `go.sum`. It was exercised in `spikes/d-tools` (templ + staticcheck). Tool modules do not count against the 12-dependency budget, but they do appear in `go.mod`.
- Security impact (OWASP/ASVS refs): A03 Software Supply Chain (exact pins, checksums, soak, govulncheck); A02 (fiber ≥ v3.4.0 for the helmet HSTS fix); A05 (templ `href`/`action` URL sanitization at the pinned version, verified in R-012).
- Consequences / follow-ups:
  - `make vuln` fails on symbol- or package-level findings only. GO-2026-5932 (`x/crypto/openpgp`, no fix, not imported) will stay as a module-level note (R-001). depguard denies `golang.org/x/crypto/openpgp`.
  - Re-evaluate templ v0.3.1070, goose v3.29.0, x/crypto v0.58.0, govulncheck v1.9.0 on or after 2026-10-26.
  - pgx/goose/validator/bluemonday APIs are verified in their phases (R-014, R-015, R-016) before use.
