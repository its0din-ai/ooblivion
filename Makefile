VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILDDATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X ooblivion/internal/version.Version=$(VERSION) -X ooblivion/internal/version.Commit=$(VERSION) -X ooblivion/internal/version.BuildDate=$(BUILDDATE)

.PHONY: build docker-build run test

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/ooblivion ./cmd/ooblivion

run: build
	./bin/ooblivion

docker-build:
	docker build --build-arg VERSION=$(VERSION) --build-arg BUILDDATE=$(BUILDDATE) -t ooblivion:local .

test:
	./e2e.sh
