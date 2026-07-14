# Kan developer tasks.
#
# These are plain, Rad-free entry points that mirror the CI gates, so any CI failure
# can be reproduced locally with `make` alone (no Rad needed). Release orchestration
# lives in ./dev (which is written in Rad and used by the maintainer).
#
# Frontend builds run through ci/with-node.sh, which activates the Node version pinned in
# .nvmrc so the embedded bundle is reproducible; `npm ci` installs exactly the lockfile.
# Your shell's default node is routinely not the pinned one, and CI rebuilds the bundle on
# the pinned version and rejects a mismatch - so this is activated for you, not asked of
# you. KAN_SKIP_NODE_CHECK=1 opts out.

.PHONY: build test fmt validate-go verify-dist serve vuln node-version help

WITH_NODE := ci/with-node.sh

## build: build the frontend then the Go binary (binary embeds the frontend)
# Uses `npm install` (fast when deps are warm, installs on a fresh clone). The strict
# lockfile-exact `npm ci` lives in verify-dist and CI, where reproducibility is gated.
build:
	$(WITH_NODE) sh -c 'cd web && npm install && npm run build'
	go build -o bin/kan ./cmd/kan

## test: run Go and frontend tests
test:
	go test ./...
	$(WITH_NODE) sh -c 'cd web && npm install && npm run test'

## node-version: report the Node that frontend builds will use
node-version:
	@$(WITH_NODE) node -v

## fmt: format Go code in place
fmt:
	gofmt -w .

## validate-go: Go quality gate - fmt check, vet, test, build (mirrors CI + ./dev -v)
validate-go:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting (run 'make fmt'):"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	go vet ./...
	go test ./...
	go build -o bin/kan ./cmd/kan

## verify-dist: confirm the committed embed is reproduced by a fresh build (CI's gate)
# Removes the committed dist first so this checks reproduction, not just "no change" -
# same semantics as CI. On a build failure it leaves the dist removed; `git checkout
# internal/api/dist` restores it.
verify-dist:
	rm -rf internal/api/dist
	$(WITH_NODE) sh -c 'cd web && npm ci && npm run build'
	@if [ -n "$$(git status --porcelain internal/api/dist)" ]; then \
		echo "internal/api/dist does not match a fresh build - commit the rebuilt assets:"; \
		git status --short internal/api/dist; \
		exit 1; \
	fi
	@echo "Embedded dist is up to date."

## serve: build then run the local server
serve: build
	./bin/kan serve

## vuln: scan Go dependencies for known vulnerabilities
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
