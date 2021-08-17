server := cacher-linux-x86_64
cli := cmd/cacherc/cacherc-linux-x86_64
binaries := ${server} ${cli}
all: ${binaries}

.PHONY: server ${binaries} cli gen test
server: ${server}
cli: ${cli}

${cli} ${server}: protos/cacher/cacher.pb.go
	CGO_ENABLED=0 GOOS=linux go build -o $@ ./$(@D)

gen: protos/cacher/cacher.pb.go

protos/cacher/cacher.pb.go: protos/cacher/cacher.proto
	go generate ./...
	goimports -w $@

test: lint test-only

# NOTE: -race requires CGO_ENABLED=1
test-only:
	CGO_ENABLED=1 go test -race -coverprofile=coverage.txt -covermode=atomic ${TEST_ARGS} ./...

run: ${binaries}
	docker-compose up --build server

GOLINT_VERSION ?= v1.41.1
HADOLINT_VERSION ?= v2.6.1
SHELLCHECK_VERSION ?= v0.7.2
OS := $(shell uname)
LOWER_OS  = $(shell echo $OS | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)

# TODO(tstromberg): Change hadolint/shell checks to be fatal once issues can be addressed
lint: out/linters/shellcheck-$(SHELLCHECK_VERSION)/shellcheck out/linters/hadolint-$(HADOLINT_VERSION) out/linters/golangci-lint-$(GOLINT_VERSION)
	out/linters/golangci-lint-$(GOLINT_VERSION) run
	out/linters/shellcheck-$(SHELLCHECK_VERSION)/shellcheck $(shell find . -name "*.sh") || true
	out/linters/hadolint-$(HADOLINT_VERSION) $(shell find . -name "*Dockerfile") || true

# Shell script linter
out/linters/shellcheck-$(SHELLCHECK_VERSION)/shellcheck:
	mkdir -p out/linters
	curl -sfL https://github.com/koalaman/shellcheck/releases/download/v0.7.2/shellcheck-$(SHELLCHECK_VERSION).$(OS).$(ARCH).tar.xz | tar -C out/linters -xJf -

# Dockerfile linter
out/linters/hadolint-$(HADOLINT_VERSION):
	mkdir -p out/linters
	curl -sfL https://github.com/hadolint/hadolint/releases/download/v2.6.1/hadolint-$(OS)-$(ARCH) > out/linters/hadolint-$(HADOLINT_VERSION)
	chmod u+x out/linters/hadolint-$(HADOLINT_VERSION)

# Go linter
out/linters/golangci-lint-$(GOLINT_VERSION):
	mkdir -p out/linters
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b out/linters $(GOLINT_VERSION)
	mv out/linters/golangci-lint out/linters/golangci-lint-$(GOLINT_VERSION)
