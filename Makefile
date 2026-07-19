PKG = github.com/o-ga09/amuse
COMMIT = $(shell git rev-parse --short HEAD)

BUILD_LDFLAGS = "-s -w -X $(PKG)/version.Revision=$(COMMIT)"

default: test

ci: test

test:
	go test ./... -coverprofile=coverage.out -covermode=count -count=1

build:
	go build -ldflags=$(BUILD_LDFLAGS) -trimpath -o amuse .

lint:
	golangci-lint run ./...

.PHONY: default ci test build lint
