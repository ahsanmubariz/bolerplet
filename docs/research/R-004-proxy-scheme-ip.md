# R-004 Scheme and IP behind a proxy
- Date: 2026-10-05
- Pinned versions: fiber v3.5.0, fasthttp v1.73.0, go 1.27.1
- Question: Does `c.Scheme()` honor `X-Forwarded-Proto` only from trusted proxies? How does `c.IP()` behave with `EnableIPValidation` + `TrustProxyConfig`? This decides HSTS emission and the limiter key. Output: a test with a trusted and an untrusted proxy.
- Sources (with versions/URLs):
  - `$GOMODCACHE/github.com/gofiber/fiber/v3@v3.5.0/req.go`:
    - `Host` at 600
    - `IP` at 649
    - `extractIPsFromHeader` at 663
    - `extractIPFromHeader` at 726
    - `hasTrustedProxyConfig` at ~800
    - `isTrustedProxyIP` at 808
    - `IPs` at 859
    - `Scheme` at 1049
    - `forwardedScheme`
    - `IsProxyTrusted` at 1410
  - Also:
    - `ctx.go:607`: `Secure()` returns `c.Scheme() == "https"`
    - `middleware/helmet/helmet.go:84`: HSTS is sent only `if c.Secure()`
    - `middleware/limiter/config.go:103`: the default `KeyGenerator` is `c.IP()`
    - `app.go:857`: `handleTrustedProxy`
    - `helpers.go:1138`: `testConn.RemoteAddr()` is always `0.0.0.0:0`
  - `go doc github.com/gofiber/fiber/v3.Config` (TrustProxy doc comment)
- Findings (facts only):
  - **Setting the remote address in tests:** `app.Test` always reports `RemoteAddr` 0.0.0.0, so it cannot exercise trust. Instead, the spike calls `app.Handler()` on a `fasthttp.RequestCtx` initialised with `fctx.Init(&req, &net.TCPAddr{IP: …}, nil)`. It also uses a real listener on `127.0.0.1:0` via `app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})`.
  - **Trust decision (`IsProxyTrusted`).** It returns `false` unless `Config.TrustProxy` is set. For TCP/UDP peers, `RemoteIP()` must match one of:
    - `Loopback`, `Private` or `LinkLocal` (when those flags are set)
    - an exact entry in the `Proxies` IP map
    - a `Proxies` CIDR

    A Unix-socket peer is trusted only if `UnixSocket` is set. Unknown address types are never trusted.
  - **`c.Scheme()`** returns:
    - `https` if the connection itself is TLS
    - otherwise `http` when the peer is **not** a trusted proxy, whatever headers it sends
    - otherwise (trusted peer) the value of the forwarding headers below, or `http` if none applies

    For a trusted peer, Scheme walks **all** request headers in wire order:
    - `X-Forwarded-Proto` / `X-Forwarded-Protocol`: takes the **leftmost** comma element.
    - `X-Forwarded-Ssl: on`: sets https.
    - `X-Url-Scheme`

    Only `http`/`https` (case-insensitive) are accepted; other values are ignored. **The last matching header line wins.** RFC 7239 `Forwarded: proto=` is **not** consulted.
  - **`c.IP()`:**
    - If the peer is a trusted proxy **and** `ProxyHeader != ""`, it extracts the IP from the header. Otherwise it returns `RemoteIP()`.
    - Repeated header lines are joined (RFC 9110 §5.2).
    - With `EnableIPValidation=false`, it returns the **raw header string verbatim**, for example `"6.6.6.6, 203.0.113.7"` or `"<script>"`.
    - With `EnableIPValidation=true` and a trust config that has `Proxies`, `Loopback`, `Private` or `LinkLocal`, it walks right-to-left, skips invalid entries and trusted IPs, and returns the **rightmost untrusted valid IP**. If every valid entry is trusted, it returns the **leftmost valid** entry. If no entry is valid, it returns `RemoteIP()`.
    - With `EnableIPValidation=true` but **no** such trust config, it returns the **leftmost valid IP**, which is spoofable. This happens with `UnixSocket: true` and nothing else.
  - **`c.IPs()`** returns the `X-Forwarded-For` list **regardless of trust**, so a client can spoof it. **`c.Host()`/`Hostname()`** honor `X-Forwarded-Host` (leftmost element) only from a trusted peer.
  - Invalid `TrustProxyConfig.Proxies` entries are only logged and dropped. Exact-IP entries are matched against `ip.String()`, so they must be written in canonical form; CIDRs avoid this pitfall.
  - **Spike results** from `go test -race -count=1 -v ./...` (all PASS; full log in `spikes/a/test-output.txt`). Config: `TrustProxy=true`, `Proxies=[10.0.0.0/8]`, `ProxyHeader=X-Forwarded-For`, `EnableIPValidation=true`. Trusted peer 10.0.0.5, untrusted peer 203.0.113.9.

    | Case | Result |
    |---|---|
    | trusted + `XFP: https` | `https` |
    | untrusted + `XFP: https` | `http` |
    | `TrustProxy=false`, peer 10.0.0.5 + `XFP: https` | `http` |
    | trusted + `X-Forwarded-Protocol: https` / `X-Forwarded-Ssl: on` / `X-Url-Scheme: https` | `https` each |
    | untrusted + `X-Forwarded-Ssl: on` | `http` |
    | trusted + `XFP: https, http` | `https` (leftmost element) |
    | trusted + two lines `XFP: https` then `XFP: http` | `http` (last line wins); reversed order gives `https` |
    | trusted + `XFP: http` + `X-Forwarded-Ssl: on` | `https` |
    | trusted + `XFP: javascript` | `http` |
    | trusted + `Forwarded: proto=https` | `http` |
    | trusted + `XFF: 1.1.1.1, 203.0.113.7, 10.0.0.9` | IP `203.0.113.7` |
    | trusted + `XFF: 6.6.6.6, 203.0.113.7` (spoofed left) | `203.0.113.7` |
    | trusted + `XFF: evil, 203.0.113.7, <x>` | `203.0.113.7` |
    | trusted + `XFF: 10.0.0.1, 10.0.0.2` (all trusted) | `10.0.0.1` |
    | trusted + `XFF: garbage` / no XFF | `10.0.0.5` (RemoteIP) |
    | trusted + two XFF lines `6.6.6.6` / `203.0.113.7` | `203.0.113.7` |
    | untrusted + `XFF: 1.1.1.1` | `203.0.113.9` |
    | `EnableIPValidation=false`, trusted + `XFF: 6.6.6.6, 203.0.113.7` | `"6.6.6.6, 203.0.113.7"` (raw) |
    | `EnableIPValidation=false`, trusted + `XFF: <script>` | `"<script>"` |
    | `ProxyHeader=""`, trusted + XFF | `10.0.0.5` |
    | `UnixSocket`-only trust, unix peer + `XFF: 6.6.6.6, 203.0.113.7` | `6.6.6.6` (spoofable) |
    | untrusted + `XFF: 6.6.6.6, 7.7.7.7` → `c.IPs()` | `["6.6.6.6" "7.7.7.7"]` |
    | `X-Forwarded-Host: evil.example` trusted / untrusted | `evil.example` / `app.example` |
    | real listener 127.0.0.1, `Loopback: true`, `XFP: https`, `XFF: 6.6.6.6, 203.0.113.7` | `https`, IP `203.0.113.7` |
    | real listener 127.0.0.1, only 10/8 trusted, same headers | `http`, IP `127.0.0.1` |
