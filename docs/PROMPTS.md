# Prompts for Claude Code

## Setup (once)

1. Create an empty repo. Put `CLAUDE.md` at the root and `BASELINE.md`, `RESEARCH.md`, `PROMPTS.md` in `docs/`.
2. Replace the placeholder module path (`example.com/app`) with yours when Claude Code runs `go mod init`.
3. Start Claude Code in the repo. Run one phase per session (or `/clear` between phases) so context stays focused.
4. Ask for a plan first on every phase, and approve it before code is written.

---

## P0: Research and decisions

```
Read CLAUDE.md, docs/BASELINE.md and docs/RESEARCH.md fully.
Do Phase P0 only. Do not write application code.
For R-001..R-007, R-012 and R-013: follow the protocol in RESEARCH.md §3.
Use the pinned-source-first hierarchy (§2): create a throwaway module in spikes/
(gitignored), `go get` the candidate versions, and confirm APIs with `go doc` and the
module cache source. Use web docs/advisories for versions and CVEs.
Write one note per item in docs/research/, write docs/adr/0001-stack.md with exact
pinned versions, and create docs/PROGRESS.md.
Finally, list every place where your findings contradict BASELINE.md and propose the edits.
Do not apply them until I approve.
```

## P1: Skeleton

```
Phase P1 per BASELINE §10. Use the versions from ADR 0001.
Create go.mod, Makefile (tools target downloads pinned tailwindcss/templ/sqlc and verifies
SHA-256), .golangci.yml, .github/workflows/ci.yml (actions pinned by SHA, minimal permissions),
framework/config with production guards (§6.13), cmd/server with /healthz and /readyz,
.env.example and .gitignore.
Write TestProdGuards first, then the implementation. Show me the plan before coding.
```

## P2: Kernel

```
Phase P2. Implement framework/kernel (Module, Router, Registry), framework/security/policy,
framework/httpx (AppError, ErrorHandler, SafeRedirect), 404 handlers, and the module boundary
rule (depguard + TestModuleBoundaries).
The policy must be a required positional argument on every route method; modules must not be
able to reach fiber.Router directly.
Acceptance tests: TestEveryRouteHasPolicy, TestErrorNoLeak, TestPanicRecovered,
TestSafeRedirect, TestModuleBoundaries. Plan first.
```

## P3: Security core

```
Phase P3. Resolve R-008..R-011 and R-022 first (notes in docs/research/).
Then implement the global middleware pipeline in the exact order of BASELINE §5:
recover, requestid, slog access log with redaction, helmet (CSP disabled), csp nonce
middleware, static, global limiter, session (Postgres storage), csrf (session mode,
form+header extractor), auth loader. Apply the fiber.Config table from §5.1.
Acceptance tests: TestSecurityHeaders (incl. HSTS behind trusted proxy), TestCSPNonce,
TestSessionCookieFlags, TestCSRF_* (missing/wrong/foreign token, bad Sec-Fetch-Site),
TestTrustProxySpoof, TestBodyLimit413, TestLogRedaction. Plan first.
```

## P4: Data

```
Phase P4. Resolve R-014 and R-015. Implement framework/db (pgx pool, WithTx, ctx timeouts),
goose embedded migrations with a `migrate` subcommand, sqlc.yaml, and a CI Postgres
service. Use separate DB roles for app (DML) and migrations (DDL) as in BASELINE §7.
Tests: migrations up/down, query timeout honored. Plan first.
```

## P5: Views and Tailwind

```
Phase P5. Implement framework/view (Page, Render, layout, CSRFField, FieldError, flash,
error pages, Asset() with manifest) and the Tailwind v4 standalone pipeline with
content-hashed output and immutable cache headers. No inline styles or scripts; the layout
uses the CSP nonce only where a script tag is unavoidable.
Tests: TestXSSEscaping for framework pages, TestAssetCacheHeaders. Plan first.
```

## P6: Auth module

```
Phase P6. Resolve R-016. Build internal/modules/auth as the reference module following
BASELINE §6.2, §6.4, §6.5, §6.6, §6.8: register, login, logout (POST only), argon2id
password service with NeedsRehash, offline common-password list, auth rate limiter
(per IP and per account), session regeneration, RBAC registry, security events,
THREATS.md, and a contract/ package.
Acceptance: TestLoginGenericError, TestSessionRotatesOnLogin, TestRateLimitLogin,
TestPasswordHashVerify, TestNeedsRehash, TestSecurityEventsEmitted. Plan first.
```

## P7: Blog module (HMVC demo)

```
Phase P7. Resolve R-018 and R-023. Build internal/modules/blog: owner-scoped CRUD via sqlc
(not-owned → 404), sort allowlist, contract.Widgets.LatestPosts used by the home page,
optional htmx delete with CSRF header and CSP-safe htmx config, cover-image upload via
os.Root per §6.10.
Acceptance: TestIDOR_*, TestSortAllowlist, TestUploadTraversal, widget test,
TestXSSEscaping for blog views. Plan first.
```

## P8: Hardening and developer experience

```
Phase P8. Resolve R-017, R-019, R-020. Build cmd/scaffold `make:module <name>` (generated
policy stub must fail tests until a real policy is chosen), httpx.SafeClient with the tests
in §6.9, multi-stage Dockerfile (CGO_ENABLED=0, distroless nonroot), SBOM step, ZAP
baseline job, Renovate config, PR template with security checklist, and generate
docs/asvs/ASVS-L2.md from the official ASVS 5.0 source with evidence links.
Done when ZAP baseline shows no High/Medium and every L2 row is Pass or N/A with reason.
```

---

## Reusable prompts

**Security review (run after each phase)**
```
Act as a skeptical application security reviewer. Review the diff since the last phase
against CLAUDE.md security invariants and BASELINE §9. For each OWASP 2025 category, say
which controls/tests cover it now and list gaps. Grep for: templ.Raw, Sprintf near SQL,
err.Error() in responses, math/rand, AutoFormat, http.Get/http.DefaultClient, os.Open with
user input, missing ctx timeouts. Report findings by severity; don't fix until I approve.
```

**Upgrade check**
```
Check for newer versions of every pinned dependency and tool. For each: release notes and
advisories since our pin, breaking changes affecting our code, which R-items must be
re-verified. Propose an upgrade plan; don't upgrade yet.
```

**Spec drift check**
```
Compare the codebase to docs/BASELINE.md. List every deviation (code does X, spec says Y),
classify as bug-in-code or spec-needs-ADR, and propose fixes.
```
