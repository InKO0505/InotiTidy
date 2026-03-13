BINARY=inotitidy

build:
	go build -o $(BINARY) cmd/inotitidy/main.go

install: build
	./install.sh

clean:
	rm -f $(BINARY)