.PHONY: build test lint
build:
	go build -o bin/ccs ./cmd/ccs
test:
	go test ./...
lint:
	go vet ./...
