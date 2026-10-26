PACKAGE_ROOT := $(CURDIR)
BUILD_SCRIPT := $(PACKAGE_ROOT)/hack/build.sh

.PHONY: build install rpm test lint lint-fix vendor

build:
	$(BUILD_SCRIPT) build

install:
	$(BUILD_SCRIPT) install

rpm:
	$(PACKAGE_ROOT)/hack/rpm.sh

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

