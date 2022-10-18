
GIT_VERSION = $(shell git describe --tags --always)
FLAGS = -ldflags "\
  -X main.VERSION=$(GIT_VERSION) \
"

.PHONY: run
run:
	go run  $(FLAGS) ./...

.PHONY: build
build:
	CGO_ENABLED=0 go build $(FLAGS) -o bin/ ./cmd/...

.PHONY: test
test:
	go test -cover ./...

.PHONY: deps
deps:
	go mod tidy
	go mod vendor

.PHONY: release
release: clean
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(FLAGS) -o bin/goose.linux.arm64 ./cmd/...
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(FLAGS) -o bin/goose.linux.amd64 ./cmd/...
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build $(FLAGS) -o bin/goose.win.arm64.exe ./cmd/...
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(FLAGS) -o bin/goose.win.amd64.exe ./cmd/...
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(FLAGS) -o bin/goose.mac.arm64 ./cmd/...
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(FLAGS) -o bin/goose.mac.amd64 ./cmd/...
	md5sum bin/* > bin/checksum

.PHONY: clean
clean:
	rm -f bin/*

.PHONY: doc
doc:
	API_EXAMPLES_PATH=`pwd`/doc/api go test ./...

.PHONY: docker-%
docker-%:
	docker-compose run -p 8080:8080 --use-aliases --rm app make $*

.PHONY: docker
docker: clean build
	@echo "goose:$(GIT_VERSION)"
	docker build -t volume-take-home:$(GIT_VERSION) .

.PHONY: version
version:
	@echo -n "$(GIT_VERSION)"
