.PHONY: build run test lint fmt

build:
	go build -o bin/gitone ./cmd/gitone

run:
	go run ./cmd/gitone

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

fmt:
	gofmt -w .
