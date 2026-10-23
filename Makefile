# Builds the portable Windows executable (build/bin/ssm-helper.exe).
# Requires Go, Node/npm and the Wails v2 CLI:
#   go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0

.PHONY: build test dev

build:
	wails build -platform windows/amd64 -trimpath -ldflags "-s -w"

test:
	go vet ./...
	go test ./...
	node scripts/check-i18n.mjs
	cd frontend && npx tsc --noEmit

dev:
	wails dev
