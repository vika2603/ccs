GOLANGCI_LINT ?= golangci-lint
GOVULNCHECK ?= govulncheck

.PHONY: build test lint fmt tidy vuln check

build:
	go build -o bin/ccs ./cmd/ccs

test:
	go test -race ./...

lint:
	go vet ./...
	$(GOLANGCI_LINT) run ./...

fmt:
	$(GOLANGCI_LINT) fmt ./...

tidy:
	go mod tidy

vuln:
	$(GOVULNCHECK) ./...

check: lint test
