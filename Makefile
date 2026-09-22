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
BUILDMODE   ?= dev      # "dev" for a `make build`, "release" for goreleaser
LDFLAGS       := -s -w \
                 -X $(PKG_VERSION).Version=$(VERSION) \
                 -X $(PKG_VERSION).GitCommit=$(GITCOMMIT) \
                 -X $(PKG_VERSION).BuildDate=$(BUILDDATE) \
                 -X $(PKG_VERSION).Build=$(BUILDMODE)

.PHONY: help build build-stamp test run fmt vet clean version-patch version-minor version-major release release-dryrun

help:
	@echo "Targets:"
	@echo "  make build  Stamped dev build ($(BUILD_DIR)/$(BINARY): VERSION+commit+date, mode dev)"
	@echo "  make test              Run the test suite"
	@echo "  make run FILE=path     Build (if needed) and run 'ggt cpro' on FILE, print to stdout"
	@echo "  make run FILE=path OUT=path  Run, write to OUT instead"
	@echo "  make fmt               gofmt all source files"
	@echo "  make vet               go vet all packages"
	@echo "  make clean             Remove build artifacts"
	@echo "  make version-patch     Bump VERSION patch     (0.5.1 -> 0.5.2)"
	@echo "  make version-minor     Bump VERSION minor     (0.5.1 -> 0.6.0)"
	@echo "  make release           Cut + publish a GitHub release for VERSION   (human-driven, not in CI)"
	@echo "  make release-dryrun    Build release artifacts into dist/ (no tag, no publish)"

# `build` is the day-to-day target and it STAMPS the binary with the semantic
# VERSION, the short HEAD commit, the build date, and the build mode (the
# default "dev"). `ggt version` then tells you what you have and whether it
# postdates a feature. Run `make build BUILDMODE=release` for a release binary.

build:
	mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD)

# `build-stamp` is a kept alias: every build is now stamped, so it is just build.
build-stamp: build
	@echo "build-stamp is now an alias of build (all builds are stamped)."

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

# `release` is the human-driven release path. It is deliberately NOT wired into
# GitHub Actions: a release that tags + publishes to GitHub is the one step we
# keep in a human's hands. It reads VERSION, REFUSES to clobber a v$(VERSION)
# tag that already exists locally OR on the remote (the tag the user fights
# back against), then creates the annotated tag, pushes it, and lets Goreleaser
# publish from it. Run it from the commit you want to ship.
release:
	@version=$$(cat $(VERSION_FILE) | tr -d '\n'); tag="v$$version"; \
      echo "Releasing $$tag (from VERSION $$version)..." ; \
      if [ -n "$$(git tag -l "$$tag")" ]; then \
        echo "ERROR: local tag $$tag already exists; refusing to clobber" >&2 ; exit 1 ; \
      fi ; \
      if git ls-remote --tags origin "refs/tags/$$tag" 2>/dev/null | grep -q . ; then \
        echo "ERROR: remote tag $$tag already exists; refusing to clobber" >&2 ; exit 1 ; \
      fi ; \
      git tag -a "$$tag" -m "Release $$tag" ; \
      git push origin "$$tag" ; \
      GITHUB_TOKEN=$$(gh auth token) goreleaser release --clean ; \
      echo "" ; \
      echo "=== Released $$tag ===" ; \
      gh release list --json tagName,assets --limit 1 ; \
      echo "  open https://github.com/glarizza/ggt/releases/tag/$$tag"

# `release-dryrun` is the escape hatch: it builds + packs the release artifacts
# into dist/ WITHOUT creating a tag, pushing, or publishing, so you can eyeball
# the platform matrix and asset names before you run the real `make release`.
release-dryrun:
	@echo "Building release artifacts to dist/ (snapshot, no tag, no publish)..." ; \
      goreleaser release --clean --snapshot ; \
      echo "" ; \
      echo "=== dist/ artifacts ===" ; \
      ls -1 dist/ 2>/dev/null || echo "(none)"