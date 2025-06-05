init-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.56.2

test:
	go test -v ./...

lint:
	golangci-lint run

create-env:
	cp .env.example .env

build-cli:
	@echo "Building CLI..."
	@mkdir -p bin
	go build -o bin/otf-cli ./cmd/otf-cli/main.go
	@echo "CLI built successfully to bin/otf-cli"