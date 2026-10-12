# Baseline Specification: Secure HMVC Framework (Go Fiber v3 + templ + Tailwind v4)

- **Version:** 0.2, 2026-10-12 (P0 research findings applied; 0.1 baseline 2026-10-05)
- **Audience:** Claude Code and human maintainers
- **Keywords:** MUST / MUST NOT / SHOULD as in RFC 2119
- **⟨verify⟩** marks an API detail or version that must be confirmed per `docs/RESEARCH.md` before implementation. Record the outcome in `docs/research/`.

---

## 0. Goals and non-goals

**Goals**
- CodeIgniter 3 + HMVC (Modular Extensions) ergonomics: self-contained modules (controller, model, views, routes) on a thin core the app developer never edits.
- Secure by default against **OWASP Top 10:2025**, verifiable against **OWASP ASVS 5.0 Level 2**.
- Server-rendered HTML first, styled with Tailwind. Optional progressive enhancement with self-hosted htmx.
- A single static binary plus Postgres. No Node.js in the toolchain.

**Non-goals for the baseline (see §13 for the deferred backlog)**
SPA frontend, GraphQL, multi-tenancy, acting as an OAuth/OIDC provider, WebSockets, plugin marketplace, CI3-style auto-routing (rejected permanently, see §2).

---

## 1. Pinned stack

| Component | Choice | Pin rule | Reason |
|---|---|---|---|
| Language | Go, latest stable ≥ 1.25 | `go 1.27.1` line in go.mod (a `toolchain` line only when it differs from `go`); CI sets `GOTOOLCHAIN=local` | Fiber v3 requires ≥ 1.25; `os.Root` |
| HTTP | `github.com/gofiber/fiber/v3` | exact version, **≥ v3.5.0** | earlier versions carry known advisories (RESEARCH §6) |
| Views | `github.com/a-h/templ` | lib and CLI at the same exact version | compile-time templates, contextual escaping |
| Database | PostgreSQL 16+ | — | |
| Driver | `github.com/jackc/pgx/v5` | exact | |
| Queries | sqlc (CLI) | exact CLI version in Makefile | generated, parameterized SQL |
| Migrations | `github.com/pressly/goose/v3` | exact | embedded SQL migrations |
| Validation | `github.com/go-playground/validator/v10` | exact | |
| Passwords | `golang.org/x/crypto/argon2` | exact | argon2id |
| HTML sanitizer | `github.com/microcosm-cc/bluemonday` | exact | only for rich-text fields |
| Session/limiter storage | `github.com/gofiber/storage/postgres/v3` via `NewFromConnection(pool, …)` (R-006) | exact | server-side sessions, shared limiter state |
| Logging | `log/slog` (stdlib) | — | |
| Config | stdlib (`os.LookupEnv` + typed parsing) | — | avoid a config dependency |
| CSS | Tailwind CSS v4 standalone CLI | version + SHA-256 in Makefile | no Node |
| JS (optional) | htmx 2.x, self-hosted | version + SHA-256 in repo | progressive enhancement |
| Tooling | staticcheck, golangci-lint (gosec, depguard, errcheck, bodyclose), govulncheck | templ/staticcheck/govulncheck via the go.mod `tool` directive; sqlc/golangci-lint/tailwindcss as checksum-verified binaries (ADR 0001) | |

**Dependency budget:** about 12 direct dependencies at most. Each addition needs an ADR. Exact versions and the 14-day soak policy live in `docs/adr/0001-stack.md`.

---

## 2. CodeIgniter 3 → framework mapping

| CI3 / HMVC concept | Here | Notes |
|---|---|---|
| `index.php` front controller | `cmd/server/main.go` | bootstrap and explicit module list only |
| `system/` | `framework/` | core; app developers don't edit it |
| `application/modules/<m>/` (MX) | `internal/modules/<m>/` | one Go package tree per module |
| `controllers/` | `handler.go` | thin: bind → validate → service → render |
| `models/` | `queries.sql` (sqlc) + `service.go` | queries are generated; rules live in the service |
| `views/` | `views/*.templ` | compiled to Go |
| `config/routes.php` | `module.go` → `Routes()` | explicit; a policy is required per route |
| Auto-routing `/class/method/arg` | **removed** | exposes every public method (A01) |
| `$this->load->model()/library()` | constructor injection via `Init(k)` | no super-object |
| `Modules::run('blog/widget')` | module `contract` package returning `templ.Component` | in-process, no internal HTTP |
| Hooks | Fiber middleware + kernel lifecycle hooks | |
| `libraries/`, `helpers/` | `framework/pkg/*` | |
| `form_validation` | validator tags + `framework/validate` | field errors safe to display |
| `$this->security->xss_clean()` | **removed** | validate input, encode output (templ) |
| Query Builder | sqlc | |
| Session library | Fiber session middleware, server-side storage | |
| `config/*.php` | typed `config.Config`, validated at boot | fail closed in production |
| Output cache | Fiber cache middleware, opt-in, Public GET routes only | |
| CLI (CI4 `spark`) | `cmd/scaffold` | `make:module <name>` |

---

## 3. Repository layout

