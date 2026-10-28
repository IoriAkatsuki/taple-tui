PACKAGE_ROOT := $(CURDIR)
BUILD_SCRIPT := $(PACKAGE_ROOT)/hack/build.sh
VERSION ?= $(shell git describe --tags --always --dirty=-dev 2>/dev/null || printf dev)
export VERSION

.PHONY: build install rpm package test lint lint-fix vendor

build:
	$(BUILD_SCRIPT) build

install:
	$(BUILD_SCRIPT) install

rpm:
	$(PACKAGE_ROOT)/hack/rpm.sh

package: build
	go run ./hack/package

test:
	go test ./...

lint:
	go vet ./...

lint-fix:
	gofmt -w cmd internal utils

# The local foxful-cli replacement must be mirrored in vendor.
vendor:
	go mod tidy
	go mod vendor
	go test ./internal/forkcheck

