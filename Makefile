SOURCES = go.mod go.sum $(shell find * -type f -name "*.go")

.PHONY: help install build .pretest test cover release clean veryclean check-goreleaser

check-goreleaser:
	@which goreleaser > /dev/null || (echo "Error: goreleaser is not installed. Please install it first: https://goreleaser.com/install/" && exit 1)

help:
	@echo "Makefile targets:"
	@echo "  help         Show this help message"
	@echo "  install      Install batch-tool to ${GOPATH}/bin"
	@echo "  build        Build the executable for the current OS and architecture using GoReleaser"
	@echo "  release      Create release packages for all platforms using GoReleaser"
	@echo "  test         Run tests"
	@echo "  cover        Run tests with coverage"
	@echo "  clean        Remove build artifacts"
	@echo "  veryclean    Remove all generated files including vendor dependencies"

install: ${GOPATH}/bin/batch-tool
${GOPATH}/bin/batch-tool: $(SOURCES)
	@echo "Installing batch-tool to ${GOPATH}/bin"
	@go install

build: check-goreleaser
	@echo "Building for current platform..."
	goreleaser build --snapshot --clean --single-target

release: check-goreleaser
	@echo "Creating release packages..."
	goreleaser release --clean

.pretest:
	go mod tidy && go mod vendor

test: .pretest
	go test ./... -v

cover: .pretest
	go test ./... -cover -v

clean:
	@rm -rf dist/

veryclean: clean
	@rm -rf vendor/
