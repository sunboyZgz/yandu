.PHONY: check test-race ui dev release integration docs-dev docs-build docs-preview
ui:
	npm --prefix web ci
	npm --prefix web run build
check: ui
	go test ./...
	go vet ./...
test-race:
	go test -race ./...
dev: ui
	python3 release/fetch-components.py
	mkdir -p .local/bin/bin
	go build -o .local/bin/yandu ./cmd/yandu
	cp release/artifacts/deps/caddy_2.11.4_mac_arm64/caddy .local/bin/bin/
	cp release/artifacts/deps/frp_0.71.0_darwin_arm64/frpc .local/bin/bin/
	.local/bin/yandu --state-dir "$(CURDIR)/.yandu" ui
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
