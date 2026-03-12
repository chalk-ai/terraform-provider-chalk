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
	go test ./... -v


docs:  ## Generate documentation
	go generate ./tools/...

docs-validate:  ## Validate generated documentation
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate --provider-dir .

fmt:  ## Format Go and Terraform files
	gofmt -s -w .
	terraform fmt -recursive ./examples/

lint:  ## Run linter
	golangci-lint run

setup-buildkite:  ## Create or update the Buildkite PR pipeline (requires BUILDKITE_API_TOKEN; must re-run when scripts/buildkite-pipeline-pr.yml changes)
	@test -n "$(BUILDKITE_API_TOKEN)" || (echo "BUILDKITE_API_TOKEN is not set" && exit 1)
	@bash scripts/setup-buildkite.sh

.PHONY: build install test fmt lint docs docs-validate setup-buildkite help