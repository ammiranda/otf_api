init-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.56.2

test:
	go test -v ./...

lint:
	golangci-lint run