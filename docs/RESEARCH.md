# Research Protocol

This file tells **Claude Code** (and you) how to turn every ⟨verify⟩ in `BASELINE.md` into a checked, sourced decision before code is written, and how to keep those decisions current.

---

## 1. Why this exists

Fiber v3 went stable in February 2026 and changed many APIs (Ctx is an interface, middleware data moved to `FromContext` helpers, session/CSRF/limiter configs changed, the extractors package was added). Several advisories hit core middleware during 2026 (§6). Model memory and blog posts lag behind this. **The pinned source code is the truth.**

## 2. Source hierarchy (highest wins)

1. **Source at the pinned version**: `go doc`, or the code in `$(go env GOMODCACHE)`.
2. **Official docs for that version**: docs.gofiber.io (pick the v3.x docs, not "Next" unless intentional), templ.guide, docs.sqlc.dev, tailwindcss.com.
3. **Release notes and security advisories**: GitHub releases, GHSA, vuln.go.dev / OSV.
4. **OWASP primary documents**: Top 10:2025, ASVS 5.0, Cheat Sheet Series.
5. **Blogs, Q&A, AI answers**: leads only, never final evidence.

If 1 and 2 disagree, trust 1 and note the doc bug.

## 3. Protocol per research item

1. **Question:** copy the R-item from §5.
2. **Locate:** check the pinned version (`go list -m <module>`), then read the source/docs from §2.
3. **Spike:** write the smallest test that proves the behavior (`spikes/` is gitignored, or go straight to a real test).
4. **Record:** `docs/research/R-xxx-<slug>.md` using the template in §7.
5. **Link:** the BASELINE ⟨verify⟩ is now resolved. If the finding changes the spec, write an ADR and update BASELINE.md in the same commit.
6. **Lock in:** the behavior must be covered by a permanent test, so an upgrade that changes it fails CI.

## 4. Toolbox

```bash
# versions
go list -m -versions github.com/gofiber/fiber/v3          # all published versions
go list -m -u all                                          # available upgrades
go version -m ./bin/server                                 # what is actually in the binary

# APIs at the pinned version
go doc -all github.com/gofiber/fiber/v3 | less
go doc github.com/gofiber/fiber/v3.Config
go doc -all github.com/gofiber/fiber/v3/middleware/csrf
go doc -all github.com/gofiber/fiber/v3/middleware/session
go doc -all github.com/gofiber/fiber/v3/middleware/helmet
go doc -all github.com/gofiber/fiber/v3/middleware/limiter
go doc -all github.com/gofiber/fiber/v3/extractors
cd "$(go env GOMODCACHE)/github.com/gofiber/fiber/v3@<ver>" && grep -rn "func New" middleware/csrf

# vulnerabilities
govulncheck ./...                                          # reachable vulns in code + stdlib
curl -s https://api.osv.dev/v1/query \
  -d '{"package":{"name":"github.com/gofiber/fiber/v3","ecosystem":"Go"}}' | jq '.vulns[] | {id, summary, aliases}'
gh api repos/gofiber/fiber/security-advisories --jq '.[] | {ghsa_id, summary, severity}'

# tools
templ version; sqlc version; ./bin/tailwindcss --help
```

## 5. Research backlog

Each item names the question, where to look, and what to produce. P0 must resolve R-001 to R-007, R-012, and R-013. The rest are resolved in their phase.

### Platform
- **R-001 Fiber version and advisories.** Latest v3 release; every advisory affecting it (GHSA/OSV/govulncheck). *Output:* pinned version plus a list of advisories checked. Must be ≥ v3.5.0.
- **R-002 Go toolchain.** Latest stable Go release and patch; any stdlib vulns (`govulncheck`). *Source:* go.dev/doc/devel/release. *Output:* `go`/`toolchain` lines.
- **R-003 `fiber.Config` and `ListenConfig` fields.** Exact names/defaults for: ServerHeader, BodyLimit, Read/Write/IdleTimeout, Immutable, ErrorHandler, TrustProxy, TrustProxyConfig, ProxyHeader, EnableIPValidation, SkipUnmatchedRoutes, PassLocalsToContext, DisableStartupMessage, TLSMinVersion. *Output:* the table in BASELINE §5.1, confirmed or corrected.
- **R-004 Scheme and IP behind a proxy.** Does `c.Scheme()` honor `X-Forwarded-Proto` only from trusted proxies? How does `c.IP()` behave with `EnableIPValidation`? This decides HSTS emission and the limiter key. *Output:* a test with a trusted and an untrusted proxy. Source: the Fiber reverse-proxy guide plus `ctx.go`.

