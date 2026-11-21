BINARY=terrat
# L56: single source of truth for the version (also read by
# packaging/build-packages.sh and packaging/install.sh).
VERSION?=$(shell cat VERSION 2>/dev/null || echo 3.0.0)
# M34: git tags carry a leading `v`; package metadata and the banner must
# not see it (`v3.0.0` would otherwise become "vv3.0.0"). `override` is
# needed so this also applies to VERSION passed on the command line.
override VERSION := $(patsubst v%,%,$(VERSION))
PREFIX?=$(HOME)/go/bin

.PHONY: all build run clean install release package package-linux site-build site-dev

all: build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(BINARY) ./cmd/terrat

run: build
	./$(BINARY)

install:
	@./packaging/install.sh "$(VERSION)" "$(PREFIX)" "$(BINARY)"

clean:
	rm -f $(BINARY)
	rm -rf dist

package: release

# L61: reject anything that is not a plain package version before the
# build uses it unquoted in -ldflags.
ifeq ($(shell printf '%s' '$(VERSION)' | grep -Eq '^[0-9][0-9A-Za-z.+~]*$$' && echo ok),ok)
else
$(error Invalid VERSION '$(VERSION)' (expected e.g. 3.0.0))
endif

# M33: package-linux was listed in .PHONY but had no target.
package-linux: release

release:
	@./packaging/build-packages.sh $(VERSION)

site-build:
	cd site && npm run build

site-dev:
	cd site && npm run dev
