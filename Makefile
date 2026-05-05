BINARY_NAME := che-ai-pullrequest-assistant

.PHONY: build test fmt lint clean

build:
	go build -o $(BINARY_NAME) .

test:
	go test ./...

fmt:
	gofmt -w .

lint: fmt
	go vet ./...

clean:
	rm -f $(BINARY_NAME)
