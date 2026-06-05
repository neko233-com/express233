VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -s -w \
	-X github.com/neko233-com/express233/internal/version.Version=$(VERSION) \
	-X github.com/neko233-com/express233/internal/version.Commit=$(COMMIT) \
	-X github.com/neko233-com/express233/internal/version.Date=$(DATE)

.PHONY: test test-race lint build run-server smoke install tidy helm-lint

test:
	go test ./... -count=1

test-race:
	go test ./... -count=1 -race

lint:
	golangci-lint run ./...

build:
	go build -ldflags "$(LDFLAGS)" -o bin/express233 ./cmd/express233
	go build -ldflags "$(LDFLAGS)" -o bin/express233-server ./cmd/express233-server

run-server:
	go run -ldflags "$(LDFLAGS)" ./cmd/express233-server -addr 127.0.0.1:23380

smoke: build
	bash scripts/ci-smoke.sh

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/express233 ./cmd/express233-server

tidy:
	go mod tidy

helm-lint:
	helm lint deploy/helm/express233-server
	helm template test deploy/helm/express233-server > /dev/null
