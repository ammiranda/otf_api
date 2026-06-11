build:
	@echo "Building all binaries..."
	@mkdir -p bin
	go build -o bin/otf-cli ./cmd/otf-cli/main.go
	go build -o bin/otf-mcp ./cmd/otf-mcp/main.go
	@echo "Built: bin/otf-cli bin/otf-mcp"

init-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s v2.12.2

test:
	go test -v ./...

lint:
	golangci-lint run

build-cli:
	@echo "Building CLI..."
	@mkdir -p bin
	go build -o bin/otf-cli ./cmd/otf-cli/main.go
	@echo "CLI built successfully to bin/otf-cli"

build-mcp:
	@echo "Building MCP server..."
	@mkdir -p bin
	go build -o bin/otf-mcp ./cmd/otf-mcp/main.go
	@echo "MCP server built successfully to bin/otf-mcp"