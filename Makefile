# Shipwake: Go core with the web app embedded.
.PHONY: all web build test test-responsive test-history lint schema types site fixtures clean

all: web build

web:            ## build the web app into internal/site/dist
	cd web && pnpm install --frozen-lockfile && pnpm build

build:          ## build the CLI (embeds whatever web/ last built)
	go build -o bin/shipwake ./cmd/shipwake

test:           ## Go tests
	go test ./...

test-responsive: web ## browser layout and resize regression checks (install Chromium once first)
	cd web && pnpm test:responsive

test-history: all ## the timeline's order on this repository's own history and runs (needs full history)
	bin/shipwake build -o .cache/self-site .
	cd web && SHIPWAKE_CHECK_SITES=../.cache/self-site pnpm exec playwright test timeline-order

fixtures:       ## fetch the real repositories the build benchmark uses (network, once)
	tools/fixtures.sh

lint:           ## vet, format and lint everything
	go vet ./...
	test -z "$$(gofmt -l cmd internal)"
	cd web && pnpm exec tsc -p . && pnpm exec biome check src

schema: build   ## regenerate the JSON Schema and the TypeScript types from the Go model
	bin/shipwake schema > spec/shipwake.schema.json
	cd web && node scripts/types.mjs

site: all       ## build a site for this repository
	bin/shipwake build --out site

clean:
	rm -rf bin site internal/site/dist/assets internal/site/dist/index.html internal/site/dist/favicon.svg
