# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Terraform provider for the Kion cloud governance platform, built on HashiCorp's Terraform Plugin Framework and structured after terraform-provider-aws. Uses kion-sdk-go (ogen-generated from the same OpenAPI spec), resolved from the module proxy via a versioned `replace` in `go.mod`.

## Common Commands

```bash
# Build
make build                    # Build provider binary

# Test
make test                     # Unit tests with coverage
make testacc                  # Acceptance tests (requires TF_ACC=1, KION_API_URL, KION_API_KEY or KION_AUTH_TOKEN)

# Lint & Format
make fmt                      # gofmt -s -w .
make lint                     # golangci-lint run (config: .golangci.yml, aligned with AWS provider)
make vet                      # go vet

# CI (mirrors ci.yml's fmt/vet/lint/test-unit jobs)
make ci                       # Run all: fmt, vet, lint, test
make ci-fmt                   # Format check (no write)
make ci-vet                   # go vet ./...
make ci-lint                  # golangci-lint run
make ci-test                  # Tests with race detection and coverage

# Scaffold new service packages
cd internal/service/<name>
go run ../../../cmd/kgen resource <PascalName>
go run ../../../cmd/kgen datasource <PascalName>

# Install locally
make install                  # Build + copy to ~/.terraform.d/plugins/

```

> Adding or removing a resource/data source changes this provider's supported
> surface, which the sibling `terraform-coverage-test` project tracks. After such
> a change, run `make coverage-new` in that repo — a new "gap" (or "stale") there
> means coverage-test needs a matching module update.

Run a single test: `go test -v -run TestAccKionLabel_basic ./internal/service/label/`

## Architecture

### Service Package Pattern (AWS-style)

Each resource lives in `internal/service/<name>/` with these files:
- `<name>.go` — Resource: struct, schema, CRUD methods, model
- `<name>_data_source.go` — Data source: struct, schema, read, model
- `<name>_test.go` — Resource acceptance tests
- `<name>_data_source_test.go` — Data source acceptance tests
- `service_package.go` — Factory registration (implements `conns.ServicePackage`)

Resources embed `framework.ResourceWithConfigure`, data sources embed `framework.DataSourceWithConfigure`. Both get the `KionClient` via `r.Meta().Client`.

### Key Internal Packages

- `internal/conns/` — `ServicePackage` interface definition
- `internal/errs/` — SDK response types to Terraform diagnostics
- `internal/flex/` — Type converters between TF Framework types and SDK types (int, string, bool, list)
- `internal/framework/` — Base types providing `Meta()` accessor for client
- `internal/servicepkg/` — Shared types (`ServicePackageResource`, `ServicePackageDataSource`)
- `internal/provider/` — Provider definition, configuration, service package registration
- `internal/kgen/` — Template-based scaffold generation (resource, datasource, convert)
- `cmd/kgen/` — CLI entry point for scaffold generator

### Generated vs Hand-Written Code

- **Generated (do NOT edit)**: `internal/provider_kion/*_gen.go` — provider schema from tfplugingen
- **Scaffolded (edit to implement)**: `internal/service/*/` — service packages with stub CRUD methods
- **Hand-written**: `internal/flex/`, `internal/errs/`, `internal/framework/`, `internal/conns/`, `internal/provider/provider.go`
- **Reference implementation**: `internal/service/label/` — fully working CRUD with SDK integration

### Provider Configuration

Provider accepts `api_url`, `api_key`, `auth_token` — all readable from environment variables `KION_API_URL`, `KION_API_KEY`, `KION_AUTH_TOKEN`. Authentication requires either `api_key` or `auth_token`.

## Key Conventions

- Resource names use `kion_` prefix (e.g., `kion_cloud_rule`, `kion_project`)
- 34 service packages in `internal/service/`
- Linting aligned with terraform-provider-aws (golangci-lint v2, `.golangci.yml`)
- Generated `*_gen.go` files excluded from all linters
- Service packages have targeted exclusions (revive, unused, staticcheck)
- Lefthook pre-push hook runs `ci-fmt`/`ci-vet`/`ci-lint`/`ci-test` before allowing pushes (skipped on tag-only pushes, which match no files)

## CI/CD

GitHub Actions only; there is no `.gitlab-ci.yml`. `.github/workflows/ci.yml` runs on pull requests and pushes to `main`: `fmt`, `vet`, `lint`, `test-unit` (race detector + coverage), `modules` (drift + `terraform validate`/`test`), `internal-refs`, `secrets`, `codeql`, and a `ci` aggregator job for branch protection. `.github/workflows/release.yml` runs goreleaser on a `v*` tag. No kion-sdk-go checkout is needed — the `replace` in `go.mod` is versioned, so the module proxy resolves it. Local `make ci` covers the first four jobs; the Lefthook pre-push hook runs them, except on tag-only pushes where it matches no files. See `.github/workflows/README.md`.

Acceptance tests: 4 parallel workers, 120-minute timeout. Sweepers (`make sweep`) clean up orphaned `test-acc`-prefixed resources.
