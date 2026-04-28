.PHONY: build build-fast build-dev web-build test test-unit lint fmt tidy clean run licenses

GO ?= go
BUILD_DIR := build/bin
BIN := $(BUILD_DIR)/spk-cockpit

build: web-build build-fast

build-fast:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -tags "webkit2_41 production" -o $(BIN) ./cmd/cockpit

# build-dev swaps `production` for `dev` so the embedded webkit2gtk enables
# developer extras (F12 / right-click → Inspect Element). Wails refuses to
# launch a binary built without one of `production` or `dev`. Used by
# `make run`.
build-dev:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -tags "webkit2_41 dev" -o $(BIN) ./cmd/cockpit

web-build:
	cd web && pnpm install --frozen-lockfile && pnpm build
	rm -rf web/embed/dist && cp -r web/dist web/embed/dist

test: test-unit
	cd web && pnpm test --run

test-unit:
	$(GO) test ./internal/...

lint:
	golangci-lint run
	cd web && pnpm lint

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BUILD_DIR) web/dist web/embed/dist

run: web-build build-dev
	$(BIN)

licenses:
	./scripts/gen-third-party-licenses.sh
