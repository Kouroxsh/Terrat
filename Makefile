BINARY=terrat
VERSION?=3.0.0
PREFIX?=$(HOME)/go/bin

.PHONY: all build run clean install release package package-linux site-build site-dev

all: build

build:
	GOTOOLCHAIN=local go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(BINARY) ./cmd/terrat

run: build
	./$(BINARY)

install:
	@./packaging/install.sh "$(VERSION)" "$(PREFIX)" "$(BINARY)"

clean:
	rm -f $(BINARY)
	rm -rf dist release

package: release

release:
	@./packaging/build-packages.sh $(VERSION)

site-build:
	cd site && npm run build

site-dev:
	cd site && npm run dev
