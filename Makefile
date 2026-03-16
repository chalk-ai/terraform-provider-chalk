HOSTNAME = registry.terraform.io
NAMESPACE = chalk-ai
NAME = chalk
BINARY = terraform-provider-${NAME}
VERSION = 0.9.5

ifeq ($(shell uname -sm), Linux x86_64)
OS_ARCH ?= linux_amd64
else ifeq ($(shell uname -sm), Linux arm64)
OS_ARCH ?= linux_arm64
else
OS_ARCH ?= darwin_arm64  # Assume Darwin by default
endif

default: help

help:  ## Show this help
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m  %-36s\033[0m %s\n", $$1, $$2}'

build:  ## Build the provider binary
	go build -o ${BINARY}

install: build  ## Build and install the provider locally
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

test:  ## Run unit tests
	go run gotest.tools/gotestsum --format testname -- ./internal/provider/... -shuffle=on -count=1 -timeout 5m


docs:  ## Generate documentation
	go generate ./tools/...

fmt:  ## Format Go and Terraform files
	gofmt -s -w .
	terraform fmt -recursive ./examples/

lint:  ## Run linter
	golangci-lint run

setup-hooks:  ## Install git pre-commit hooks via prek (requires prek: brew install j178/tap/prek)
	prek install

release:  ## Tag and create a new release (increments patch version)
	@bash scripts/release.sh

.PHONY: build install test fmt lint docs setup-hooks release help
