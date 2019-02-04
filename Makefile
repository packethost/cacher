binary := cacher-linux-x86_64
all: ${binary}

.PHONY: ${binary} gen test
${binary}:
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
run: ${binary}
	${binary}
test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ${TEST_ARGS} ./...
else
run: ${binary}
	docker-compose up -d --build db
	docker-compose up --build app
test:
	docker-compose up -d --build db
	docker-compose up test
endif
