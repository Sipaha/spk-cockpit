.PHONY: build build-fast web-build test test-unit lint fmt tidy clean run

GO ?= go
BUILD_DIR := build/bin
BIN := $(BUILD_DIR)/spk-cockpit

build: web-build build-fast

build-fast:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -tags "webkit2_41 production" -o $(BIN) ./cmd/cockpit

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

run: build-fast
	$(BIN) start --foreground
