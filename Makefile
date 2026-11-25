SHELL = /bin/bash
PROJECT_NAME = routerctl
PROJECT_ROOT = $(abspath .)
DIST_DIR = dist
MODULE_NAME = github.com/sunerpy/router-tools

GIT_TAG = $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT_ID = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_FILES = $(shell find . -name "*.go" -not -path "./vendor/*")
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build build-local build-all clean fmt fmt-go fmt-oxfmt fmt-check lint test check run help gui-test gui-build gui-build-windows gui-installer-windows

all: fmt lint test build

build: fmt
	@echo "Building $(PROJECT_NAME)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w 		-X $(MODULE_NAME)/internal/buildinfo.Version=$(GIT_TAG) 		-X $(MODULE_NAME)/internal/buildinfo.BuildTime=$(BUILD_TIME) 		-X $(MODULE_NAME)/internal/buildinfo.CommitID=$(COMMIT_ID)" 		-o $(DIST_DIR)/$(PROJECT_NAME) ./cmd/routerctl

build-local:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 go build -trimpath -o $(DIST_DIR)/$(PROJECT_NAME) ./cmd/routerctl

build-all:
	@for platform in $(PLATFORMS); do 		GOOS=$$(echo $$platform | cut -d/ -f1); 		GOARCH=$$(echo $$platform | cut -d/ -f2); 		OUTPUT=$(DIST_DIR)/$(PROJECT_NAME)-$$GOOS-$$GOARCH; 		if [ "$$GOOS" = "windows" ]; then OUTPUT=$$OUTPUT.exe; fi; 		echo "Building $$platform -> $$OUTPUT"; 		GOOS=$$GOOS GOARCH=$$GOARCH CGO_ENABLED=0 go build -trimpath 			-ldflags="-s -w" -o $$OUTPUT ./cmd/routerctl || exit 1; 	done

clean:
	@rm -rf $(DIST_DIR)
	@go clean -testcache

fmt: fmt-oxfmt fmt-go

fmt-oxfmt:
	@if command -v oxfmt > /dev/null 2>&1; then 		oxfmt --no-error-on-unmatched-pattern --ignore-path "$(PROJECT_ROOT)/.oxfmtignore" "$(PROJECT_ROOT)"; 	else 		echo "oxfmt not found. Install with: npm install --global oxfmt@0.64.0"; 	fi

fmt-go:
	@if command -v goimports > /dev/null 2>&1; then 		echo "$(GO_FILES)" | tr ' ' '\n' | xargs -P 4 goimports -w -local $(MODULE_NAME); 	else 		echo "goimports not found. Install: go install golang.org/x/tools/cmd/goimports@v0.49.0"; 	fi
	@if command -v gofumpt > /dev/null 2>&1; then 		echo "$(GO_FILES)" | tr ' ' '\n' | xargs -P 4 gofumpt -extra -w; 	else 		echo "gofumpt not found. Install: go install mvdan.cc/gofumpt@v0.11.0"; 	fi

fmt-check:
	@set -eu; 	command -v goimports >/dev/null 2>&1 		|| { echo "goimports is required: go install golang.org/x/tools/cmd/goimports@v0.49.0"; exit 1; }; 	command -v gofumpt >/dev/null 2>&1 		|| { echo "gofumpt is required: go install mvdan.cc/gofumpt@v0.11.0"; exit 1; }; 	command -v oxfmt >/dev/null 2>&1 		|| { echo "oxfmt is required: npm install --global oxfmt@0.64.0"; exit 1; }; 	gofumpt_files=$$(echo "$(GO_FILES)" | tr ' ' '\n' | xargs gofumpt -extra -l); 	goimports_files=$$(echo "$(GO_FILES)" | tr ' ' '\n' | xargs goimports -l -local $(MODULE_NAME)); 	if [ -n "$$gofumpt_files$$goimports_files" ]; then 		printf 'Files needing gofumpt:\n%s\n' "$$gofumpt_files"; 		printf 'Files needing goimports:\n%s\n' "$$goimports_files"; 		exit 1; 	fi; 	oxfmt --no-error-on-unmatched-pattern --ignore-path "$(PROJECT_ROOT)/.oxfmtignore" --check "$(PROJECT_ROOT)"

lint:
	@go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 		|| { echo "golangci-lint v2 is required: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1"; exit 1; }
	@golangci-lint run ./...

test:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 go test ./... -count=1 -race -cover -coverprofile=$(DIST_DIR)/coverage.out
	@go tool cover -html=$(DIST_DIR)/coverage.out -o $(DIST_DIR)/coverage.html

check: fmt-check lint test build-local gui-test

# Linux requires GTK3 and WebKit2GTK 4.1 development packages.
gui-test:
	cd cmd/routergui/frontend && pnpm test && pnpm build
	go test ./internal/guiapi ./cmd/routergui

gui-build:
	cd cmd/routergui && wails build -clean -tags webkit2_41

gui-build-windows:
	cd cmd/routergui && wails build -platform windows/amd64 -skipbindings -o routergui-windows-amd64.exe

gui-installer-windows:
	cd cmd/routergui && wails build -platform windows/amd64 -skipbindings -nsis -o routergui-windows-amd64-setup.exe

run: build-local
	./$(DIST_DIR)/$(PROJECT_NAME)

help:
	@echo "Targets: build build-local build-all clean fmt fmt-check lint test check run gui-test gui-build gui-build-windows gui-installer-windows"
