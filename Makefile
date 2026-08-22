BINARY := ggt
CMD := ./cmd/ggt
BUILD_DIR := bin

# --- Version stamping.  Human source of truth is VERSION; build injects it
# plus commit + build date via -X into ggt/internal/version.
VERSION_FILE := VERSION
VERSION    ?= $(shell cat $(VERSION_FILE) 2>/dev/null || echo 0.0.0)
GITCOMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILDDATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG_VERSION:= ggt/internal/version
LDFLAGS       := -s -w \
                 -X $(PKG_VERSION).Version=$(VERSION) \
                 -X $(PKG_VERSION).GitCommit=$(GITCOMMIT) \
                 -X $(PKG_VERSION).BuildDate=$(BUILDDATE)

.PHONY: help build build-stamp test run fmt vet clean version-patch version-minor version-major

help:
	@echo "Targets:"
	@echo "  make build             Build the binary into $(BUILD_DIR)/$(BINARY)"
	@echo "  make test              Run the test suite"
	@echo "  make run FILE=path     Build (if needed) and run 'ggt cpro' on FILE, print to stdout"
	@echo "  make run FILE=path OUT=path  Run, write to OUT instead"
	@echo "  make fmt               gofmt all source files"
	@echo "  make vet               go vet all packages"
	@echo "  make clean             Remove build artifacts"

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

build-stamp:
		mkdir -p $(BUILD_DIR)
		go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD)

test:
	go test ./... -v

run: build
ifndef FILE
	$(error Usage: make run FILE=path/to/chart.txt [OUT=path/to/out.cpro])
endif
ifdef OUT
	$(BUILD_DIR)/$(BINARY) cpro $(FILE) --output $(OUT)
	@echo "wrote $(OUT)"
else
	$(BUILD_DIR)/$(BINARY) cpro $(FILE)
endif

fmt:
	gofmt -l -w .

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)


version-patch: # Bump patch version (0.1.0 -> 0.1.1)
	@current=$$(cat $(VERSION_FILE) | tr -d '\n'); \
      major=$$(echo $$current | cut -d. -f1); \
      minor=$$(echo $$current | cut -d. -f2); \
      patch=$$(echo $$current | cut -d. -f3); \
      new_patch=$$((patch + 1)); \
      new_version="$$major.$$minor.$$new_patch"; \
      echo "Bumping version: $$current -> $$new_version"; \
      echo $$new_version > $(VERSION_FILE); \
      echo "Version updated to $$new_version"

version-minor: # Bump minor version (0.1.0 -> 0.2.0)
	@current=$$(cat $(VERSION_FILE) | tr -d '\n'); \
      major=$$(echo $$current | cut -d. -f1); \
      minor=$$(echo $$current | cut -d. -f2); \
      new_minor=$$((minor + 1)); \
      new_version="$$major.$$new_minor.0"; \
      echo "Bumping version: $$current -> $$new_version"; \
      echo $$new_version > $(VERSION_FILE); \
      echo "Version updated to $$new_version"

version-major: # Bump major version (0.1.0 -> 1.0.0)
	@current=$$(cat $(VERSION_FILE) | tr -d '\n'); \
      major=$$(echo $$current | cut -d. -f1); \
      new_major=$$((major + 1)); \
      new_version="$$new_major.0.0"; \
      echo "Bumping version: $$current -> $$new_version"; \
      echo $$new_version > $(VERSION_FILE); \
      echo "Version updated to $$new_version"