.PHONY: build install test vet clean

PREFIX ?= /usr/local

build:
	go build -o bin/anvil ./cmd/anvil

install: build
	install -m 755 bin/anvil $(PREFIX)/bin/anvil

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin/
