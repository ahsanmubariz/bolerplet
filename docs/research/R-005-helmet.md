# R-005 helmet
- Date: 2026-10-05
- Pinned versions: fiber v3.5.0, go 1.27.1 (toolchain), fasthttp v1.73.0, gofiber/utils/v2 v2.4.1
- Question: Config fields and defaults of `middleware/helmet` at the pinned version; confirm the HSTS fix (CVE-2026-53624 / GHSA-gv83-gqw6-9j2c / GO-2026-5912) and how HSTS emission is decided; how to suppress the CSP header so the custom csp middleware owns it; PermissionPolicy field name and default.
- Sources (with versions/URLs):
  - Source at pin: `$GOMODCACHE/github.com/gofiber/fiber/v3@v3.5.0/middleware/helmet/{config.go,helmet.go}`; `ctx.go:607` (`Secure`); `req.go:1049` (`Scheme`), `req.go:1410` (`IsProxyTrusted`); `helpers.go:1138` (`testConn.RemoteAddr` = `0.0.0.0:0`); `constants.go:243`.
  - Pre-fix comparison: `fiber/v3@v3.3.0/middleware/helmet/helmet.go:67` vs `@v3.4.0` same line.
  - OSV: https://api.osv.dev/v1/vulns/GO-2026-5912 (aliases CVE-2026-53624, GHSA-gv83-gqw6-9j2c; fixed 3.4.0; fix PR https://github.com/gofiber/fiber/pull/4389). OSV query for fiber/v3 on 2026-10-05: no other helmet advisory.
- Findings (facts only):
  - **Config fields and defaults (v3.5.0)**:

    | Field | Type | Default | Header |
    |---|---|---|---|
    | `Next` | `func(fiber.Ctx) bool` | nil | — |
    | `XSSProtection` | string | `"0"` | X-XSS-Protection |
    | `ContentTypeNosniff` | string | `"nosniff"` | X-Content-Type-Options |
    | `XFrameOptions` | string | `"SAMEORIGIN"` | X-Frame-Options |
    | `ContentSecurityPolicy` | string | `""` (no header) | Content-Security-Policy |
    | `CSPReportOnly` | bool | false | switches CSP to Content-Security-Policy-Report-Only |
    | `ReferrerPolicy` | string | `"no-referrer"` | Referrer-Policy |
    | `PermissionPolicy` | string | `""` (no header) | Permissions-Policy |
    | `CrossOriginEmbedderPolicy` | string | `"require-corp"` | Cross-Origin-Embedder-Policy |
    | `CrossOriginOpenerPolicy` | string | `"same-origin"` | Cross-Origin-Opener-Policy |
    | `CrossOriginResourcePolicy` | string | `"same-origin"` | Cross-Origin-Resource-Policy |
    | `OriginAgentCluster` | string | `"?1"` | Origin-Agent-Cluster |
    | `XDNSPrefetchControl` | string | `"off"` | X-DNS-Prefetch-Control |
    | `XDownloadOptions` | string | `"noopen"` | X-Download-Options |
    | `XPermittedCrossDomain` | string | `"none"` | X-Permitted-Cross-Domain-Policies |
    | `HSTSMaxAge` | int (seconds) | 0 (no header) | Strict-Transport-Security |
    | `HSTSExcludeSubdomains` | bool | false (so `includeSubDomains` is ON whenever MaxAge > 0) | |
    | `HSTSPreloadEnabled` | bool | false | adds `; preload` |

  - **Empty string means "use the default", not "omit"** for every string field except `ContentSecurityPolicy` and `PermissionPolicy` (`configDefault` refills empty values). Those two are the only string headers that can be suppressed through config; HSTS is suppressed by `HSTSMaxAge: 0`.
  - `configDefault` panics if `HSTSMaxAge < 0`, or if `HSTSPreloadEnabled && HSTSExcludeSubdomains`.
  - The HSTS value is built once at `New()`: `max-age=N` + `; includeSubDomains` unless excluded + `; preload` if enabled.
  - **HSTS decision (fix confirmed)**: v3.5.0 sets HSTS only when `c.Secure() && hstsHeaderValue != ""`. `c.Secure()` is `c.Scheme() == "https"`. v3.3.0 checked `c.Protocol() == "https"` (`Protocol()` returns the HTTP version, e.g. `HTTP/1.1`, so HSTS was never sent, which is the CVE). v3.4.0 introduced `c.Secure()`, and v3.5.0 keeps it.
  - **`c.Scheme()`** (req.go:1049): returns `https` if the fasthttp conn is TLS. Otherwise, if `!IsProxyTrusted()`, it returns `http` and ignores every header. If the peer is a trusted proxy (`TrustProxy: true` and the remote IP matches `TrustProxyConfig.Proxies`, or the Loopback/Private/LinkLocal/UnixSocket flags), it reads `X-Forwarded-Proto` / `X-Forwarded-Protocol` (only the **first** comma-separated value; only `http`/`https` are accepted, anything else is ignored), `X-Forwarded-Ssl: on`, and `X-Url-Scheme`. When several of these headers are present, the last one iterated wins.
  - So HSTS is tied to the trusted-proxy config. Behind a TLS-terminating proxy with no `TRUSTED_PROXIES`, HSTS is **silently never sent**. A proxy that *appends* to `X-Forwarded-Proto` (`<client value>, https`) lets the client choose the scheme, because the first value wins. The proxy must overwrite `X-Forwarded-Proto` and strip `X-Forwarded-Ssl`, `X-Forwarded-Protocol` and `X-Url-Scheme`.
  - Helmet writes headers with `c.Set` **before** `c.Next()`. A later middleware (our csp) that calls `c.Set(Content-Security-Policy, …)` is the only writer when `ContentSecurityPolicy == ""`.
  - Permissions-Policy header name is `fiber.HeaderPermissionsPolicy` = `"Permissions-Policy"`; the field is **`PermissionPolicy`** (singular, no "s").
  - `app.Test` connections report RemoteAddr `0.0.0.0:0`, so tests that exercise a trusted proxy use `TrustProxyConfig{Proxies: []string{"0.0.0.0"}}`.
