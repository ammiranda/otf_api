build:
	go build ./...
	cd cmd/otf-cli && go build ./...
	cd cmd/otf-mcp && go build ./...

test:
	go test -v ./...

tidy:
	go mod tidy
	cd cmd/otf-cli && go mod tidy
	cd cmd/otf-mcp && go mod tidy

build-cli:
	@echo "Building CLI..."
	@mkdir -p bin
	cd cmd/otf-cli && go build -o ../../bin/otf-cli .
	@echo "CLI built successfully to bin/otf-cli"

build-mcp:
	@echo "Building MCP server..."
	@mkdir -p bin
	cd cmd/otf-mcp && go build -o ../../bin/otf-mcp .
	@echo "MCP server built successfully to bin/otf-mcp"

lint:
	golangci-lint run

init-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.56.2

create-env:
	cp .env.example .env
