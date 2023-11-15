server := cacher
cli := cmd/cacherc/cacherc

.PHONY: all
all: ${server} ${cli}

.PHONY: cli ${cli}
cli: ${cli}

.PHONY: server ${server}
server: ${server}

${cli} ${server}: protos/cacher/cacher.pb.go
	CGO_ENABLED=0 go build -o $@ ./$(@D)

.PHONY: gen
gen: protos/cacher/cacher.pb.go

protos/cacher/cacher.pb.go: protos/cacher/cacher.proto
	go generate ./...
	goimports -w $@

.PHONY: test
test: lint test-only

.PHONY: test-only
test-only: server
	CGO_ENABLED=1 go test -race -coverprofile=coverage.txt -covermode=atomic ${TEST_ARGS} ./...

.PHONY: run
run:
	docker-compose up --build server

.PHONY: lint
include lint.mk
