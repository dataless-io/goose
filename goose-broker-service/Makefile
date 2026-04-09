.PHONY: run
run:
	go run ./cmd/brokerd

.PHONY: build
build:
	CGO_ENABLED=0 go build -o bin/brokerd ./cmd/brokerd

.PHONY: test
test:
	go test ./...

.PHONY: tidy
tidy:
	go mod tidy