- Spike results (all pass, `go test -race`):
  - `ContentSecurityPolicy: ""` produces no `Content-Security-Policy` or `-Report-Only` header from helmet. With a downstream csp stand-in there is exactly one CSP header (ours).
  - HSTS is absent on plain http, absent with a spoofed `X-Forwarded-Proto: https` when `TrustProxy` is off, and absent when the peer is not in `Proxies`.
  - HSTS is present (`max-age=63072000; includeSubDomains`) in two setups. **Both work**: (a) trusted proxy plus `X-Forwarded-Proto: https` (also `X-Forwarded-Ssl: on` and `X-Url-Scheme: https`) via `app.Test`; (b) a real `tls.Listen` listener passed to `app.Listener` with no proxy config.
  - `X-Forwarded-Proto: "https, http"` gives HSTS; `"http, https"` gives none (first value wins); a junk value gives none.
- Decision / impact on BASELINE.md:
  - §6.1 is confirmed with corrections. Use `helmet.Config{HSTSMaxAge: 63072000, HSTSExcludeSubdomains: false, HSTSPreloadEnabled: false, ContentSecurityPolicy: "", XFrameOptions: "DENY", PermissionPolicy: "camera=(), microphone=(), geolocation=(), payment=()"}`. `XFrameOptions: "DENY"` aligns with CSP `frame-ancestors 'none'`; the default is SAMEORIGIN, not DENY.
  - PermissionPolicy has **no default**, so it must be set explicitly or no header is sent.
  - The COEP `require-corp` default blocks any cross-origin subresource that lacks CORP/CORS. This is consistent with the "no third-party CDN assets" rule, but any future cross-origin image needs a deliberate override.
  - §6.13: the production guard "HSTS is disabled" means `HSTSMaxAge <= 0`. Add a guard: in production, either TLS is terminated in-process or `TRUSTED_PROXIES` is non-empty. Otherwise `c.Scheme()` is never https and HSTS is never emitted.
  - §5.1 / R-004 deployment note: the reverse proxy MUST overwrite `X-Forwarded-Proto` and strip `X-Forwarded-Ssl` / `X-Forwarded-Protocol` / `X-Url-Scheme` from clients.
  - Pin ≥ v3.4.0 is required for HSTS. The current pin v3.5.0 contains the fix.
- Proven by test: `TestHelmetDefaults`, `TestHelmetCSPSuppressed`, `TestHelmetHSTSScheme` (table: http / spoofed / untrusted / trusted XFP / X-Forwarded-Ssl / X-Url-Scheme / first-value-wins), `TestHelmetHSTSPreloadValue`, `TestHelmetPreloadWithExcludePanics`, `TestHelmetHSTSRealTLS` in `spikes/b/helmet_test.go` (spike module `spikeb`, gitignored). Port to `framework/security/headers` tests in P3 (lock-in, RESEARCH §3.6).
- Re-check trigger: any fiber minor/patch upgrade (helmet `config.go`/`helmet.go`, `req.go` `Scheme`/`IsProxyTrusted`); any new GHSA for helmet or proxy-header handling; change of reverse proxy.
