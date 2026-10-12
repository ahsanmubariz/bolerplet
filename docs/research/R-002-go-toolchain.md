# R-002 Go toolchain
- Date: 2026-10-05
- Pinned versions: go 1.27.1, fiber v3.5.0, govulncheck v1.8.0
- Question: What are the latest stable Go release and patch? Are there any stdlib vulns? Output: `go`/`toolchain` lines for go.mod.
- Sources (with versions/URLs):
  - https://go.dev/dl/?mode=json (2026-10-05): `go1.27.1 stable=true`, `go1.26.8 stable=true`
  - https://go.dev/doc/devel/release:
    - go1.27.0 released 2026-08-19.
    - go1.27.1 released 2026-09-01, with "fixes to cgo, the compiler, the runtime, the go fix command, and the database/sql, debug/elf, encoding/json, net/http, os, simd, and simd/archsimd packages". It lists no security fixes.
    - go1.26.8 released 2026-09-01.
  - OSV `POST https://api.osv.dev/v1/query` with `{"package":{"name":"stdlib"|"toolchain","ecosystem":"Go"},"version":…}`
  - govulncheck v1.8.0 run in `spikes/a` ("scanned … the go1.27.1 standard library")
  - `$GOMODCACHE/github.com/gofiber/fiber/v3@v3.5.0/go.mod` → `go 1.25.0`
  - Local experiment in the scratchpad: `go mod tidy` behavior for `go`/`toolchain` combinations
- Findings (facts only):
  - Latest stable is **go1.27.1**. The previous supported line is go1.26.8. Both were released 2026-09-01.
  - OSV returns **0 vulns** for `stdlib` and `toolchain` at 1.27.1, 1.27.0 and 1.26.8. As a sanity check, the same query returns 46 at 1.25.0 and 33 at 1.26.0. The most recent stdlib batch (GO-2026-5972, 6088–6091, 6218, published 2026-08-13) was fixed in 1.25.13 / 1.26.6 / 1.27.0-rc.3.
  - govulncheck v1.8.0 on go1.27.1 reports no stdlib findings.
  - fiber v3.5.0 requires **go ≥ 1.25.0**. fasthttp v1.75.0, if adopted (R-001), requires go ≥ 1.26.
  - `go mod tidy` with go1.27.1:
    - `go 1.27.1` + `toolchain go1.27.1`: tidy **removes** the toolchain line because it is redundant.
    - `go 1.27.0` + `toolchain go1.27.1`: tidy keeps both lines.
    - `go mod init` under go1.27.1 writes `go 1.27.1`.
  - With the system go1.22 and the default `GOTOOLCHAIN=auto`, a module that declares `go 1.27.1` auto-switches to go1.27.1. Verified: `go version` printed `go1.27.1`.
- Decision / impact on BASELINE.md:
  - go.mod uses:
    ```
    go 1.27.1
    ```
    No separate `toolchain` line, because tidy drops it when it equals `go`. This makes 1.27.1 the minimum for every build, so no build can use an older, unpatched stdlib. If a lower language version is ever wanted, use `go 1.27.0` + `toolchain go1.27.1` instead.
  - CI pins `actions/setup-go` to `1.27.1` (or `go-version-file: go.mod`) and sets `GOTOOLCHAIN=local` so the build fails rather than silently downloading a different toolchain. Locally, `GOTOOLCHAIN=auto` (the default) is fine.
  - BASELINE §1 wording "`go` + `toolchain` lines in go.mod" should become "`go 1.27.1` line in go.mod (a `toolchain` line only when it differs from `go`)".
- Proven by test: `go mod tidy` experiment (above), plus govulncheck in `spikes/a` (see R-001). Permanent: `make vuln` in CI.
- Re-check trigger: each Go point release (go.dev/doc/devel/release, golang-announce), any stdlib GO-ID in the weekly govulncheck run, or adopting a dependency with a higher `go` minimum.

## Addendum 2026-10-12: go1.27.2 security release
- Trigger: `make vuln` in P1 (govulncheck v1.8.0) reported 8 **reachable** vulnerabilities on go1.27.1, all called via `fiber.App.Listen` / `fiber.init`:
  - GO-2026-6603, GO-2026-6610, GO-2026-6611, GO-2026-6617: `net/http/internal/http2` (also in `golang.org/x/net` ≤ v0.59.0)
  - GO-2026-6605, GO-2026-6613: `net/http`
  - GO-2026-6607: `crypto/tls`
  - GO-2026-6608: `net/textproto`
- All are fixed in **go1.27.2** (released 2026-10-08; go.dev/dl lists `go1.27.2` and `go1.26.9` as stable) and **`golang.org/x/net` v0.60.0** (2026-10-08).
- Decision: `go 1.27.2` in go.mod and `golang.org/x/net v0.60.0` (indirect, pinned through MVS). This takes the ADR 0001 security exception to the 14-day soak. After the change, govulncheck reports 0 symbol- and package-level findings; only the known module-level GO-2026-5932 (openpgp, R-001) remains.
