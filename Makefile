SOURCES = go.mod go.sum $(shell find * -type f -name "*.go")

RELEASE_LINUX := $(foreach arch, amd64 arm64 ,release/batch-tool-linux-${arch}.txz)
RELEASE_MACOS := $(foreach arch, amd64 arm64 ,release/batch-tool-darwin-${arch}.txz)
RELEASE_WINDOWS := $(foreach arch, amd64 ,release/batch-tool-windows-${arch}.zip)

RELEASES = $(RELEASE_LINUX) $(RELEASE_MACOS) $(RELEASE_WINDOWS)

# scrape target filenames for the OS and architecture needed
TARGET_OS = $(shell echo '$@' | sed 's|.*[bin|release]/batch-tool-\([^-./]\+\).*|\1|')
TARGET_ARCH = $(shell echo '$@' | sed 's|.*[bin|release]/batch-tool-[^-./]\+-\([^-./]\+\).*|\1|')

.PHONY: help install build .pretest test cover release clean veryclean

help:
	@echo "Makefile targets:"
	@echo "  help         Show this help message"
	@echo "  install      Install batch-tool to ${GOPATH}/bin"
	@echo "  build        Build the executable for the current OS and architecture"
	@echo "  release      Create release packages for all platforms"
	@echo "  clean        Remove build artifacts"
	@echo "  veryclean    Remove all generated files including vendor dependencies"

install: ${GOPATH}/bin/batch-tool
${GOPATH}/bin/batch-tool: $(SOURCES)
	@echo "Installing batch-tool to ${GOPATH}/bin"
	@go install

build: bin/batch-tool
bin/batch-tool: $(SOURCES)
	@echo "Building batch-tool"
	@go build -o $@

bin/%: $(SOURCES)
	@echo "Building batch-tool for $(TARGET_OS)-$(TARGET_ARCH)"
	@GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go build -o $@

.pretest:
	go mod tidy && go mod vendor

test: .pretest
	go test ./... -v

cover: .pretest
	go test ./... -cover -v

release: $(RELEASES)

release/%.txz: bin/%
	@echo "Creating TAR release package for $(TARGET_OS)-$(TARGET_ARCH)"
	@mkdir -p release/
ifeq "$(TARGET_OS)" "windows"
	@$(foreach target, $^, cp $(target) release/batch-tool.exe && tar --transform='s|.*/||' --remove-files -caf $@ release/batch-tool.exe;)
else
	@$(foreach target, $^, cp $(target) release/batch-tool && tar --transform='s|.*/||' --remove-files -caf $@ release/batch-tool;)
endif

release/%.zip: bin/%
	@echo "Creating ZIP release package for $(TARGET_OS)-$(TARGET_ARCH)"
	@mkdir -p release/
ifeq "$(TARGET_OS)" "windows"
	@$(foreach target, $^, cp $(target) release/batch-tool.exe && zip -mqj $@ release/batch-tool.exe;)
else
	@$(foreach target, $^, cp $(target) release/batch-tool && zip -mqj $@ release/batch-tool;)
endif

clean:
	@rm -rf bin/ release/

veryclean: clean
	@rm -rf vendor/