### Security middleware
- **R-005 helmet.** Config fields and defaults at the pinned version; confirm the HSTS fix (CVE-2026-53624); how to suppress the CSP header so the custom csp middleware owns it; PermissionPolicy field name.
- **R-006 session.** `New` vs `NewStore` vs `NewWithStore`; how a handler gets the session (`session.FromContext`?); Regenerate/Destroy/Reset/Release semantics and when Release is required; IdleTimeout/AbsoluteTimeout; cookie option names; `__Host-` compatibility; a Postgres storage adapter that implements the v3 Storage interface (module path and version).
- **R-007 csrf.** Session mode behavior (is a cookie still set?); `extractors.Chain(FromForm, FromHeader)` accepted by the security validation?; `TokenFromContext`; ErrorHandler signature; SingleUseToken trade-offs; `Sec-Fetch-Site` handling; status of PR #4596 (CrossOriginProtectionOnly mode) and whether to adopt it as an extra layer; Go 1.25 `net/http.CrossOriginProtection` as a comparison.
- **R-008 limiter.** Available algorithms (fixed vs sliding window), KeyGenerator and LimitReached signatures, Storage usage, response headers, redaction.
- **R-009 requestid + logger.** `requestid.FromContext`; logger `CustomTags`/`Stream`; whether to use Fiber's logger middleware or a custom slog middleware (recommendation: a custom slog middleware for full control).
- **R-010 recover.** PanicHandler field, stack trace option default, how the error reaches ErrorHandler.
- **R-011 static.** `fs.FS` support (go:embed), MaxAge/Cache-Control, Browse off by default, dotfile handling, Compress, the trailing `*` route requirement.

### Views and frontend
- **R-012 templ + Fiber v3.** Recommended render pattern (`BodyWriter` vs stream writer); passing `fiber.Ctx` as `context.Context`; any CSP nonce helper in templ (check templ.guide security section); how `templ.URL`/`SafeURL` sanitize `href`; `templ generate --watch` for `make dev`.
- **R-013 Tailwind v4 standalone.** Binary names per OS/arch; whether releases publish checksums (else compute and pin our own); `@source` path semantics; whether `.templ` files are detected; plugin availability in the standalone v4 build (forms/typography); `--minify` / `--watch` flags.
- **R-018 htmx (P7).** Current 2.x version; CSP-relevant config (eval, indicator styles, self requests, script tags in swaps) and the meta-tag config format; `hx-headers` for CSRF.

### Data
- **R-014 sqlc.** pgx/v5 engine config, nullable type overrides, `:execrows`, patterns for an allowlisted dynamic ORDER BY.
- **R-015 goose.** Embedded migrations API, advisory locking for concurrent boots, separate migration role.

### Standards
- **R-016 Passwords.** Current OWASP Password Storage Cheat Sheet argon2id parameters; ASVS 5.0 V6 requirements (minimum/maximum length, breached-password check, no composition rules).
- **R-017 ASVS source.** Location and format of the machine-readable ASVS 5.0 release in github.com/OWASP/ASVS (CSV/JSON) to generate the L2 tracker.
- **R-019 golangci-lint.** Current major version config format; depguard rule syntax for module boundaries; recommended gosec excludes (none by default).
- **R-020 ZAP baseline.** Current GitHub Action version and rules-file format.
- **R-021 Boundary test.** `go list -deps -json` approach as a fallback to depguard.
- **R-022 Body limits.** Whether fasthttp reads the full body before middleware runs; `StreamRequestBody` implications; how to enforce per-route limits.
- **R-023 Multipart.** Fiber v3 `FormFile` and multipart limits (BodyLimit enforcement per the v3 notes); memory vs temp-file behavior.

## 6. Seed findings (researched 2026-10-05; re-verify before relying on them)

