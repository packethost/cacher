server := cacher-linux-x86_64
cli := cmd/cacherc/cacherc-linux-x86_64
binaries := ${server} ${cli}
all: ${binaries}

.PHONY: server cli gen test
server: ${server}
cli: ${cli}

${server}:
	CGO_ENABLED=0 GOOS=linux go build -o $@ ./$(@D)

${cli}:
	CGO_ENABLED=0 GOOS=linux go build -o $@ ./$(@D)

ifeq ($(origin GOBIN), undefined)
GOBIN := ${PWD}/bin
export GOBIN
endif

gen:
	PATH=$$GOBIN:$$PATH go install ./vendor/github.com/golang/protobuf/protoc-gen-go
	PATH=$$GOBIN:$$PATH go install ./vendor/github.com/spf13/cobra/cobra
	PATH=$$GOBIN:$$PATH go generate ./...

ifeq ($(CI),drone)
run: ${server}
	${server}
test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ${TEST_ARGS} ./...
else
run: ${binaries}
	docker-compose up -d --build db
	docker-compose up --build server cli
test:
	docker-compose up -d --build db
	docker-compose up test
endif
