BINARY := ggt
CMD := ./cmd/ggt
BUILD_DIR := bin

.PHONY: help build test run fmt vet clean

help:
	@echo "Targets:"
	@echo "  make build             Build the binary into $(BUILD_DIR)/$(BINARY)"
	@echo "  make test              Run the test suite"
	@echo "  make run FILE=path     Build (if needed) and run 'ggt chopro' on FILE, print to stdout"
	@echo "  make run FILE=path OUT=path  Run, write to OUT instead"
	@echo "  make fmt               gofmt all source files"
	@echo "  make vet               go vet all packages"
	@echo "  make clean             Remove build artifacts"

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

test:
	go test ./... -v

run: build
ifndef FILE
	$(error Usage: make run FILE=path/to/chart.txt [OUT=path/to/out.chopro])
endif
ifdef OUT
	$(BUILD_DIR)/$(BINARY) chopro $(FILE) --output $(OUT)
	@echo "wrote $(OUT)"
else
	$(BUILD_DIR)/$(BINARY) chopro $(FILE)
endif

fmt:
	gofmt -l -w .

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)
