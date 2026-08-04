BINARY=inotitidy
VERSION?=dev
LDFLAGS=-s -w -X main.version=$(VERSION)

.PHONY: build run test race vet fmt lint install clean

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/inotitidy

run:
	go run ./cmd/inotitidy

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

lint: fmt vet
	go test -race ./...

install: build
	bash ./install.sh

clean:
	rm -f $(BINARY)
	rm -rf dist
