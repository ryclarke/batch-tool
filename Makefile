SOURCES = go.mod go.sum $(shell find * -type f -name "*.go")

.PHONY: help
help:
	@echo "Makefile targets:"
	@echo "  help         Show this help message"
	@echo "  deps         Install required tools (gotestsum, goreleaser, golangci-lint)"
	@echo "  install      Install batch-tool to ${GOPATH}/bin"
	@echo "  build        Build the executable for the current OS and architecture using GoReleaser"
	@echo "  release      Create release packages for all platforms using GoReleaser"
	@echo "  test         Run tests"
	@echo "  cover        Run tests with coverage report"
	@echo "  lint         Run golangci-lint"
	@echo "  lint-fix     Run golangci-lint with auto-fix"
	@echo "  clean        Remove build artifacts"
	@echo "  veryclean    Remove all generated files including vendor dependencies"

.PHONY: deps
deps:
	@echo "Installing required tools..."
	go install gotest.tools/gotestsum@latest
	go install github.com/goreleaser/goreleaser/v2@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2

.PHONY: install
install: ${GOPATH}/bin/batch-tool
${GOPATH}/bin/batch-tool: $(SOURCES)
	@echo "Installing batch-tool to ${GOPATH}/bin"
	@go install -ldflags "-s -w -X github.com/ryclarke/batch-tool/config.Version=$(shell git tag | tail -n1)"

.PHONY: completions
completions: ~/.local/share/bash-completion/completions/batch-tool
~/.local/share/bash-completion/completions/batch-tool: ${GOPATH}/bin/batch-tool
	@echo "Configuring bash completions for batch-tool..."
	@mkdir -p ~/.local/share/bash-completion/completions
	@${GOPATH}/bin/batch-tool completion bash > $@

.PHONY: build
build: .goreleaser
	@echo "Building for current platform..."
	goreleaser build --snapshot --clean --single-target

.PHONY: release
release: .goreleaser
	@echo "Creating release packages..."
	goreleaser release --clean

.PHONY: test
test: vendor .gotestsum
	@gotestsum -- -race ./...

.PHONY: cover
cover: vendor .gotestsum
	@gotestsum -- -race -coverprofile=coverage.out ./...

.PHONY: lint
lint: .golangci-lint
	@golangci-lint run

.PHONY: lint-fix
lint-fix: .golangci-lint
	@golangci-lint run --fix

.PHONY: vendor
vendor:
	@go mod tidy && go mod vendor

.PHONY: clean
clean:
	@rm -rf dist/

.PHONY: veryclean
veryclean: clean
	@rm -rf vendor/

.PHONY: .gotestsum
.gotestsum:
	@which gotestsum > /dev/null || (echo "Error: gotestsum is not installed. Please install it first: https://github.com/gotestyourself/gotestsum#install"; exit 1)

.PHONY: .goreleaser
.goreleaser:
	@which goreleaser > /dev/null || (echo "Error: goreleaser is not installed. Please install it first: https://goreleaser.com/install/"; exit 1)

.PHONY: .golangci-lint
.golangci-lint:
	@which golangci-lint > /dev/null || (echo "Error: golangci-lint is not installed. Please install it first: https://golangci-lint.run/docs/welcome/install/"; exit 1)