- Decision / impact on BASELINE.md:
  - **Trust works as intended.** `c.Scheme()` and `c.IP()` honor forwarding headers **only** when the TCP peer is a configured trusted proxy. So HSTS (helmet uses `c.Secure()`, which uses `c.Scheme()`) and the limiter key (default `c.IP()`) are safe when two conditions hold. First, `EnableIPValidation=true`. Second, trust is given by `Proxies` CIDRs.
  - BASELINE §5.1, `EnableIPValidation` row: change "rightmost-untrusted XFF parsing" to "**MUST be true**: without it `c.IP()` returns the raw `X-Forwarded-For` string (client-controlled, unvalidated). With it, `c.IP()` returns the rightmost untrusted valid IP".
  - BASELINE §5.1, `TrustProxyConfig.Proxies` row: add "`Loopback`, `Private`, `LinkLocal`, `UnixSocket` stay `false`. Boot fails on any unparsable entry, because Fiber silently drops it."
  - BASELINE §6.13, new production guards:
    - `TRUSTED_PROXIES` entries must parse.
    - It must not contain `0.0.0.0/0` or `::/0` (already listed).
    - `UnixSocket` trust is not supported without `Proxies`, because the IP becomes leftmost and spoofable.
    - `TrustProxy` implies `EnableIPValidation`.
  - **Deployment requirement** (BASELINE §5 / ops docs). The trusted proxy MUST **overwrite** `X-Forwarded-Proto` rather than append to it, because Fiber reads the leftmost element and the last line. It MUST also **strip** client-supplied `X-Forwarded-Protocol`, `X-Forwarded-Ssl`, `X-Url-Scheme` and `X-Forwarded-Host`. Otherwise a client behind a trusted proxy can flip the scheme or host. The proxy MUST append the peer address to `X-Forwarded-For`.
  - Code rule: never use `c.IPs()` for security decisions such as rate limiting, audit or allowlists. Use `c.IP()` only. Never build absolute URLs from `c.Host()`/`c.BaseURL()`; use `BASE_URL` from config.
  - §6.1 test "HSTS is present when the scheme is https, including behind a trusted proxy" is feasible with the `Handler()` + `RequestCtx.Init` technique. The permanent test must also assert that HSTS is **absent** for an untrusted peer that sends `X-Forwarded-Proto: https`.
- Proven by test:
  - In `spikes/a/proxy_test.go`: `TestSchemeTrustedVsUntrusted`, `TestIPTrustedVsUntrusted`, `TestIPsIgnoresTrust`, `TestHostTrustedVsUntrusted`, `TestRealListenerLoopback`.
  - Port these to `framework/kernel` (or `framework/httpx`) in P1 as `TestProxyTrustScheme` / `TestProxyTrustIP`.
- Re-check trigger: any fiber upgrade that touches `req.go` (`Scheme`, `IP`, `IsProxyTrusted`, `extractIPFromHeader`); any change to proxy topology or `TRUSTED_PROXIES` semantics.
