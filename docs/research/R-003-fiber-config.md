# R-003 fiber.Config and ListenConfig fields
- Date: 2026-10-05
- Pinned versions: fiber v3.5.0, go 1.27.1, fasthttp v1.73.0
- Question: What are the exact names, types and defaults at the pinned version for ServerHeader, BodyLimit, Read/Write/IdleTimeout, Immutable, ErrorHandler, TrustProxy, TrustProxyConfig, ProxyHeader, EnableIPValidation, SkipUnmatchedRoutes, PassLocalsToContext, DisableHeadAutoRegister, ListenConfig.DisableStartupMessage and ListenConfig.TLSMinVersion? Also needed: the ErrorHandler/Handler signatures, whether `fiber.Ctx` is an interface that implements `context.Context`, the shape of `fiber.Router`, the route-constraint syntax, and the custom-constraint API. Output: BASELINE §5.1 table confirmed or corrected.
- Sources (with versions/URLs):
  - `go doc github.com/gofiber/fiber/v3.{Config,TrustProxyConfig,ListenConfig,ErrorHandler,Handler,Router,CustomConstraint,ConstraintHandler,DefaultErrorHandler}` at v3.5.0
  - `$GOMODCACHE/github.com/gofiber/fiber/v3@v3.5.0/`:
    - `app.go`: `New()` defaults at line 695ff, `handleTrustedProxy` at 857, `RegisterCustomConstraint` at 888, `Test` at 1391, `DefaultErrorHandler`
    - `listen.go`: `listenConfigDefault` at 169, `validateTLSMinVersion` at 200, `Listen` at 214
    - `ctx.go`: 136–178 and `Value` at 661
    - `ctx_interface_gen.go`
    - `adapter.go`: `toFiberHandler`, `collectHandlers`
    - `constraint.go`, `path.go` (75–80, 455–535), `router.go` (1023), `group.go`
- Findings (facts only):
  - **Corrected BASELINE §5.1 table** (every field exists at v3.5.0 under exactly this name):

    | Field | Type at v3.5.0 | Fiber default | Baseline value | Why |
    |---|---|---|---|---|
    | `ServerHeader` | `string` | `""` (no `Server` header is sent) | `""` (set explicitly) | no fingerprinting |
    | `BodyLimit` | `int` | `4 * 1024 * 1024`; `<= 0` falls back to the default | max allowed upload size (default 4 MiB) | DoS (see R-022) |
    | `ReadTimeout` / `WriteTimeout` / `IdleTimeout` | `time.Duration` | `0` = unlimited; `IdleTimeout == 0` uses `ReadTimeout` | 10s / 15s / 60s | slowloris, A10. `ReadTimeout` covers the full body read, so revisit with R-022/R-023 for large uploads |
    | `Immutable` | `bool` | `false` | `true` | Ctx strings stay valid after the handler returns |
    | `ErrorHandler` | `fiber.ErrorHandler` = `func(fiber.Ctx, error) error` | `fiber.DefaultErrorHandler`, which **sends `err.Error()` to the client** for non-`*fiber.Error` errors | `httpx.ErrorHandler` | A10; the default leaks internals |
    | `TrustProxy` | `bool` | `false` | `len(cfg.TrustedProxies) > 0` | |
    | `TrustProxyConfig` | `fiber.TrustProxyConfig{Proxies []string; LinkLocal, Loopback, Private, UnixSocket bool}` | `fiber.DefaultTrustProxyConfig` (all empty/false) | `Proxies` from `TRUSTED_PROXIES` (IPs or CIDRs), never `0.0.0.0/0`/`::/0`; `Loopback`/`LinkLocal`/`Private`/`UnixSocket` = `false` | IP spoofing. **Fiber only `log.Warnf`s and drops unparsable entries**, so `config.Validate()` must parse every entry and fail boot on error |
    | `ProxyHeader` | `string` | `""` | `fiber.HeaderXForwardedFor` only when proxies are configured, else `""` | |
    | `EnableIPValidation` | `bool` | `false` | `true` (MUST) | Enables rightmost-untrusted XFF parsing. **With `false`, `c.IP()` returns the raw header verbatim** (R-004) |
    | `SkipUnmatchedRoutes` | `bool` | `false` | `false` (set explicitly) | `true` answers 404/405 before any middleware runs, so the limiter, access log and `Use`-based 404 handler are bypassed |
    | `PassLocalsToContext` | `bool` | `false` | `false` | Only controls whether `fiber.StoreInContext` also writes to `c.Context()` |
    | `DisableHeadAutoRegister` | `bool` | `false` | `false` | HEAD is auto-registered for each GET |
    | `ListenConfig.DisableStartupMessage` | `bool` | `false` | `true` in production | |
    | `ListenConfig.TLSMinVersion` | `uint16` | `tls.VersionTLS12` (also when set to `0`) | `tls.VersionTLS12` (or `VersionTLS13`) when terminating TLS in-process | A04. Only TLS 1.2/1.3 accepted: any other value **panics** in `Listen`. **Ignored when `ListenConfig.TLSConfig` is set** (set `MinVersion` there); also ignored by `app.Listener(ln)` |

    Other `ListenConfig` defaults worth knowing:
    - `ListenerNetwork` defaults to `"tcp4"`, so **IPv6 is not listened on unless set to `"tcp"`**.
    - `ShutdownTimeout` defaults to `10s`.
    - `ReadBufferSize` defaults to `4096`. It caps the total header size, so large cookies get rejected.
  - **Signatures:**
    - `type Handler = func(Ctx) error`
    - `type ErrorHandler = func(Ctx, error) error`

    Both are type aliases.
  - **`fiber.Ctx` is an interface** (`ctx_interface_gen.go`). The default implementation is `*fiber.DefaultCtx`. `fiber.CustomCtx` extends it.
  - **`fiber.Ctx` satisfies `context.Context`** (a compile-time assertion passes), but:
    - `Deadline()` returns `(zero, false)`, and `Done()` returns `nil`, so it is **never cancelled**. `Err()` returns `nil`. `Value(key)` reads the fasthttp user values (Locals).
    - Ctx is pooled and reused. Its own doc says: "Pass Context() to anything that is cancellation-aware or that outlives the handler."
    - `c.Context()` returns the context set via `c.SetContext`, or `context.Background()` (verified in `TestCtxIsContext`). It carries **no deadline and no client-disconnect cancellation** by default.
  - **`fiber.Router` interface** (implemented by `*fiber.App` and `*fiber.Group`):
    - `Use(args ...any) Router`
    - `Get/Head/Post/Put/Delete/Connect/Options/Trace/Patch/Query(path string, handler any, handlers ...any) Router`
    - `Add(methods []string, path string, handler any, handlers ...any) Router`
    - `All(path string, handler any, handlers ...any) Router`
    - `Group(prefix string, handlers ...any) Router`
    - `Domain(host string) Router`
    - `RouteChain(path string) Register`
    - `Route(prefix string, fn func(router Router), name ...string) Router`
    - `Name(name string) Router`

    Notes:
    - Handlers are typed `any`. `toFiberHandler` accepts `fiber.Handler`, `func(Ctx)`, ten Express-style `func(Req, Res, …)` forms, `net/http` handlers, and `fasthttp.RequestHandler` / `func(*fasthttp.RequestCtx) error`. Any other type **panics at registration** (`collectHandlers`).
    - There is no `Static` method (static is middleware), and no `Mount` (mounting is `Use(prefix, *fiber.App)`).
    - `Query` is the new HTTP QUERY method. It is in `DefaultMethods`.
  - **Route constraint syntax:**
    - `/:name<constraint>`, with several constraints separated by `;` and arguments in `(…)` separated by `,`. Examples: `/:id<int>`, `/:s<minLen(3);maxLen(20)>`. Only `<int>` and custom `<ulid>` were exercised in the spike.
    - Built-ins: `int`, `bool`, `float`, `alpha`, `guid`, `datetime`, `minLen`/`minlen`, `maxLen`/`maxlen`, `len`, `betweenLen`/`betweenlen`, `min`, `max`, `range`, `regex`. The regex engine is `Config.RegexHandler` (default `regexp.MustCompile`).
    - A failing constraint yields 404 (verified: `/n/abc` on `/n/:id<int>` gives 404).
  - **Custom constraint API:**
    - Register with `func (app *App) RegisterCustomConstraint(constraint fiber.CustomConstraint)`, where `type CustomConstraint interface { Name() string; Execute(param string, args ...string) bool }`.
    - An implementation may also implement `ConstraintAnalyzer{ Analyze(args []string) ([]any, error) }`.
    - Custom names take priority over built-ins.
    - Constraints are resolved when the route is registered (`router.go:1023`, using `app.customConstraints`).
  - **Fail-open hazards (verified in `TestConstraints`):**
    - An **unknown or misspelled constraint name is silently dropped**. `path.go:527` does `if handler == nil { continue }`, so `/x/:id<itn>` matches `abc` with 200.
    - A custom constraint **registered after the route is ignored**: `/u/nope` returns 200.
