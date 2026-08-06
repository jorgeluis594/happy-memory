.DEFAULT_GOAL := help

GO ?= go
LINT := $(GO) tool golangci-lint
VULNCHECK := $(GO) tool govulncheck

.PHONY: help fmt fmt-check lint test vuln verify build tidy check release-check release-snapshot release-validate installers-test

help: ## Show available commands.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go source files.
	$(GO) fmt ./...

fmt-check: ## Check formatting without modifying files.
	$(LINT) fmt --diff

lint: ## Run configured linters (includes govet).
	$(LINT) run

test: ## Run tests with the race detector and coverage.
	$(GO) test -race -cover ./...

vuln: ## Check known vulnerabilities in dependencies.
	$(VULNCHECK) ./...

verify: ## Verify downloaded module checksums.
	$(GO) mod verify

build: ## Compile the CLI.
	$(GO) build ./cmd/happy-memory

tidy: ## Synchronize module files with source imports.
	$(GO) mod tidy

check: fmt-check lint test vuln verify build ## Run all non-mutating quality checks.

release-check: ## Validate the GoReleaser configuration.
	goreleaser check

release-snapshot: ## Build a local release snapshot without publishing.
	goreleaser release --snapshot --clean

release-validate: ## Inspect snapshot names, formats, contents, and checksums.
	sh scripts/validate-release.sh dist

installers-test: ## Test installers against controlled local release fixtures.
	sh scripts/test-installers.sh
