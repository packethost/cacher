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

test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ${TEST_ARGS} ./...

run: ${binaries}
	docker-compose up --build server
