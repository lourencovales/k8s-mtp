export GOPRIVATE := git.assilvestrar.club

MODULE := git.assilvestrar.club/lourenco/k8s-mtp
BIN_DIR := bin
BINARIES := api controller webhook
REGISTRY := git.assilvestrar.club/lourenco/k8s-mtp
VERSION ?= latest

.PHONY: all build test lint ko-build ko-local clean
.DEFAULT_GOAL := build

all: test lint build

build:
	mkdir -p $(BIN_DIR)
	for bin in $(BINARIES); do \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN_DIR)/$$bin ./cmd/$$bin/; \
	done

test:
	go test ./... -count=1

lint:
	go vet ./...

ko-build:
	KO_DOCKER_REPO=$(REGISTRY) ko build --bare --platform=linux/amd64 ./cmd/api ./cmd/controller ./cmd/webhook

ko-local:
	ko build --local --bare

clean:
	rm -rf $(BIN_DIR)
