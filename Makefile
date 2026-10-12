# Versions and checksums: docs/adr/0001-stack.md. templ, staticcheck and
# govulncheck are pinned in go.mod (tool directive) and run with `go tool`.
SQLC_VERSION     := 1.31.1
GOLANGCI_VERSION := 2.14.0
TAILWIND_VERSION := 4.3.3

# SHA-256 of the release archives/binaries (from the release checksums or the
# GitHub asset digest). Downloads that don't match fail the build.
SQLC_SHA256_linux_amd64      := 497ae4fcdfa64c5b0c311ffe4c2bd991e43991e82e5367792ed78bc2dca27354
SQLC_SHA256_linux_arm64      := b7cae247740d0c51a1e657479e5b2d21e6fef428f596682a01bc55bf4ab8a23d
SQLC_SHA256_darwin_amd64     := c5af76772e3785d21663a62697056b383f07629979b1bd25b93872e73dbd519b
SQLC_SHA256_darwin_arm64     := 21602158c99eb1f2bae197a66abfb1941d1e9e50b23125bb193349c6b1acc71e
GOLANGCI_SHA256_linux_amd64  := ab90aeb7b066f92a33415b638a50fe5344bbb75a0d32ad30cc248d88f81032ab
GOLANGCI_SHA256_linux_arm64  := ee7ec5f3453d15ddf106fae5a4d6c71737712348a979d1fe9cd52ec7ea299bae
GOLANGCI_SHA256_darwin_amd64 := a5667c1c3536be1740133213e1e822bfb8f0d98ea12903174d6d5f635e4ed68d
GOLANGCI_SHA256_darwin_arm64 := 5ef5f36a7147e91dc58ef9ef4d11bb7bad5ead0c76eb6c01327a73c641d1dcc3
TAILWIND_SHA256_linux_amd64  := dc61b3ac6b8c9ca874c0cc4c57b2409791a64c5540404ca5f5367360babc313a
TAILWIND_SHA256_linux_arm64  := 55fd0b241214eff3de1e8ee4f22796662f2d2e7a49bcfca7477cfd0bac398195
TAILWIND_SHA256_darwin_amd64 := 7922e0953f2110c05976e3bf58f14e643d90427575e766b7d433f5f80cbee7e1
TAILWIND_SHA256_darwin_arm64 := cdf646702987a743464dff4d9c60fd4480d1c1e73dd819a9a67f1078815dce9d

GOOS   := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
PLAT   := $(GOOS)_$(GOARCH)
TW_OS  := $(subst darwin,macos,$(GOOS))
TW_ARCH := $(subst amd64,x64,$(GOARCH))
SHA256SUM := $(shell command -v sha256sum >/dev/null 2>&1 && echo sha256sum || echo "shasum -a 256")

SQLC     := bin/sqlc-$(SQLC_VERSION)
GOLANGCI := bin/golangci-lint-$(GOLANGCI_VERSION)
TAILWIND := bin/tailwindcss-$(TAILWIND_VERSION)

# verify <file> <sha256>: fails (and deletes the file) on mismatch.
verify = echo "$(2)  $(1)" | $(SHA256SUM) -c - >/dev/null || { echo "checksum mismatch for $(1)" >&2; rm -f $(1); exit 1; }

.PHONY: tools generate css lint vuln test check build dev

tools: $(SQLC) $(GOLANGCI) $(TAILWIND)

$(SQLC):
	@test -n "$(SQLC_SHA256_$(PLAT))" || { echo "no sqlc checksum for $(PLAT)" >&2; exit 1; }
	@mkdir -p bin/tmp
	curl -fsSL -o bin/tmp/sqlc.tar.gz https://github.com/sqlc-dev/sqlc/releases/download/v$(SQLC_VERSION)/sqlc_$(SQLC_VERSION)_$(PLAT).tar.gz
	@$(call verify,bin/tmp/sqlc.tar.gz,$(SQLC_SHA256_$(PLAT)))
	tar -xzf bin/tmp/sqlc.tar.gz -C bin/tmp sqlc && mv bin/tmp/sqlc $@ && rm -rf bin/tmp

$(GOLANGCI):
	@test -n "$(GOLANGCI_SHA256_$(PLAT))" || { echo "no golangci-lint checksum for $(PLAT)" >&2; exit 1; }
	@mkdir -p bin/tmp
	curl -fsSL -o bin/tmp/golangci.tar.gz https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_VERSION)/golangci-lint-$(GOLANGCI_VERSION)-$(GOOS)-$(GOARCH).tar.gz
	@$(call verify,bin/tmp/golangci.tar.gz,$(GOLANGCI_SHA256_$(PLAT)))
	tar -xzf bin/tmp/golangci.tar.gz -C bin/tmp --strip-components=1 && mv bin/tmp/golangci-lint $@ && rm -rf bin/tmp

$(TAILWIND):
	@test -n "$(TAILWIND_SHA256_$(PLAT))" || { echo "no tailwindcss checksum for $(PLAT)" >&2; exit 1; }
	@mkdir -p bin
	curl -fsSL -o $@ https://github.com/tailwindlabs/tailwindcss/releases/download/v$(TAILWIND_VERSION)/tailwindcss-$(TW_OS)-$(TW_ARCH)
	@$(call verify,$@,$(TAILWIND_SHA256_$(PLAT)))
	chmod +x $@

# ponytail: sqlc and css run once sqlc.yaml (P4) and web/styles/input.css (P5) exist.
generate: $(SQLC)
	go tool templ generate
	$(if $(wildcard sqlc.yaml),$(SQLC) generate)

css: $(TAILWIND)
	$(if $(wildcard web/styles/input.css),$(TAILWIND) -i web/styles/input.css -o web/static/dist/app.css --minify)

# The gofmt check runs inside golangci-lint (formatters.enable in .golangci.yml).
lint: $(GOLANGCI)
	go vet ./...
	go tool staticcheck ./...
	$(GOLANGCI) run ./...

# govulncheck exits non-zero only for vulnerabilities our code can reach.
vuln:
	go tool govulncheck ./...

test:
	go test -race -cover ./...

# Generated output must match what is committed (CI checks the whole tree).
check: generate css
	git diff --exit-code -- '*_templ.go' internal/db web/static/dist
	$(MAKE) lint vuln test

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

# ponytail: tailwind --watch joins in P5.
dev:
	go tool templ generate -watch -cmd "go run ./cmd/server"
