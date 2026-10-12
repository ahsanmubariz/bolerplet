# R-012 templ + Fiber v3
- Date: 2026-10-12
- Pinned versions: templ v0.3.1020 (lib + CLI via `go tool`), fiber v3.5.0, go 1.27.1
- Question: What is the recommended render pattern (`BodyWriter` vs stream writer)? Can `fiber.Ctx` be passed as `context.Context`? Does templ have a CSP nonce helper? How do `templ.URL`/`SafeURL` sanitize `href`? Does `templ generate --watch` work for `make dev`?
- Sources (with versions/URLs):
  - `$GOMODCACHE/github.com/a-h/templ@v0.3.1070/url.go`, `runtime.go`, `jsonscript.go`, `generator/generator.go` (`isExpressionAttributeValueURL`)
  - https://api.github.com/repos/a-h/templ/releases: v0.3.1070 published 2026-10-04 (changelog: deps/CI chores, LSP features, `.templignore_*`; no security fixes); v0.3.1020 published 2026-05-10; v0.3.1001 2026-02-28
  - OSV query `github.com/a-h/templ` (Go): 0 vulns
  - `go tool templ generate --help` (v0.3.1070)
  - Spike `spikes/d` (`spike_test.go`, `eh_test.go`, `views/xss.templ`; gitignored), run on v0.3.1070 and again on v0.3.1020 in a scratch copy with identical results
- Findings (facts only):
  - **Ctx as context:** `var _ context.Context = fiber.Ctx(nil)` compiles. `comp.Render(c, …)` and `templ.WithNonce(c, nonce)` both accept the Ctx directly.
  - **BodyWriter render, error midway:** with `comp.Render(c, c.Response().BodyWriter())` the partial output stays in the response body.
    - If the ErrorHandler responds with `c.SendString`/`Send`, the body is **replaced** (`/fail-bw` → 500, body = error page only).
    - If the ErrorHandler renders a templ page through the same BodyWriter helper, the error page is **appended** to the partial body: `bodywriter: status=500 len=29188 partial_rows_leaked=true`.
    - Calling `c.Response().ResetBody()` on render failure fixes it: `bodywriter+reset: status=500 len=298 partial_rows_leaked=false`.
  - **Buffered render** (`bytes.Buffer` then `c.Send`) behaves correctly too, at the cost of one copy.
  - **`SendStreamWriter`:** a mid-render error is swallowed. Response is `200` with truncated HTML (`/fail-stream -> status=200 len=28890`). Unsuitable.
  - **URL sanitization:** `templ.URL(s)` returns `about:invalid#TemplFailedSanitizationURL` when `s` has a scheme (text before the first `:` that contains no `/`) other than `http`, `https`, `mailto`, `tel`, `ftp`, `ftps` (case-insensitive). Verified: `javascript:alert(1)`, ` JaVaScRiPt:alert(1)`, `data:text/html,…` are all blocked.
  - The generator applies `templ.URL` **automatically only** to string expressions in `<a href>`, `<link href>`, `<form action>`, `<object data>`.
  - **Not sanitized** (verified with `javascript:alert(1)`): `<img src>`, `<iframe src>`, `<button formaction>`, any other attribute, and **spread attributes** (`<a { templ.Attributes{"href": x}... }>` emits `href="javascript:alert(1)"`). Values are still HTML-attribute-escaped, so no attribute breakout (`"><img onerror>` becomes `&#34;&gt;&lt;img…`).
  - `//evil.example/x` passes `templ.URL` (scheme-relative). This is not XSS, but it is an off-site link.
  - **Nonce:** `templ.WithNonce(ctx, n)` / `templ.GetNonce(ctx)` exist. `templ.JSONScript` adds `nonce="…"` from context automatically and escapes `</script>` as `</script>`. A **literal `<script>` in a template does not get a nonce** (`<script>console.log("inline")</script>`); it needs `nonce={ templ.GetNonce(ctx) }`. The nonce attribute value is escaped (`a"b` becomes `a&#34;b`).
  - **Watch:** `templ generate -watch` exists (default pattern `(.+\.go$)|(.+\.templ$)|(.+_templ\.txt$)`), with `-cmd` and `-proxy` options.
  - v0.3.1020 and v0.3.1070 produced byte-identical spike output.
- Decision / impact on BASELINE.md:
  - Pin templ **v0.3.1020** (lib in `require`, CLI via the go.mod `tool` directive so both share one version). v0.3.1070 is 8 days old with no security content; adopt it after the 14-day soak (ADR 0001 policy).
  - §8.1 `Render` must change. It sets the nonce on the context and resets the body on failure:
    ```go
    func Render(c fiber.Ctx, status int, comp templ.Component) error {
    	c.Status(status)
    	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
    	ctx := templ.WithNonce(c, csp.NonceFromContext(c))
    	if err := comp.Render(ctx, c.Response().BodyWriter()); err != nil {
    		c.Response().ResetBody()
    		return err
    	}
    	return nil
    }
    ```
    Never use `SendStreamWriter` for templ pages.
  - §8.1 template rules, add:
    - Every `<script>` carries `nonce={ templ.GetNonce(ctx) }`. Prefer `templ.JSONScript` for passing data.
    - User data in any URL attribute other than `a/link href`, `form action`, `object data` (e.g. `img src`, `button formaction`) must be wrapped in `templ.URL(...)` explicitly.
    - No spread `templ.Attributes` with user-controlled keys or URL values.
    - User URLs that should stay on-site go through `httpx.SafeRedirect`-style validation, because `templ.URL` allows `//host`.
  - `TestXSSEscaping` must include `img src` / spread cases to lock in the above, and assert that every `<script` in rendered pages has the request nonce.
  - CSP `script-src 'self' 'nonce-…'` without `'unsafe-inline'` remains the backstop for `javascript:` URLs that slip through.
  - `make dev`: `go tool templ generate -watch` alongside tailwind `--watch` and the server.
- Proven by test: `TestXSSPayloads`, `TestNonce`, `TestNonceExplicit`, `TestRenderErrorMidway`, `TestErrorHandlerRendersTemplAfterPartial` in `spikes/d` (gitignored). Run: `cd spikes/d && GOTOOLCHAIN=go1.27.1 go test -count=1 -v .` → `PASS ok spiked`. Permanent equivalents in P5: `TestRenderResetsBodyOnError`, `TestXSSEscaping`, `TestCSPNonce` (header nonce == every script's nonce).
- Re-check trigger: any templ upgrade (re-run the XSS/nonce cases; diff `url.go` and `isExpressionAttributeValueURL`); any fiber change to `Response().BodyWriter`/`ResetBody`.
