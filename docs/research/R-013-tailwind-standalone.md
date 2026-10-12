# R-013 Tailwind v4 standalone CLI
- Date: 2026-10-12
- Pinned versions: tailwindcss v4.3.3 (standalone binary), templ v0.3.1020
- Question: What are the binary names per OS/arch? Do releases publish checksums? What are the `@source` path semantics? Are `.templ` files detected? Are the forms/typography plugins available in the standalone v4 build? What do `--minify` / `--watch` do?
- Sources (with versions/URLs):
  - https://api.github.com/repos/tailwindlabs/tailwindcss/releases/latest: `v4.3.3`, published 2026-07-16, with asset `sha256sums.txt`
  - https://github.com/tailwindlabs/tailwindcss/releases/download/v4.3.3/sha256sums.txt
  - `./bin/tailwindcss-linux-x64 --help` (prints `≈ tailwindcss v4.3.3`)
  - Spike `spikes/d/styles/*.css`, `spikes/d/views*/` and a scratch dir with a `.templ`-only and a `.go`-only source folder
- Findings (facts only):
  - **Binaries** (names from `sha256sums.txt`): `tailwindcss-linux-x64`, `-linux-x64-musl`, `-linux-arm64`, `-linux-arm64-musl`, `-macos-x64`, `-macos-arm64`, `-windows-x64.exe`.
  - **Checksums are published** per release in `sha256sums.txt`. Values for v4.3.3:
    - linux-x64 `dc61b3ac6b8c9ca874c0cc4c57b2409791a64c5540404ca5f5367360babc313a`
    - linux-arm64 `55fd0b241214eff3de1e8ee4f22796662f2d2e7a49bcfca7477cfd0bac398195`
    - macos-x64 `7922e0953f2110c05976e3bf58f14e643d90427575e766b7d433f5f80cbee7e1`
    - macos-arm64 `cdf646702987a743464dff4d9c60fd4480d1c1e73dd819a9a67f1078815dce9d`
    - windows-x64 `e0e260ce048014e9268f6237ff18f8ccf02cef521cbd0ae04e82c2cdf7aa3955`

    The downloaded linux-x64 binary's `sha256sum` matched.
  - **Flags:** `-i/--input`, `-o/--output`, `-w/--watch[=always]` (`always` keeps watching when stdin closes, which is needed when run from make in the background), `--poll`, `-m/--minify`, `--optimize`, `--cwd`, `--map`, `--silent`.
  - **`@source`** paths are resolved relative to the CSS file. An explicit `@source` scans the folder even though `spikes/` is gitignored. `@import "tailwindcss" source(none);` disables automatic detection, so only explicit `@source` paths are scanned.
  - **Detection:** a folder holding only `a.templ` produced `tracking-tighter`, `skew-y-[13deg]` and `hover:text-blue-800`. A folder holding only `b.go` (class names in a Go string literal) produced `decoration-wavy`. So `.templ` and generated `_templ.go` files are both scanned.
  - **Plugins are bundled:** `@plugin "@tailwindcss/forms";` and `@plugin "@tailwindcss/typography";` work in the standalone binary with no Node and no download (`.prose…` and `input:where([type=email])…` rules emitted). The forms option `@plugin "@tailwindcss/forms" { strategy: "class"; }` emits only `.form-input`/etc. classes and no global element styles.
  - The forms plugin emits `data:image/svg+xml` background images (select arrow, checkbox). These are allowed by the baseline CSP `img-src 'self' data:`.
  - Output is a single external CSS file with no inline styles, so it is compatible with `style-src 'self'`.
- Decision / impact on BASELINE.md:
  - Pin tailwindcss **v4.3.3**. Makefile `tools` downloads `tailwindcss-$(OS)-$(ARCH)` into `./bin/` and verifies it against **our own committed** SHA-256 values (copied from the release `sha256sums.txt` above), not against a checksum file fetched at the same time.
  - §8.2 entry file becomes:
    ```css
    @import "tailwindcss" source(none);
    @source "../../internal/modules";
    @source "../../framework/view";
    @plugin "@tailwindcss/forms" { strategy: "class"; }
    @plugin "@tailwindcss/typography";
    ```
    `source(none)` keeps the scan deterministic (no accidental scanning of `docs/`, `spikes/`, `bin/`).
  - §8.2 "Form styling ⟨verify, R-013⟩" is resolved: the official forms + typography plugins are usable. Typography (`prose`) is for sanitized rich text only. No new dependency, because both ship inside the pinned binary.
  - `make dev` runs `./bin/tailwindcss -i … -o … --watch=always`.
- Proven by test: spike builds in `spikes/d/styles` and the scratch `.templ`-only/`.go`-only experiment (class presence checked with `grep` on the minified output). Permanent: the CI `generate-check` job (`make css` then `git diff --exit-code` on the manifest) and the `make tools` checksum-failure test (BASELINE §9 A03).
- Re-check trigger: any tailwindcss upgrade (new checksums, re-run detection and plugin checks); any change in where templates live.
