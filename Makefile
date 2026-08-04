BINARY=inotitidy

.PHONY: build run test vet install clean

build:
	go build -o $(BINARY) ./cmd/inotitidy

run:
	go run ./cmd/inotitidy

test:
	go test ./...

vet:
	go vet ./...

install: build
	bash ./install.sh

clean:
	rm -f $(BINARY)
