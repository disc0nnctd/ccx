.PHONY: build build-cc-connect build-codex-switcher sync-upstream sync-cc-connect sync-codex-switcher deploy

# Build cc-connect and place binary in bin/
build-cc-connect:
	mkdir -p bin
	cd cc-connect && go build -ldflags "-s -w" -o ../bin/cc-connect ./cmd/cc-connect

# Build codex-switcher (Tauri)
build-codex-switcher:
	cd codex-switcher && cargo tauri build

# Default: build just cc-connect (primary service)
build: build-cc-connect

# Sync from upstream repos
sync-cc-connect:
	git subtree pull --prefix=cc-connect cc-connect-upstream main --squash

sync-codex-switcher:
	git subtree pull --prefix=codex-switcher codex-switcher-upstream main --squash

sync-upstream: sync-cc-connect sync-codex-switcher

# Deploy: rebuild binary and restart service
deploy: build-cc-connect
	sudo systemctl restart cc-connect

# Status check
status:
	sudo systemctl status cc-connect
	./custom/commands/codex-switch

# Clean build artifacts
clean:
	rm -f bin/cc-connect
	cd cc-connect && go clean
