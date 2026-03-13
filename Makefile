BINARY=inotitidy

build:
	go build -o $(BINARY) ./cmd/inotitidy

install: build
	bash ./install.sh

clean:
	rm -f $(BINARY)