# R-001 Fiber version and advisories
- Date: 2026-10-05
- Pinned versions: fiber v3.5.0, go 1.27.1, fasthttp v1.73.0, gofiber/utils/v2 v2.4.1, gofiber/schema v1.8.3, govulncheck v1.8.0
- Question: What is the latest Fiber v3 release, and which advisories (GHSA/OSV/govulncheck) affect it and its transitive dependencies? Output: pinned version plus the list of advisories checked. Must be ≥ v3.5.0.
- Sources (with versions/URLs):
  - `go list -m -versions github.com/gofiber/fiber/v3` (2026-10-05): `v3.0.0-beta.2 … v3.0.0 v3.1.0 v3.2.0 v3.3.0 v3.4.0 v3.5.0`
  - https://api.github.com/repos/gofiber/fiber/releases: v3.5.0 published 2026-08-13T07:24:30Z (latest v3); v3.4.0 2026-07-02; v3.3.0 2026-05-22; v3.2.0 2026-04-25
  - `$GOMODCACHE/github.com/gofiber/fiber/v3@v3.5.0/go.mod`
  - OSV: `POST https://api.osv.dev/v1/query` for `github.com/gofiber/fiber/v3`, `github.com/valyala/fasthttp`, `github.com/gofiber/utils/v2`, `github.com/gofiber/schema` (Go ecosystem); `GET https://api.osv.dev/v1/vulns/GO-2026-4950`
  - GitHub advisories: https://api.github.com/repos/gofiber/fiber/security-advisories, …/gofiber/utils/…, …/gofiber/schema/…, …/valyala/fasthttp/… (the `gh` CLI is not installed, so these were fetched with curl)
  - fasthttp release notes v1.74.0 (2026-09-07) and v1.75.0 (2026-10-05): https://github.com/valyala/fasthttp/releases
  - Module proxy `@latest`: fasthttp v1.75.0, gofiber/utils/v2 v2.6.1, gofiber/schema v1.8.8, golang.org/x/crypto v0.57.0
  - `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` (and `-show verbose`) on `spikes/a/main.go`
