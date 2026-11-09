# Builds the portable Windows executable (build/bin/ssm-helper.exe).
# Requires Go, Node/npm and the Wails v2 CLI:
#   go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0

.PHONY: build test dev

# Shown on the About screen (see about.go)
COMMIT := $(shell git rev-parse HEAD 2>/dev/null)
COMMIT_TIME := $(shell git log -1 --format=%cI 2>/dev/null)
COMMIT_MODIFIED := $(shell git status --porcelain 2>/dev/null | grep -q . && echo true)

build:
	wails build -platform windows/amd64 -trimpath \
		-ldflags "-s -w -X main.commit=$(COMMIT) -X main.commitTime=$(COMMIT_TIME) -X main.commitModified=$(COMMIT_MODIFIED)"

test:
	go vet ./...
	go test ./...
	node scripts/check-i18n.mjs
	cd frontend && npx tsc --noEmit

dev:
	wails dev
