IMAGE ?= energything
CI_BUILD_ID ?= local
DOCKER ?= docker
GO ?= go

.PHONY: build build-go-local test vet test-race docker-build
build: build-go-local

build-go-local:
	$(GO) run ./scripts/build.go -build-id "$(CI_BUILD_ID)" -go "$(GO)"

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

# Requires a cgo-capable host/compiler; independent of production cross-builds.
test-race:
	$(GO) test -race ./...

docker-build: build-go-local
	$(DOCKER) build -t $(IMAGE):$(CI_BUILD_ID) .
