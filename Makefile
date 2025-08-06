SOURCES = go.mod go.sum $(shell find * -type f -name "*.go")

.PHONY: help deps install build release test cover vendor clean veryclean .gotestsum .goreleaser

help:
	@echo "Makefile targets:"
	@echo "  help         Show this help message"
	@echo "  deps         Install required tools (gotestsum, goreleaser)"
	@echo "  install      Install batch-tool to ${GOPATH}/bin"
	@echo "  build        Build the executable for the current OS and architecture using GoReleaser"
	@echo "  release      Create release packages for all platforms using GoReleaser"
	@echo "  test         Run tests"
	@echo "  cover        Run tests with coverage report"
	@echo "  clean        Remove build artifacts"
	@echo "  veryclean    Remove all generated files including vendor dependencies"

deps:
	@echo "Installing required tools..."
	go install gotest.tools/gotestsum@latest
	go install github.com/goreleaser/goreleaser/v2@latest

install: ${GOPATH}/bin/batch-tool
${GOPATH}/bin/batch-tool: $(SOURCES)
	@echo "Installing batch-tool to ${GOPATH}/bin"
	@go install -ldflags "-s -w -X github.com/ryclarke/batch-tool/config.Version=$(shell git tag | tail -n1)"

build: .goreleaser
	@echo "Building for current platform..."
	goreleaser build --snapshot --clean --single-target

release: .goreleaser
	@echo "Creating release packages..."
	goreleaser release --clean

test: vendor .gotestsum
	@gotestsum ./...

cover: vendor .gotestsum
	@gotestsum -- -coverprofile=coverage.out ./...

vendor:
	@go mod tidy && go mod vendor

clean:
	@rm -rf dist/

veryclean: clean
	@rm -rf vendor/

.gotestsum:
	@which gotestsum > /dev/null || (echo "Error: gotestsum is not installed. Please install it first: https://github.com/gotestyourself/gotestsum#install" && exit 1)

.goreleaser:
	@which goreleaser > /dev/null || (echo "Error: goreleaser is not installed. Please install it first: https://goreleaser.com/install/" && exit 1)
