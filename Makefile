.PHONY: build build-fast build-desktop release test test-go test-front lint fmt tidy clean run licenses build-frontend

BIN_DIR := build/bin
BIN     := $(BIN_DIR)/spk-cockpit
GO      := go

# build is the default — produces a dev desktop binary with DevTools enabled.
build: build-frontend build-desktop

# build-fast skips the frontend build; useful when only Go changed and
# cmd/cockpit/dist already has a fresh frontend build.
build-fast: build-desktop

build-frontend:
	cd web && pnpm install --frozen-lockfile && pnpm build

# build-desktop produces the dev binary: -tags wails enables the desktop
# runner, no `production` tag → DevTools enabled (see internal/desktop/devtools_dev.go).
# The dist/ copy ritual is required so `//go:embed all:dist` in
# cmd/cockpit/embed.go finds the bundled frontend at compile time.
build-desktop:
	mkdir -p $(BIN_DIR)
	rm -rf cmd/cockpit/dist && cp -r web/dist cmd/cockpit/dist
	CGO_ENABLED=1 $(GO) build -tags wails -trimpath -ldflags="-w -s" -o $(BIN) ./cmd/cockpit
	rm -rf cmd/cockpit/dist
	mkdir -p cmd/cockpit/dist
	touch cmd/cockpit/dist/.gitkeep

# release builds the desktop binary with the production tag set: DevTools off.
release: build-frontend
	mkdir -p $(BIN_DIR)
	rm -rf cmd/cockpit/dist && cp -r web/dist cmd/cockpit/dist
	CGO_ENABLED=1 $(GO) build -tags "wails production" -trimpath -ldflags="-w -s" -o $(BIN_DIR)/spk-cockpit-release ./cmd/cockpit
	rm -rf cmd/cockpit/dist
	mkdir -p cmd/cockpit/dist
	touch cmd/cockpit/dist/.gitkeep

# run is dev-mode shorthand: build then exec.
run: build
	$(BIN)

test: test-go test-front

test-go:
	$(GO) test -tags wails -race -timeout 120s ./...

test-front:
	cd web && pnpm test --run

lint:
	golangci-lint run --build-tags wails
	cd web && pnpm lint

fmt:
	$(GO) fmt ./...
	cd web && pnpm prettier -w src

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR) web/dist cmd/cockpit/dist
	mkdir -p cmd/cockpit/dist
	touch cmd/cockpit/dist/.gitkeep

licenses:
	./scripts/gen-third-party-licenses.sh
