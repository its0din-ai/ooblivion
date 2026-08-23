COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
DESCRIBE ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
VERSION ?= $(shell echo "$(DESCRIBE)" | sed -E 's/-[0-9]+-g/-g/')
BUILDDATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X ooblivion/internal/version.Version=$(VERSION) -X ooblivion/internal/version.Commit=$(COMMIT) -X ooblivion/internal/version.BuildDate=$(BUILDDATE)

.PHONY: build docker-build run up test

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/ooblivion ./cmd/ooblivion

run: build
	./bin/ooblivion

docker-build:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILDDATE=$(BUILDDATE) -t ooblivion:local .

up:
	OOB_VERSION="$(VERSION)" OOB_COMMIT="$(COMMIT)" docker compose up -d --build

test:
	./e2e.sh
