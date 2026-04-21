# Copilot Agent Instructions for Azure Container Networking

## Go Version Upgrade Procedure

When assigned an issue to upgrade the Go version in this repository, follow this procedure exactly.

### Architecture Overview

This repo uses a **template system** for Dockerfiles:
- `build/images.mk` defines `GO_IMG` (Go builder image) and `MARINER_DISTROLESS_IMG` (runtime base image)
- `.tmpl` files in each component directory are rendered into Dockerfiles by `make dockerfiles`
- `make dockerfiles` uses `renderkit` and `skopeo` to resolve image tags to SHA digests
- The signed binary pipeline uses `.pipelines/build/scripts/install-go.sh` which has a `DEFAULT_IMAGE` fallback SHA

### Files to Update (in order)

1. **`build/images.mk`** — Update `GO_IMG` tag (e.g., `golang:1.27-azurelinux3.0`)
2. **`go.mod`** — Update `go` directive to match new version
3. **Run `go mod tidy`** — Fix any dependency issues
4. **`.pipelines/build/scripts/install-go.sh`** — Update `DEFAULT_IMAGE` SHA
   - Get new SHA: `skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:<VERSION>-azurelinux3.0 --format "{{.Digest}}"`
5. **`bpf-prog/ipv6-hp-bpf/linux.Dockerfile`** — Update Go image SHA (this file is NOT template-managed)
   - Uses MCR image: `mcr.microsoft.com/oss/go/microsoft/golang@sha256:...`
6. **`npm/linux.Dockerfile`** and **`npm/windows.Dockerfile`** — Update Go tag (NOT template-managed, no SHA pin)
7. **`.devcontainer/Dockerfile`** — Update `VARIANT` arg
8. **Run `make dockerfiles`** — Regenerate all template-based Dockerfiles

### FIPS / System Crypto (Go 1.26+)

For Go versions >= 1.26, Microsoft's Go fork requires FIPS-compliant crypto:

- **GOEXPERIMENT=systemcrypto** must be set in:
  - All `.tmpl` Dockerfile templates (as `ENV GOEXPERIMENT=systemcrypto` after the builder FROM line)
  - All `.pipelines/build/scripts/*.sh` pipeline scripts (as `export GOEXPERIMENT=systemcrypto`)
  - `bpf-prog/ipv6-hp-bpf/linux.Dockerfile`
  - `npm/linux.Dockerfile`

- **Runtime base image** must include crypto libraries:
  - `MARINER_DISTROLESS_IMG` in `build/images.mk` must be `distroless/base` (NOT `distroless/minimal`)
  - `bpf-prog` runtime stage must use `azurelinux/distroless/base:3.0`

- **Remove** `MS_GO_NOSYSTEMCRYPTO=1` if present in any Dockerfile

### Checking for Breaking Changes

Before making changes, research the new Go version:

1. Check release notes at `https://go.dev/doc/go<MAJOR>.<MINOR>` (e.g., `https://go.dev/doc/go1.27`)
2. Look for deprecated APIs used in this codebase
3. Check if GOEXPERIMENT behavior changed (e.g., if systemcrypto becomes default)
4. Verify MS Go fork release notes at `https://github.com/nicholasgasior/renderkit` (check if new flags are needed)
5. Check `.github/go-upgrade-rules.yaml` if it exists for known transition requirements

### Validation Steps

After making all changes:

1. `go build ./...` — Verify compilation
2. `go vet ./...` — Check for issues
3. `make dockerfiles` — Ensure templates render correctly
4. `go mod tidy` — Ensure deps are clean
5. Verify no `replace` directives are needed for incompatible deps
6. Check that ARM and AMD builds both work (inspect Dockerfile multi-arch support)

### PR Guidelines

- Title: `chore: upgrade Go <OLD> → <NEW>`
- Reference the tracking issue in the PR body
- Include a summary of what changed and any breaking changes found
- List all files modified
- If FIPS changes were needed, call them out explicitly

### Important Notes

- The `npm/` component is no longer being released — update its Dockerfiles but don't worry about testing
- `cilium-log-collector` uses `CGO_ENABLED=1` (exception to the norm)
- The `baseimages.yaml` CI workflow will fail if `make dockerfiles` output doesn't match committed Dockerfiles
- Never use 3-part version tags in `build/images.mk` — use 2-part floating tags (e.g., `1.27`, not `1.27.0`) which resolve to latest patch via skopeo
