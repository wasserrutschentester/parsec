GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./.git/*")
GO_PACKAGES ?= $(shell go list ./... | grep -v /vendor/)

# Get raw version from git
GIT_VER := $(shell git describe --tags --always --match "v[0-9]*.[0-9]*.[0-9]*" --dirty 2>/dev/null || echo "v0.0.0-unknown")

# Format the version string (untrimmed hash is fine):
# 1. If it doesn't start with 'v' (just a commit hash), prepend 'v0.0.0-'
# 2. Replace '-dirty' with '+dirty' for semver compliance
VERSION ?= $(shell echo $(GIT_VER) | sed -e '/^v/! s/^/v0.0.0-/' -e 's/-dirty/+dirty/')

LDFLAGS = -X codeberg.org/n0ne/parsec/cmd.Version=$(VERSION)

.PHONY: init
init: ## Initialize local development environment
	git config core.hooksPath scripts
	chmod +x scripts/commit-msg

.PHONY: all
all: lint

.PHONY: build
build: ## Build the binary
	go build -ldflags="$(LDFLAGS)" -o parsec main.go

vendor:
	go mod tidy
	go mod vendor

format: install-gofumpt ## Format source code
	@gofumpt -extra -w ${GOFILES_NOVENDOR}

formatcheck:
	@([ -z "$(shell gofumpt -d $(GOFILES_NOVENDOR) | head)" ]) || (echo "Source is unformatted"; exit 1)

install-linter:
	@hash golangci-lint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest ; \
	fi ; \

install-gofumpt:
	@hash gofumpt > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go install mvdan.cc/gofumpt@latest; \
	fi ; \

.PHONY: clean
clean:
	go clean -i ./...

.PHONY: lint
lint: install-linter ## Lint code
	@echo "Running golangci-lint"
	golangci-lint run

.PHONY: vet
vet:
	@echo "Running go vet..."
	@go vet $(GO_PACKAGES)

.PHONY: test
test:
	go test -race ./...
