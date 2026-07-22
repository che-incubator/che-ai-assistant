BINARY_NAME := che-ai-assistant

.PHONY: build test fmt lint clean

build:
	go build -o $(BINARY_NAME) .

test:
	go test ./...

fmt:
	gofmt -w .
	goimports -w .

lint: fmt
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)
