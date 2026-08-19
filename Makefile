VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION)

build:
	@echo "Building all binaries (version $(VERSION))..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/otf-cli ./cmd/otf-cli/
	go build -ldflags "$(LDFLAGS)" -o bin/otf-mcp ./cmd/otf-mcp/
	@echo "Built: bin/otf-cli bin/otf-mcp"

init-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s v2.12.2

test:
	go test -v ./...

lint:
	golangci-lint run

build-cli:
	@echo "Building CLI (version $(VERSION))..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/otf-cli ./cmd/otf-cli/
	@echo "CLI built successfully to bin/otf-cli"

build-mcp:
	@echo "Building MCP server (version $(VERSION))..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/otf-mcp ./cmd/otf-mcp/
	@echo "MCP server built successfully to bin/otf-mcp"