```
.
├── CLAUDE.md
├── Makefile
├── go.mod / go.sum
├── sqlc.yaml
├── .golangci.yml
├── cmd/
│   ├── server/main.go            # bootstrap: config → kernel → modules → listen
│   └── scaffold/main.go          # make:module generator
├── framework/
│   ├── config/                   # typed config, env parsing, production guards
│   ├── kernel/                   # Kernel, Module interface, Router, route registry
│   ├── security/
│   │   ├── policy/               # Policy, Public, Authenticated, Can, Any
│   │   ├── authz/                # RBAC: roles → permissions (in code)
│   │   ├── csp/                  # per-request nonce + CSP header
│   │   ├── headers/              # helmet config
│   │   ├── session/              # session store setup, helpers (Regenerate, Flash)
│   │   ├── csrf/                 # csrf setup, error handler
│   │   ├── ratelimit/            # global + named limiters
│   │   └── password/             # argon2id hash/verify/needsRehash
│   ├── httpx/                    # AppError, ErrorHandler, SafeClient, safe redirect
│   ├── view/                     # Render, Page data, layouts, form helpers, assets
│   ├── validate/                 # validator wrapper, field errors
│   ├── db/                       # pgx pool, WithTx, migrations runner
│   ├── logging/                  # slog setup, redaction, security events
│   └── pkg/                      # sanitize, ids, files (os.Root), etc.
├── internal/
│   ├── db/                       # sqlc output (generated; do not edit)
│   └── modules/
│       ├── auth/                 # reference module: register/login/logout
│       │   ├── contract/         # what other modules may import
│       │   ├── module.go  handler.go  service.go  queries.sql
│       │   ├── views/*.templ
│       │   ├── THREATS.md
│       │   └── module_test.go
│       └── blog/                 # reference HMVC module: owner-scoped CRUD + widget
├── migrations/                   # goose SQL, embedded
├── web/
│   ├── styles/input.css          # Tailwind entry
│   ├── js/                       # htmx (pinned) + tiny app.js (no eval)
│   └── static/dist/              # build output (hashed), go:embed
├── docs/
│   ├── BASELINE.md  RESEARCH.md  PROGRESS.md
│   ├── research/                 # R-xxx notes
│   ├── adr/                      # decisions
│   └── asvs/ASVS-L2.md           # requirement tracker
└── .github/workflows/ci.yml
```

---

## 4. Kernel and module contract

### 4.1 Module interface (sketch; ⟨verify⟩ Fiber types against the pinned version)

```go
// framework/kernel/module.go
type Module interface {
	Name() string          // unique, lowercase; used in logs and the route registry
	Init(k *Kernel) error  // receive deps: Config, DB, Logger, Sessions, Authz
	Routes(r *Router)      // declare routes; called exactly once at boot
}
```

Modules are listed explicitly in `cmd/server/main.go`, e.g. `k.Register(auth.New(), blog.New())`. No `init()` side effects and no reflection.

### 4.2 Policy-enforcing router

```go
// framework/kernel/router.go
type Router struct{ /* wraps fiber.Router + module name + *Registry */ }

func (r *Router) Group(prefix string, opts ...GroupOption) *Router // e.g. AsAPI() for JSON errors
func (r *Router) Get(path string, h fiber.Handler, p policy.Policy, mw ...fiber.Handler)
func (r *Router) Post(path string, h fiber.Handler, p policy.Policy, mw ...fiber.Handler)
func (r *Router) Put(path string, h fiber.Handler, p policy.Policy, mw ...fiber.Handler)
func (r *Router) Patch(path string, h fiber.Handler, p policy.Policy, mw ...fiber.Handler)
func (r *Router) Delete(path string, h fiber.Handler, p policy.Policy, mw ...fiber.Handler)
// Deliberately NOT exposed to modules: Use, All, Add, Route, RouteChain, Domain,
// raw Group, Query, Connect, Trace (Fiber v3 has no Static/Mount methods; R-003).
```

- The policy is a **required positional argument**, so a route without one does not compile.
- Handlers are typed `fiber.Handler`, which is stricter than Fiber's `any` (Fiber only panics at registration on a bad type).
- A `nil` policy panics at boot.
- The handler chain is `[policy.Middleware(p), mw..., h]`.
- Each registration appends `{module, method, fullPath, policyName}` to the Registry, which tests use.
- Route params MUST use constraints where typed (`/:id<int>`, or a custom constraint such as `<ulid>`).
- Custom constraints MUST be registered by the kernel before any module's `Routes()`. Fiber silently ignores a constraint registered after the route.
- Boot MUST fail on any `<…>` constraint that is neither built-in nor registered. Fiber silently drops unknown names, so a typo such as `<itn>` fails open (R-003).

### 4.3 Policies

```go
// framework/security/policy
type Policy interface {
	Name() string
	Authorize(c fiber.Ctx) error // nil = allow; ErrUnauthenticated / ErrForbidden otherwise
}
var Public Policy                // explicit opt-out, Name() == "public"
func Authenticated() Policy
func Can(perm string) Policy     // permission must exist in authz registry, else panic at boot
func Any(ps ...Policy) Policy
```

