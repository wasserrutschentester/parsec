GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./.git/*")
GO_PACKAGES ?= $(shell go list ./... | grep -v /vendor/)

# Format the version string using the semver-compliant version script
VERSION ?= $(shell ./scripts/version.sh)

LDFLAGS = -X codeberg.org/upPollo/parsec/cmd.Version=$(VERSION)

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