- Decision / impact on BASELINE.md:
  - BASELINE §5.1: replace the table with the corrected table above. Add rows for `SkipUnmatchedRoutes: false` and `ListenConfig.ListenerNetwork` (`"tcp"` if IPv6 is needed). Change `TrustProxy` to `len(cfg.TrustedProxies) > 0`. Mark `EnableIPValidation: true` as MUST. Note that `TLSMinVersion` is ignored when `TLSConfig` is used.
  - BASELINE §4.2:
    - The kernel `Router` keeps `fiber.Handler`-typed parameters. This is deliberately stricter than Fiber's `any`, which only fails at registration with a panic.
    - The "NOT exposed" list should read: `Use, All, Add, Route, RouteChain, Domain, Group (raw), Query, Connect, Trace` (there is no `Static`/`Mount` method in v3).
    - Custom constraints (`ulid`) MUST be registered in the kernel before any module's `Routes()`.
    - The kernel MUST reject at boot any route whose `<…>` names a constraint that is neither built-in nor registered: walk the parsed pattern, or keep an allowlist and check it in the Registry. Otherwise a typo fails open.
  - BASELINE §6.13: add boot guards. Each entry in `TRUSTED_PROXIES` must parse as an IP or CIDR. `TrustProxyConfig.UnixSocket` must not be used without `Proxies` (see R-004).
  - CLAUDE.md "`context` from `fiber.Ctx` passed to DB calls" should be refined to: "pass `c.Context()` (not `c`) to DB and other cancellation-aware calls, wrapped in a timeout. Never let `c` escape the handler."
- Proven by test: `TestConfigDefaults`, `TestNoServerHeaderByDefault`, `TestConstraints`, `TestCtxIsContext` and the compile-time assertions in `spikes/a/config_test.go`. Run with `go test -race -count=1 -v ./...` (all PASS; output in `spikes/a/test-output.txt`). Permanent tests to add in P1:
  - `TestNoServerHeader`
  - a config snapshot test for the §5.1 values
  - `TestUnknownConstraintRejectedAtBoot`
- Re-check trigger: any fiber minor or patch upgrade (re-run `go doc fiber.Config` / `ListenConfig` diff and `TestConstraints`).
