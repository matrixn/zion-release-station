SHELL := /usr/bin/env bash

VERSION ?= 0.1.0
BUILD_NUMBER ?= $(shell date +%s)
DIST_DIR := dist
GO_BIN ?= $(shell if command -v go >/dev/null 2>&1; then command -v go; elif [ -x /tmp/zion-go/bin/go ]; then printf '%s' /tmp/zion-go/bin/go; fi)

.PHONY: help frontend backend test lint spk spk-validate deploy-nas install-nas nas-status nas-restart nas-logs nas-health clean

help:
	@printf '%s\n' \
		'make frontend       Build the Vue frontend' \
		'make backend        Build the Linux AMD64 Go binary' \
		'make test           Run backend and frontend checks' \
		'make lint           Run formatting and static checks' \
		'make spk            Build a Synology SPK in dist/' \
		'make spk-validate   Validate the SPK structure' \
		'make deploy-nas     Build and deploy through SSH using .env.nas' \
		'make nas-status     Read package status over SSH' \
		'make nas-restart    Restart package over SSH' \
		'make nas-logs       Read package logs over SSH' \
		'make nas-health     Run the remote HTTP health check'

frontend:
	./scripts/build-frontend.sh

backend:
	VERSION=$(VERSION) ./scripts/build-backend.sh

test:
	@test -n "$(GO_BIN)" || { printf '%s\n' 'Go toolchain not found. Set GO_BIN=/path/to/go.' >&2; exit 127; }
	"$(GO_BIN)" test ./...
	cd frontend && npm run build

lint:
	gofmt -w cmd internal
	@test -n "$(GO_BIN)" || { printf '%s\n' 'Go toolchain not found. Set GO_BIN=/path/to/go.' >&2; exit 127; }
	"$(GO_BIN)" vet ./...
	cd frontend && npm run typecheck

spk:
	VERSION=$(VERSION) BUILD_NUMBER=$(BUILD_NUMBER) ./scripts/build-spk.sh

spk-validate:
	./scripts/validate-spk.sh

deploy-nas:
	./scripts/deploy-nas.sh

install-nas:
	./scripts/deploy-nas.sh

nas-status:
	./scripts/nas-status.sh

nas-restart:
	./scripts/nas-restart.sh

nas-logs:
	./scripts/nas-logs.sh

nas-health:
	./scripts/nas-health.sh

clean:
	rm -rf $(DIST_DIR) build/backend build/frontend build/package frontend/dist