| Finding | Source |
|---|---|
| Fiber v3.0.0 released 2026-02-02; v3.5.0 (2026-08-13) was latest on that date | github.com/gofiber/fiber/releases |
| Fiber v3 requires Go 1.25+; `Ctx` implements `context.Context` | docs.gofiber.io/whats_new |
| **CVE-2026-53624 / GO-2026-5912**: helmet never set HSTS (checked protocol version instead of scheme). Fixed in v3.4.0 | GHSA-gv83-gqw6-9j2c |
| **CVE-2026-42554**: XSS via `AutoFormat()` choosing HTML from attacker-controlled `Accept`. Fixed in v3.2.0 / v2.52.13 (corrected by R-001) | GHSA-qjv7-627w-8qjv |
| **CVE-2026-25882 / GO-2026-4543**: DoS via route parameter overflow. Fixed in v3.1.0 / v2.52.12 (R-001) | api.osv.dev/v1/vulns/GO-2026-4543 |
| **CVE-2026-45045 / GO-2026-5887**: fixed in v3.3.0 / v2.52.14 (read the advisory for impact) | GHSA-gcfq-8gqf-4876 |
| Default helmet sends no HSTS and no CSP; default CORS allows all origins; default CSRF cookie is insecure; order Helmet → CORS → CSRF; never extract the CSRF token from the cookie | docs.gofiber.io/blog/fiber-v3-security-middleware-stack (2026-05-02) |
| Session v3: IdleTimeout + AbsoluteTimeout; manual `Release()`; secure-token IDs by default; context-aware Regenerate/Destroy | docs.gofiber.io/whats_new |
| CSRF v3: `Expiration` → `IdleTimeout` (default 30m); validates `Sec-Fetch-Site` on unsafe methods; values redacted in logs | docs.gofiber.io/whats_new |
| `c.IP()` walks XFF right-to-left to the first untrusted IP when `EnableIPValidation` + `TrustProxyConfig` are set | docs.gofiber.io/whats_new |
| Middleware data accessed via `FromContext` helpers, not string Locals keys | docs.gofiber.io/whats_new |
| PR #4596 adds a tokenless `CrossOriginProtectionOnly` CSRF mode (status unknown) | github.com/gofiber/fiber/pull/4596 |
| OWASP Top 10:2025 final (Jan 2026): A01 Broken Access Control (now includes SSRF) … A10 Mishandling of Exceptional Conditions | owasp.org/Top10/2025/ |
| ASVS 5.0.0 (May 2025): about 350 requirements, 17 chapters, levels L1–L3 | github.com/OWASP/ASVS |
| `os.Root` (Go 1.24) resists traversal and symlink escape, but doesn't stop access to unintended files *inside* the root | go.dev/blog/osroot |
| Tailwind ships a standalone CLI that runs without Node | tailwindcss.com / GitHub releases |

## 7. Templates

### Research note: `docs/research/R-xxx-<slug>.md`

```markdown
# R-xxx <title>
- Date: YYYY-MM-DD
- Pinned versions: fiber vX.Y.Z, go 1.xx.x, ...
- Question:
- Sources (with versions/URLs):
- Findings (facts only):
- Decision / impact on BASELINE.md:
- Proven by test: <TestName> in <path>
- Re-check trigger: (e.g. any fiber minor upgrade)
```

### ADR: `docs/adr/NNNN-<slug>.md`

```markdown
# NNNN <decision>
- Status: proposed | accepted | superseded by NNNN
- Context:
- Options considered (with pros/cons):
- Decision:
- Security impact (OWASP/ASVS refs):
- Consequences / follow-ups:
```

## 8. Ongoing cadence

- **Every PR:** `make check` (includes govulncheck).
- **Weekly (CI schedule):** govulncheck on main; Renovate/Dependabot PRs; OSV query for gofiber.
- **Every Fiber/templ/Tailwind upgrade:** re-run the R-items whose "re-check trigger" matches; read the release notes and advisories since the last pin; full test suite plus ZAP baseline.
- **Watch:** github.com/gofiber/fiber security advisories, the Go security announcements list, and OWASP project releases.

## 9. Reading path for the human maintainer (in order, about 1–2 days)

1. OWASP Top 10:2025: read each category's "How to prevent" section. *Map each one to BASELINE §9.*
2. OWASP Cheat Sheets (cheatsheetseries.owasp.org): Authentication, Session Management, Password Storage, CSRF Prevention, Content Security Policy, SSRF Prevention, Error Handling, Logging, File Upload, Input Validation, Authorization.
3. ASVS 5.0: skim L1 fully, then the L2 requirements in the authentication, session, authorization, validation/encoding, and configuration chapters.
4. Go: go.dev/doc/security/best-practices, the `html/template` security model (pkg.go.dev/html/template), go.dev/blog/osroot, govulncheck docs.
5. Fiber v3: docs.gofiber.io/whats_new, then the docs pages for helmet, csrf, session, limiter, extractors, reverse proxy, error handling, and the 2026 blog posts on the security stack and sessions.
6. templ.guide (security section), docs.sqlc.dev (pgx), Tailwind v4 docs (installation → standalone CLI, `@source`).

While reading, keep a running list of disagreements with BASELINE.md. Each one becomes an ADR.