- **Fail closed:** any error or unknown state means deny.
- Unauthenticated on an HTML route → redirect to `/login?next=<path>`. On an API group → 401 JSON.
- `next` MUST pass `httpx.SafeRedirect`: relative path only, starts with `/`, does not start with `//` or `/\`, no scheme. Otherwise use `/`.
- Object-level checks are NOT policies. They live in queries (§7), and a not-owned resource returns 404.

### 4.4 Module boundaries

A module may import `framework/...`, `internal/db`, and **other modules' `contract` subpackages only**. This is enforced by a depguard rule plus `TestModuleBoundaries`, which walks `go list -deps`.

Cross-module rendering (the HMVC "widget") works through an interface in the contract package:

```go
// internal/modules/blog/contract/contract.go
type Widgets interface {
	LatestPosts(ctx context.Context, limit int) (templ.Component, error)
}
```

### 4.5 Boot sequence

1. Load config and run the production guards (§6.13).
2. Logger.
3. DB pool, then migrations (if `MIGRATE_ON_BOOT=true`).
4. Session store.
5. Build the Fiber app (§5.1).
6. Global middleware (§5).
7. `Init()` every module, then `Routes()` every module.
8. Freeze the registry. Boot fails if any permission referenced by `Can()` is undefined.
9. Install the 404 handler and listen.
10. On shutdown: drain connections, then close the DB.

---

## 5. Request pipeline (global middleware, in this order)

| # | Middleware | Purpose | Notes |
|---|---|---|---|
| 1 | recover | turn panics into 500 | custom PanicHandler logs the stack server-side only ⟨verify field name⟩ |
| 2 | requestid | correlation | expose via `requestid.FromContext` |
| 3 | access log | slog JSON | method, route pattern, status, latency, ip, request_id, user_id; never the query string on `/auth/*` |
| 4 | helmet | static security headers | HSTS on; **CSP disabled here** (set by #5) |
| 5 | csp | per-request nonce + CSP header | `framework/security/csp` |
| 6 | static | `/assets/*` from embedded FS | before session, so assets never create sessions; `Cache-Control: public, max-age=31536000, immutable` for hashed files; no directory browsing |
| 7 | limiter (global) | abuse control | 300 req/min per client IP (configurable) |
| 8 | session | server-side session | |
| 9 | csrf | synchronizer token in session | unsafe methods only |
| 10 | auth loader | user from session into Locals | no enforcement; policies enforce |
| 11 | module routes | policy per route | |
| 12 | 404 handler | HTML or JSON per group | |

- **CORS is not installed by default.** A module that needs it enables it on its own group with explicit origins. Never `*` with credentials.
- The order static → limiter → session is load-bearing: the session middleware writes a storage row and sets a cookie on **every** request that reaches it (R-006).
- **Reverse proxy requirements** (R-004): the trusted proxy MUST overwrite `X-Forwarded-Proto` (not append), MUST strip client-supplied `X-Forwarded-Protocol`, `X-Forwarded-Ssl`, `X-Url-Scheme` and `X-Forwarded-Host`, and MUST append the peer address to `X-Forwarded-For`.
- Code rule: use only `c.IP()` for security decisions (rate limits, audit, allowlists), never `c.IPs()`. Never build absolute URLs from `c.Host()`/`c.BaseURL()`; use `BASE_URL` from config.
- `Config.ErrorHandler` = `httpx.ErrorHandler` (§6.7).

### 5.1 `fiber.Config` baseline (verified at fiber v3.5.0, R-003/R-004)

| Field | Value | Why |
|---|---|---|
| `ServerHeader` | `""` (set explicitly) | no fingerprinting |
| `BodyLimit` | max allowed upload size (default 4 MiB) | DoS; see the per-route note below |
| `ReadTimeout` / `WriteTimeout` / `IdleTimeout` | 10s / 15s / 60s | slowloris, A10. `ReadTimeout` covers the full body read; revisit with R-022/R-023 for large uploads |
| `Immutable` | `true` | Ctx strings stay valid after the handler returns; prevents subtle data races |
| `ErrorHandler` | `httpx.ErrorHandler` | A10; Fiber's default sends `err.Error()` to the client |
| `TrustProxy` | `len(cfg.TrustedProxies) > 0` | |
| `TrustProxyConfig.Proxies` | from `TRUSTED_PROXIES` (IPs or CIDRs), never `0.0.0.0/0` or `::/0`. `Loopback`, `Private`, `LinkLocal`, `UnixSocket` stay `false` | IP spoofing. Fiber silently drops unparsable entries, so boot fails on them (§6.13) |
| `ProxyHeader` | `X-Forwarded-For` only when proxies are configured, else `""` | |
| `EnableIPValidation` | `true` (**MUST**) | without it `c.IP()` returns the raw `X-Forwarded-For` string (client-controlled); with it, the rightmost untrusted valid IP |
| `SkipUnmatchedRoutes` | `false` (set explicitly) | `true` answers 404/405 before middleware, bypassing the limiter, access log and 404 handler |
| `PassLocalsToContext` | `false` | |
| `DisableHeadAutoRegister` | `false` (default) | |
| `ListenConfig.DisableStartupMessage` | `true` in production | |
| `ListenConfig.ListenerNetwork` | `"tcp"` if IPv6 is needed | the default `"tcp4"` does not listen on IPv6 |
| `ListenConfig.TLSMinVersion` | `tls.VersionTLS12` when terminating TLS in-process | A04. Ignored when `ListenConfig.TLSConfig` is set (set `MinVersion` there) |

**Per-route body limit:** the global `BodyLimit` is the largest upload. Non-upload routes get `limits.Body(64 << 10)` middleware that rejects a declared `Content-Length` above the limit. ⟨verify whether fasthttp reads the body before middleware, and whether `StreamRequestBody` is needed (R-022)⟩

---

## 6. Security controls specification

### 6.1 Headers and CSP (A02, A05)

**helmet**
- `HSTSMaxAge: 63072000`, include subdomains.
- `HSTSPreloadEnabled: false` until the domain owner decides to preload (it is hard to undo).
- `ContentSecurityPolicy: ""` (owned by the csp middleware).
- `XFrameOptions: "DENY"` (the default is SAMEORIGIN), matching CSP `frame-ancestors 'none'`.
- `ReferrerPolicy: "same-origin"`, **not** `no-referrer`: with `no-referrer`, the CSRF Origin/Referer check rejects every HTTPS form POST that arrives with `Origin: null` (R-007).
- Keep the defaults: nosniff, COEP/COOP/CORP. COEP `require-corp` blocks any cross-origin subresource without CORP/CORS; a future cross-origin image needs a deliberate override.
- `PermissionPolicy: "camera=(), microphone=(), geolocation=(), payment=()"` unless a module opts in. It has **no default**, so without it no header is sent (R-005).
- Requires fiber ≥ v3.4.0 (HSTS fix, CVE-2026-53624). HSTS is emitted only when `c.Scheme()` is https, which behind a proxy requires a trusted proxy (R-004).

**CSP middleware**
- Generates a 16-byte `crypto/rand` nonce per request, base64-encoded, stored under a typed Locals key, and sends:

```
default-src 'none'; script-src 'self' 'nonce-{N}'; style-src 'self'; img-src 'self' data:;
font-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none';
base-uri 'none'; object-src 'none'; upgrade-insecure-requests
```

- `upgrade-insecure-requests` is sent in production only.
- Optional `report-to` endpoint is deferred.
- **Tests:** the nonce differs per request; the nonce in the header equals the nonce on every `<script>` in the HTML; HSTS is present when the scheme is https, including behind a trusted proxy, and **absent** for an untrusted peer that sends `X-Forwarded-Proto: https` (R-004); `Referrer-Policy: same-origin`.

### 6.2 Sessions (A07)

- Server-side storage (Postgres adapter), wired as `session.NewWithStore(session.Config{Storage: pg, Extractor: extractors.FromCookie("__Host-sid"), …})`. Handlers get the session with `session.FromContext(c)` and nil-check it (R-006).
- Cookie `__Host-sid`: Secure, HttpOnly, SameSite=Lax, Path `/`, no Domain.
- IdleTimeout 30m. Fiber's `AbsoluteTimeout: 12h` is a store-wide backstop counted from session **creation**. The real 12h (8h for admin role) limit counts from login: the auth loader compares the stored `auth_time` and on expiry calls `Destroy()` and redirects to login.
- ID from the middleware's secure generator; never user-supplied (an unknown cookie ID is not adopted).
- Login, role change and password change call `Regenerate()` (new ID, old row deleted, data **kept**; set `uid`/`auth_time` after it), then rotate the CSRF token (§6.3). **Logout calls `Destroy()`**, not `Regenerate()`, which would keep `uid`. Avoid `Reset()` on authenticated flows: it drops the absolute deadline.
- Store only the user ID, auth time, roles version, and flash. No PII blobs.
- **Tests:** cookie flags, ID rotation on login, idle expiry, absolute expiry from `auth_time`, logout invalidates server-side.

### 6.3 CSRF (A01/A08)

- Session-backed synchronizer token (`Session: store`).
- Extractor: `extractors.Chain(extractors.FromForm("_csrf"), extractors.FromHeader(csrf.HeaderName))` (htmx uses the header). **Never** extract from the cookie; Fiber panics at boot if the chain reads the CSRF cookie (R-007).
- Cookie `__Host-csrf`: Secure, HttpOnly, SameSite=Lax, Path `/`, no Domain.
- IdleTimeout 30m. `SingleUseToken: false`. `TrustedOrigins: nil`.
- The session middleware runs before csrf (§5 #8 → #9).
- On login, logout and privilege change, rotate the token with `csrf.HandlerFromContext(c).DeleteToken(c)` after `Regenerate()`. `Regenerate()` alone keeps the pre-login token valid.
- On failure: 403 page, plus security event `csrf.rejected` with a `reason` attribute mapped from Fiber's exported errors.
- Templates get the token through `view.Page.CSRF`, filled from `csrf.TokenFromContext(c)` with the `fiber.Ctx` (not `c.Context()`). Forms use `@view.CSRFField()`. htmx gets it via an `hx-headers` attribute on `<body>` (no inline script).
- Logout MUST be POST.
- Not adopted for now: Fiber PR #4596 (`CrossOriginProtectionOnly`, unmerged) and `net/http.CrossOriginProtection`; re-evaluate when #4596 ships.
- **Tests:** missing token → 403; wrong token → 403; token from another session → 403; malformed `Sec-Fetch-Site` → 403; cross-site request with a foreign `Origin` → 403 (`Sec-Fetch-Site: cross-site` alone is not a rejection reason); HTTPS form POST with `Origin: null` and a same-origin Referer → 200; GET is not blocked.

### 6.4 Authentication (A07)

- Login error is always "Invalid email or password". For an unknown user, verify against a fixed dummy hash so timing is similar.
- Password rules follow ASVS 5.0 V6 ⟨verify current numbers, R-016⟩:
  - minimum length per ASVS, maximum at least 64 characters, all Unicode allowed, no composition rules;
  - reject passwords on a bundled common/breached list (offline file, no external call by default).
- argon2id parameters come from the current OWASP Password Storage Cheat Sheet ⟨verify, R-016⟩ (baseline: m=19 MiB, t=2, p=1), with a 16-byte salt, 32-byte key, and PHC-format string.
- Compare with `subtle.ConstantTimeCompare`. `NeedsRehash` triggers a re-hash on successful login.
- Sensitive operations (password or email change) re-prompt for the current password.
- Registration doesn't reveal whether an email exists. It uses a generic message and, later, email verification.
- Password reset and MFA are deferred (§13), but `auth/contract` reserves hooks for them.

### 6.5 Authorization (A01)

- RBAC defined in code: `authz.Define("post.create", "post.delete.any", ...)` and `authz.Role("editor", perms...)`. User roles are stored in the DB.
- Policies check permissions. Object ownership is enforced in SQL.
- Admin routes live in a separate group with `Can("admin.access")` plus a shorter absolute timeout.
- **Tests:** `TestEveryRouteHasPolicy` (registry: no route without a policy, and the only `public` unsafe-method routes are login/register); IDOR tests per module.

### 6.6 Rate limiting (A06/A07)

- Global per-IP limiter (from the pipeline above).
- Named limiter `auth`: 5 POSTs/min per IP **and** 10/hour per normalized account identifier (custom KeyGenerator).
- Storage is shared (Postgres or Redis) so multiple instances agree.
- On limit: 429, a generic page, a `Retry-After` header, and event `ratelimit.hit`.
- ⟨verify algorithm options and KeyGenerator signature, R-008⟩

### 6.7 Errors and exceptional conditions (A10)

```go
// framework/httpx
type AppError struct {
	Status int    // HTTP status
	Code   string // stable machine code, e.g. "post.not_found"
	Public string // safe message for users
	Err    error  // internal cause, logged only
}
```

**ErrorHandler**
- `AppError` → its status and Public message.
- `*fiber.Error` → its code with the standard status text.
- Anything else → 500 "Something went wrong" plus the request ID.
- Logs the full cause at ≥ 500 (and at 4xx for security-relevant codes).
- Renders an HTML or JSON body based on the **route group flag**, not on `Accept`.

**Rules**
- Authz and validation errors fail closed.
- Every DB and HTTP call has a context deadline (default 5s).
- No `panic` for control flow.
- Goroutines started from handlers must not capture `fiber.Ctx`; copy the values they need first.

**Tests:** a panic route and an internal-error route both return a generic body with no stack, no SQL, and no file paths.

### 6.8 Logging and alerting (A09)

- slog JSON to stdout.
- Redaction ReplaceAttr drops or masks the keys `password`, `token`, `secret`, `authorization`, `cookie`, `session`, `csrf`.
- Security events, with stable names:
  - `auth.login_failed`, `auth.login_succeeded`, `auth.logout`
  - `authz.denied`, `csrf.rejected`, `ratelimit.hit`
  - `session.expired`, `password.changed`, `admin.action`
- Docs include alert rules (e.g. more than 20 `auth.login_failed` per minute per IP, any `admin.action` outside business hours) as a sample for the log platform.

### 6.9 Outbound HTTP and SSRF (A01)

`httpx.SafeClient` is the only way modules make outbound requests.

```go
dialer := &net.Dialer{
	Timeout: 5 * time.Second,
	Control: func(network, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || !isPublicUnicast(ip.Unmap()) {
			return ErrBlockedDestination
		}
		return nil
	},
}
```

- `isPublicUnicast` rejects:
  - loopback, private, link-local, multicast, unspecified;
  - `100.64.0.0/10`, `0.0.0.0/8`, `192.0.0.0/24`, `198.18.0.0/15`, `240.0.0.0/4`;
  - IPv6 ULA `fc00::/7`.
- Transport: `Proxy: nil`, scheme allowlist `https` (and `http` only if configured), max 3 redirects (each re-dialed through the same guard), response body capped with `io.LimitReader`, optional host allowlist.
- **Tests:** blocks `127.0.0.1`, `::1`, `10.0.0.1`, `169.254.169.254`, a DNS name resolving to a private IP, and a public→private redirect.

### 6.10 Files and uploads (A01/A05)

- The upload directory is opened once with `os.OpenRoot`. All operations go through the `*os.Root`.
- Stored names are generated (random hex) and never come from the user.
- Type is detected from content (`http.DetectContentType`) against an allowlist, with a size cap.
- Metadata (original name, owner, type, size) is stored in the DB.
- User uploads are served via a handler with `Content-Disposition: attachment` (or inline only for allowlisted image types), `X-Content-Type-Options: nosniff`, and the correct `Content-Type`.
- **Tests:** traversal names (`../x`, absolute paths, symlink escape), oversize, disallowed type.

### 6.11 Cryptography and secrets (A04)

- Secrets come from env/secret manager only: `COOKIE_ENC_KEY` (base64, 32 bytes, length checked at boot). `SESSION_KEY` currently has no consumer, because session IDs are opaque random tokens that Fiber does not sign (R-006). Its fate (drop it, or key an HMAC of stored session IDs) is decided by an ADR before P3.
- `.env` is for local dev only and is gitignored. A `.env.example` is committed.
- Tokens (reset, verification) are 32 bytes from `crypto/rand`. Only their SHA-256 is stored in the DB; they are single-use with an expiry.
- Anything sensitive stored client-side uses encryptcookie (AES-GCM). Prefer not storing it client-side at all.
- DB connections use `sslmode=verify-full` in production.

### 6.12 Input validation (A05)

- Bind into request structs per handler (never bind directly into DB models).
- `validate` tags handle type, length, format, and enum allowlists. `framework/validate` turns failures into field-error maps for templates.
- Normalize before validating (trim, Unicode NFC for identifiers, lowercase emails).
- Reject unknown enum values; don't coerce them.

### 6.13 Configuration and production guards (A02)

Boot MUST fail when `APP_ENV=production` and any of these is true:
- cookie Secure is off;
- `COOKIE_ENC_KEY` (and `SESSION_KEY`, if the ADR keeps it) is missing or the wrong length;
- HSTS is disabled (`HSTSMaxAge <= 0`);
- TLS is neither terminated in-process nor `TRUSTED_PROXIES` non-empty (otherwise `c.Scheme()` is never https and HSTS is never sent);
- `DEBUG=true`;
- `TRUSTED_PROXIES` contains `0.0.0.0/0` or `::/0`;
- DB `sslmode` is not `verify-full`/`verify-ca` (unless `DB_ALLOW_INSECURE_LOCAL=true` with a Unix socket);
- `BASE_URL` is not https.

In **every** environment, boot MUST fail when (R-003, R-004):
- any `TRUSTED_PROXIES` entry does not parse as an IP or CIDR (Fiber silently drops it);
- `UnixSocket` trust is enabled without `Proxies` (the IP becomes leftmost and spoofable);
- `TrustProxy` is on without `EnableIPValidation`.

`config.Validate()` returns all violations at once.

---

## 7. Data layer

- **Pool:** pgx/v5. The app uses a least-privilege DB role (DML only). Migrations use a separate role with DDL rights.
- **Migrations:** goose with SQL files embedded via `embed.FS`, run by `cmd/server migrate` or `MIGRATE_ON_BOOT`. ⟨verify advisory locking, R-015⟩
- **Queries:** sqlc with the pgx/v5 engine. One `queries.sql` per module, with output in `internal/db`. Every query that touches a user-owned table has an owner filter, for example:

```sql
-- name: GetPostForOwner :one
SELECT * FROM posts WHERE id = $1 AND owner_id = $2;
-- name: DeletePostForOwner :execrows
DELETE FROM posts WHERE id = $1 AND owner_id = $2;
```

  If `:execrows` returns 0, the service returns `ErrNotFound`, which surfaces as 404.
- **Dynamic sort:** the request value maps through `map[string]string{"new":"created_at DESC", ...}` to separate sqlc queries (or a `CASE` on `sqlc.arg`). An unknown value means the default sort. ⟨verify the best sqlc pattern, R-014⟩
- **Transactions:** `db.WithTx(ctx, pool, func(q *db.Queries) error)`.
- Every call uses `c.Context()` wrapped with a timeout. Not `c` itself: it is pooled, never cancelled, and must not escape the handler (R-003).
- **Session storage table** (R-006): created by a goose migration with the DDL role. `CREATE TABLE sessions (k TEXT PRIMARY KEY NOT NULL DEFAULT '', v BYTEA NOT NULL, e BIGINT NOT NULL DEFAULT 0); CREATE INDEX sessions_e_idx ON sessions (e);`. The app role gets only `SELECT, INSERT, UPDATE, DELETE`. Storage is built with `postgres.NewFromConnection(pool, postgres.Config{Table: "sessions", GCInterval: time.Minute})`. Never set `Reset: true` or call `store.Reset`. Construction panics on error, so boot wraps it with recover → error.

---

## 8. Views, assets, Tailwind

### 8.1 templ rendering (verified at templ v0.3.1020 + fiber v3.5.0, R-012)

```go
// framework/view
type Page struct {
	Title string
	CSRF  string     // from csrf.TokenFromContext(c)
	Nonce string     // from csp middleware
	Flash []Flash
	User  *UserView  // nil when anonymous
}

func Render(c fiber.Ctx, status int, comp templ.Component) error {
	c.Status(status)
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	ctx := templ.WithNonce(c, csp.NonceFromContext(c))
	if err := comp.Render(ctx, c.Response().BodyWriter()); err != nil {
		c.Response().ResetBody() // otherwise the partial page leaks into the error page
		return err
	}
	return nil
}
```

- Never render templ pages with `SendStreamWriter`: a mid-render error is swallowed and the client gets 200 with truncated HTML.
- Layout components take `Page` explicitly.
- Helpers: `CSRFField()`, `Asset("app.css")` (manifest lookup), `FieldError(name)`.
- Forbidden in templates: `templ.Raw` on non-constant data, `on*=` attributes, `style=` attributes, `javascript:` URLs, spread `templ.Attributes` with user-controlled keys or URL values.
- Every `<script>` carries `nonce={ templ.GetNonce(ctx) }`; templ does not add it to literal script tags. Pass data to scripts with `templ.JSONScript` (nonce added automatically, `</script>` escaped).
- templ sanitizes URLs automatically only in `<a href>`, `<link href>`, `<form action>`, `<object data>`. User data in any other URL attribute (`img src`, `button formaction`, …) MUST be wrapped in `templ.URL(...)`.
- `templ.URL` allows scheme-relative `//host` URLs. Links that must stay on-site use the `httpx.SafeRedirect` rules.
- `TestXSSEscaping` renders known payloads (`<script>`, `"><img onerror>`, `javascript:` links, `img src`, spread attributes) through every module's main views and asserts they come out escaped or sanitized.

### 8.2 Tailwind v4 (standalone CLI)

```css
/* web/styles/input.css */
@import "tailwindcss" source(none);
@source "../../internal/modules";
@source "../../framework/view";
@plugin "@tailwindcss/forms" { strategy: "class"; }
@plugin "@tailwindcss/typography";
```

- `source(none)` turns off automatic detection so only the listed folders are scanned. `@source` paths are relative to the CSS file. `.templ` and `_templ.go` files are both detected (R-013).
- `make css` runs the pinned binary from `./bin/tailwindcss` (downloaded by `make tools` and verified against SHA-256 values committed in the Makefile): `-i web/styles/input.css -o web/static/dist/app.css --minify`. A small Go step then content-hashes it to `app.<hash>.css` and writes `manifest.json`. `make dev` uses `--watch=always`.
- Class names must be complete string literals in `.templ`/`.go` files (no string-built class names).
- `style-src 'self'` works because all CSS is in one external file. The forms plugin's `data:` SVG backgrounds are covered by `img-src 'self' data:`.
- The official forms (class strategy, so no global element styles) and typography plugins ship inside the standalone binary, so they add no dependency. `prose` is for sanitized rich text only.

### 8.3 htmx (optional, Phase 7)

- Self-hosted pinned file with SHA-256 recorded.
- Configure via a `<meta name="htmx-config">` tag: eval disabled, no indicator style injection, self-requests only, no script tags in swapped content. ⟨verify current option names, R-018⟩
- The CSRF header comes from `hx-headers` on `<body>`.

---

## 9. OWASP Top 10:2025 control matrix

| Category | Controls (section) | Acceptance tests |
|---|---|---|
| **A01** Broken Access Control | policy router §4.2–4.3, owner-scoped SQL §7, SafeRedirect, SafeClient §6.9, os.Root §6.10, no auto-routing | `TestEveryRouteHasPolicy`, `TestIDOR_*`, `TestSafeRedirect`, `TestSafeClientBlocks*`, `TestUploadTraversal` |
| **A02** Security Misconfiguration | production guards §6.13, headers/CSP §6.1, no CORS by default, empty Server header | `TestProdGuards`, `TestSecurityHeaders`, `TestNoServerHeader` |
| **A03** Software Supply Chain | pinned versions §1, govulncheck, ADR per dependency, checksum-verified binaries, SHA-pinned CI actions, SBOM, distroless image | CI `vuln` job, `make tools` checksum failure test |
| **A04** Cryptographic Failures | argon2id §6.4, crypto/rand, key checks, TLS ≥ 1.2, DB verify-full | `TestPasswordHashVerify`, `TestNeedsRehash`, `TestKeyLengthGuard` |
| **A05** Injection | sqlc only §7, sort allowlist, templ escaping §8.1, validation §6.12, no shell exec | `TestSortAllowlist`, `TestXSSEscaping`, gosec in CI |
| **A06** Insecure Design | THREATS.md per module, rate limits §6.6, service-layer invariants, scaffold template | `TestRateLimitLogin`, review checklist in PR template |
| **A07** Authentication Failures | sessions §6.2, auth flows §6.4 | `TestSessionCookieFlags`, `TestSessionRotatesOnLogin`, `TestLoginGenericError`, `TestAbsoluteTimeout` |
| **A08** Software/Data Integrity | session CSRF §6.3, no gob/unsafe deserialization of client data, self-hosted assets, protected CI | `TestCSRF_*` |
| **A09** Logging & Alerting | slog, redaction, security events §6.8 | `TestSecurityEventsEmitted`, `TestLogRedaction` |
| **A10** Exceptional Conditions | ErrorHandler §6.7, recover, timeouts, fail-closed authz | `TestErrorNoLeak`, `TestPanicRecovered`, `TestBodyLimit413`, `TestTimeouts` |

---

## 10. Phased build plan

Each phase ends with `make check` green, `docs/PROGRESS.md` updated, and a commit.

**P0. Research and decisions**
- Resolve RESEARCH §5 items R-001 to R-007 and R-012/R-013.
- Create `docs/research/*`, `docs/adr/0001-stack.md`, `docs/PROGRESS.md`.
- **DoD:** every ⟨verify⟩ for P1–P3 has a note with source plus pinned version.

**P1. Skeleton**
- `go mod init`, pinned deps, Makefile (`tools` with checksums, `generate`, `css`, `lint`, `vuln`, `test`, `check`, `build`).
- `.golangci.yml`, CI workflow (lint/test/vuln/build), `cmd/server` with config + production guards, `/healthz` and `/readyz`.
- **DoD:** CI green; `TestProdGuards`.

**P2. Kernel**
- Module interface, policy router, registry, ErrorHandler, AppError, SafeRedirect, 404 handlers, boundary rule.
- **DoD:** `TestEveryRouteHasPolicy`, `TestErrorNoLeak`, `TestPanicRecovered`, `TestSafeRedirect`, `TestModuleBoundaries`, `TestUnknownConstraintRejectedAtBoot`, `TestProxyTrustScheme`, `TestProxyTrustIP`.

**P3. Security core**
- helmet, CSP nonce, request ID, slog + redaction, global limiter, session, CSRF, trust-proxy config, body limits.
- **DoD:** `TestSecurityHeaders`, `TestCSPNonce`, `TestSessionCookieFlags`, `TestCSRF_*`, `TestTrustProxySpoof`, `TestBodyLimit413`, `TestLogRedaction`.

**P4. Data**
- pgx pool, goose migrations (embedded), sqlc config, WithTx, timeouts, CI Postgres service.
- **DoD:** migration up/down test; a query-timeout test.

**P5. Views and Tailwind**
- Layout, Page, Render, form helpers, flash, error pages, Tailwind pipeline with hashed assets + manifest, static serving.
- **DoD:** `TestXSSEscaping` (framework pages), `TestAssetCacheHeaders`, CSP still clean (no violations in a headless check, optional).

**P6. Auth module (reference module)**
- Register, login, logout (POST), password service (argon2id), common-password check, auth limiter, session rotation, authz RBAC registry, security events, THREATS.md.
- **DoD:** `TestLoginGenericError`, `TestSessionRotatesOnLogin`, `TestRateLimitLogin`, `TestPasswordHashVerify`, `TestNeedsRehash`, `TestSecurityEventsEmitted`.

**P7. Blog module (HMVC demo)**
- Owner-scoped CRUD, sort allowlist, `contract.Widgets` used by the home page from another module, optional htmx delete with CSRF header, file upload with os.Root (cover image).
- **DoD:** `TestIDOR_*`, `TestSortAllowlist`, `TestUploadTraversal`, widget test, `TestXSSEscaping` (module views).

**P8. Hardening and developer experience**
- `cmd/scaffold make:module` (generates module.go/handler/service/queries.sql/views/THREATS.md/tests with a policy stub that fails until set).
- SafeClient + tests.
- Dockerfile (multi-stage, `CGO_ENABLED=0`, distroless nonroot).
- SBOM, ZAP baseline job, Renovate/Dependabot config, PR template with security checklist.
- ASVS L2 tracker filled with evidence links.
- **DoD:** ZAP baseline with no High/Medium; tracker complete with N/A justified.

---

## 11. CI pipeline (`.github/workflows/ci.yml`)

Jobs (actions pinned by commit SHA, least-privilege `permissions:`):

1. **generate-check:** `templ generate && sqlc generate && make css`, then `git diff --exit-code`.
2. **lint:** gofmt check, `go vet`, staticcheck, golangci-lint (gosec, depguard, errcheck, bodyclose, …).
3. **test:** `go test -race -cover ./...` with a Postgres service.
4. **vuln:** `govulncheck ./...`. Fails on any finding in reachable code.
5. **build:** `make build`; container build; SBOM.
6. **dast** (main branch / nightly): docker compose up, then ZAP baseline with a rules file.

---

## 12. ASVS 5.0 Level 2 tracking

`docs/asvs/ASVS-L2.md` is generated from the official ASVS 5.0 machine-readable release ⟨verify source file, R-017⟩. Columns: requirement ID, level, applicable (Y/N + reason), control (section in this doc), evidence (test name / file), status. It is updated at the end of P3, P6, and P8.

---

## 13. Deferred backlog (post-baseline)

Password reset (hashed single-use tokens), email verification, TOTP/WebAuthn MFA, OIDC login (consumer), audit-log module, CSP `report-to` endpoint, admin panel module, i18n, background jobs, multi-instance cache invalidation, HSTS preload decision.
