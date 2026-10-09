.PHONY: check test-race ui dev dev-build dev-serve release integration docs-dev docs-build docs-preview
DEV_BIN_DIR ?= $(CURDIR)/.local/bin
DEV_STATE_DIR ?= $(CURDIR)/.yandu
ui:
	npm --prefix web ci
	npm --prefix web run build
check: ui
	go test ./...
	go vet ./...
test-race:
	go test -race ./...
dev-build: ui
	python3 release/fetch-components.py
	mkdir -p "$(DEV_BIN_DIR)/bin"
	go build -o "$(DEV_BIN_DIR)/yandu" ./cmd/yandu
	cp release/artifacts/deps/caddy_2.11.4_mac_arm64/caddy "$(DEV_BIN_DIR)/bin/"
	cp release/artifacts/deps/frp_0.71.0_darwin_arm64/frpc "$(DEV_BIN_DIR)/bin/"
dev: dev-build
	"$(DEV_BIN_DIR)/yandu" --state-dir "$(DEV_STATE_DIR)" ui
dev-serve: dev-build
	"$(DEV_BIN_DIR)/yandu" --state-dir "$(DEV_STATE_DIR)" serve
release:
	python3 release/build.py
integration:
	python3 release/fetch-components.py
	YANDU_TEST_CADDY="$(CURDIR)/release/artifacts/deps/caddy_2.11.4_mac_arm64/caddy" YANDU_TEST_FRPC="$(CURDIR)/release/artifacts/deps/frp_0.71.0_darwin_arm64/frpc" YANDU_TEST_FRPS="$(CURDIR)/release/artifacts/deps/frp_0.71.0_darwin_arm64/frps" go test -v ./tests
docs-dev:
	npm --prefix docs-site ci
	npm --prefix docs-site run dev
docs-build:
	npm --prefix docs-site ci
	npm --prefix docs-site run build
	npm --prefix docs-site run check
docs-preview:
	npm --prefix docs-site run preview