- Findings (facts only):
  - **Latest Fiber v3 is v3.5.0** (2026-08-13). No newer tag exists as of 2026-10-05.
  - **Transitive versions pulled by fiber v3.5.0** (its go.mod declares `go 1.25.0`): `github.com/valyala/fasthttp v1.73.0`, `github.com/gofiber/utils/v2 v2.4.1`, `github.com/gofiber/schema v1.8.3`, `github.com/google/uuid v1.6.0`, `github.com/tinylib/msgp v1.6.4`, `github.com/philhofer/fwd v1.2.0`, `github.com/mattn/go-colorable v0.1.15`, `github.com/mattn/go-isatty v0.0.24`, `github.com/valyala/bytebufferpool v1.0.0`, `github.com/andybalholm/brotli v1.2.2`, `github.com/klauspost/compress v1.19.2`, `golang.org/x/crypto v0.54.0`, `golang.org/x/net v0.57.0`, `golang.org/x/sys v0.47.0`, `golang.org/x/text v0.40.0`. These are exactly the 16 non-root modules govulncheck scanned. `go.sum` also carries some test-only modules that are not compiled into the binary. `go mod why -m` shows that `github.com/shamaton/msgpack/v3 v3.2.0` and `github.com/fxamacker/cbor/v2 v2.9.2` come only via `fiber/v3.test`, and `github.com/xyproto/randomstring v1.0.5` only via `brotli.test`.
  - **Every advisory for `github.com/gofiber/fiber/v3` (OSV + GHSA):**

    | GO ID | GHSA | CVE | Summary | Fixed (v3) | v3.5.0 affected? |
    |---|---|---|---|---|---|
    | GO-2026-4534 | GHSA-2mr3-m5q5-wgp6 | CVE-2026-25899 | DoS via flash-cookie unbounded allocation | 3.1.0 | No |
    | GO-2026-4540 | GHSA-m3c2-496v-cw3v | CVE-2026-25891 | Arbitrary file read in static middleware (Windows) | 3.1.0 | No |
    | GO-2026-4543 | GHSA-mrq8-rjmw-wpq3 | CVE-2026-25882 | DoS via route parameter overflow (v2 fixed 2.52.12) | 3.1.0 | No |
    | GO-2026-5069 | GHSA-35hp-hqmv-8qg8 | CVE-2026-30246 | Cache middleware key ignores query string | 3.2.0 | No |
    | GO-2026-5585 | GHSA-qjv7-627w-8qjv | CVE-2026-42554 | XSS via `AutoFormat` content negotiation (v2 fixed 2.52.13) | **3.2.0** | No |
    | GO-2026-5886 | GHSA-g5vh-55hw-rxm8 | CVE-2026-44332 | BasicAuth default authorizer timing oracle | 3.3.0 | No |
    | GO-2026-5887 | GHSA-gcfq-8gqf-4876 | CVE-2026-45045 | X-Real-IP spoofing in proxy `BalancerForward` (v2 fixed 2.52.14) | 3.3.0 | No |
    | GO-2026-5912 | GHSA-gv83-gqw6-9j2c | CVE-2026-53624 | helmet never set HSTS (bad protocol check) | 3.4.0 | No |
    | — | GHSA-c5qf-v26h-rp5r | — | idempotency MemoryLock leaks a mutex per key (DoS). GHSA lists only v2 ≤ 2.52.14 (fixed 2.52.15), with no v3 range and no OSV entry. | n/a | Not listed for v3; we do not use idempotency |

    The older GHSAs in the repo (GHSA-68rr-p4fp-j59v, GHSA-qx2q-88mx-vhg7, GHSA-hg3g-gphw-5hhm, GHSA-98j2-3j3p-fw2v, GHSA-fmg4-x8pw-hjhg, GHSA-mv73-f69x-444p, GHSA-94w9-97p3-p368, GHSA-3q5p-3558-364f, GHSA-9cx9-x2gp-9qvh) affect only `fiber` v1 or `fiber/v2`.
  - **fasthttp v1.73.0:** GO-2022-0355 / GHSA-fx95-883v-4q4h / CVE-2022-21221 (path traversal, fixed 1.34.0) does not affect it. GO-2026-4950 (authorization bypass via double-encoded paths in `ServeFile`/`ServeFS`/`SendFile`, fixed 1.70.0) does not affect it. The GitHub repo has no published advisories.
  - **fasthttp has newer, unadvertised hardening.** v1.74.0 rejects `Transfer-Encoding` on HTTP/1.0 and rejects out-of-range `Content-Length`. v1.75.0 (released 2026-10-05) prevents cookie injection in `RequestHeader.SetCookie`, preserves framing for bodyless responses, bounds zstd decoder memory, and makes path normalization linear-time. It requires Go ≥ 1.26. Neither release has a GHSA or OSV ID. From v1.74.0, fasthttp also replaces `andybalholm/brotli` with a new dependency, `github.com/molecule-man/go-brrr`. A scratch copy of this spike passed `go test ./...` with fiber v3.5.0 on fasthttp v1.74.0 and on v1.75.0.
  - **gofiber/utils/v2 v2.4.1:** GO-2025-4208 / GHSA-m98w-cqp3-qcqr / CVE-2025-66565 (UUID silently falls back to predictable values) was fixed in 2.0.0-rc.4, so v2.4.1 is not affected. Latest is v2.6.1.
  - **gofiber/schema v1.8.3:** no OSV or GHSA entries. Latest is v1.8.8.
  - **govulncheck v1.8.0** scanned `spikes/a/main.go`, which imports fiber plus the helmet, csrf, session, limiter, requestid, recover and static middleware, against the 17 modules and the go1.27.1 stdlib. Symbol results: none. Package results: none. Module results: 4, all in `golang.org/x/crypto v0.54.0`, none imported:
    - GO-2026-6355 and GO-2026-6354: `x/crypto/ssh` DoS, fixed in v0.56.0
    - GO-2026-6303: `x/crypto/ssh` source-address option, fixed in v0.55.0
    - GO-2026-5932: `x/crypto/openpgp` unmaintained, with no fix
    - Exit code 0. Output is in `spikes/a/govulncheck.txt` and `spikes/a/govulncheck-verbose.txt`.
- Decision / impact on BASELINE.md:
  - Pin `github.com/gofiber/fiber/v3 v3.5.0` exactly. This satisfies the BASELINE §1 "≥ v3.5.0" rule, and no known advisory affects it.
  - Require `golang.org/x/crypto` directly (BASELINE §1 already lists it for argon2) at **v0.57.0**. MVS then lifts fiber's v0.54.0, which clears GO-2026-6303/6354/6355 at module level. GO-2026-5932 (openpgp) has no fix and will keep showing as a module-level, not-called finding: `make vuln` must fail only on symbol- or package-level findings. Add a depguard deny for `golang.org/x/crypto/openpgp`.
  - fasthttp override is an **open decision that needs an ADR** (BASELINE §1 "No new dependency without an ADR" applies, since go-brrr is new). Options:
    - (a) Keep fiber's tested v1.73.0.
    - (b) Require v1.74.0 for the framing and Content-Length fixes.
    - (c) Require v1.75.0 for cookie-injection and quadratic-path hardening (released today).

    Recommendation: (c) once a Fiber patch adopts it or after a 2-week soak. Until then use (a), with a weekly OSV/release check.
  - RESEARCH §6 seed correction: CVE-2026-42554 is fixed in **v3.2.0 / v2.52.13**, not v3.1.0 / v2.52.12. CVE-2026-25882 / GO-2026-4543 is fixed in **v3.1.0 / v2.52.12**.
- Proven by test: govulncheck run on `spikes/a/main.go` (results above). Permanent equivalent: `make vuln` in CI plus the weekly scheduled govulncheck/OSV job (RESEARCH §8).
- Re-check trigger: any fiber, fasthttp, gofiber/utils or golang.org/x/crypto version change; any new GHSA on gofiber/fiber or valyala/fasthttp; weekly CI OSV query.
