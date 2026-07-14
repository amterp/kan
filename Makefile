# Kan developer tasks.
#
# These are plain, Rad-free entry points that mirror the CI gates, so any CI failure
# can be reproduced locally with `make` alone (no Rad needed). Release orchestration
# lives in ./dev (which is written in Rad and used by the maintainer).
#
# Frontend builds use the Node version pinned in .nvmrc so the embedded bundle is
# reproducible; `npm ci` installs exactly the lockfile. Targets that build the frontend
# depend on check-node, which enforces that pin rather than trusting it.

.PHONY: build test fmt validate-go verify-dist serve vuln check-node help

NODE_VERSION := $(shell tr -d ' \t\n\r' < .nvmrc)

## check-node: fail unless the active Node matches .nvmrc
# The built frontend is committed and CI rebuilds it with this exact version, failing if
# the result differs. A build on the wrong Node therefore doesn't break here - it breaks
# in CI, after the commit, which is a slow and confusing way to find out. So we check up
# front. Set KAN_SKIP_NODE_CHECK=1 to bypass (you own the resulting diff).
check-node:
	@if [ -n "$$KAN_SKIP_NODE_CHECK" ]; then exit 0; fi
	@command -v node >/dev/null 2>&1 || { \
		echo "Node is not installed or not on PATH (this repo pins Node $(NODE_VERSION), see .nvmrc)."; \
		exit 1; \
	}
	@want="$(NODE_VERSION)"; \
	have="$$(node -v | sed 's/^v//')"; \
	if [ "$${have%%.*}" != "$${want%%.*}" ]; then \
		echo "Node $$have is active, but this repo pins Node $$want (.nvmrc)."; \
		echo "The frontend bundle is committed and CI rebuilds it on Node $$want, so building"; \
		echo "on a different major version can produce a bundle CI then rejects."; \
		echo; \
		echo "Switch, then re-run:"; \
		echo "  nvm use          # or: fnm use / asdf install"; \
		echo; \
		echo "Non-interactive (nvm is a shell function, so it must be sourced - and don't"; \
		echo "pipe it, or it runs in a subshell and the PATH change is lost):"; \
		echo "  . \"\$$NVM_DIR/nvm.sh\" && nvm use && make build"; \
		echo; \
		echo "To bypass: KAN_SKIP_NODE_CHECK=1 make build"; \
		exit 1; \
	fi

## build: build the frontend then the Go binary (binary embeds the frontend)
# Uses `npm install` (fast when deps are warm, installs on a fresh clone). The strict
# lockfile-exact `npm ci` lives in verify-dist and CI, where reproducibility is gated.
build: check-node
	cd web && npm install && npm run build
	go build -o bin/kan ./cmd/kan

## test: run Go and frontend tests
test: check-node
	go test ./...
	cd web && npm install && npm run test

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
verify-dist: check-node
	rm -rf internal/api/dist
	cd web && npm ci && npm run build
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
