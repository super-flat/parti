VERSION 0.6

FROM golang:1.18.2-alpine

golang-base:
    WORKDIR /app

    # install gcc dependencies into alpine for CGO
    RUN apk add gcc musl-dev curl git openssh

    # install docker tools
    # https://docs.docker.com/engine/install/debian/
    RUN apk add --update --no-cache docker

    # install the go generator plugins
    RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    RUN export PATH="$PATH:$(go env GOPATH)/bin"

    # install buf from source
    RUN GO111MODULE=on GOBIN=/usr/local/bin go install github.com/bufbuild/buf/cmd/buf@v1.4.0

    # install linter
    # binary will be $(go env GOPATH)/bin/golangci-lint
    RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.50.0
    RUN ls -la $(which golangci-lint)

protogen:
    FROM +golang-base

    # copy the proto files to generate
    COPY --dir proto/ ./
    COPY buf.work.yaml buf.gen.yaml ./
    # generate the pbs
    RUN buf generate \
        --template buf.gen.yaml \
        --path proto/parti

    # save artifact to
    SAVE ARTIFACT pb pb AS LOCAL pb

deps:
    FROM +golang-base

    WORKDIR /app

    COPY go.mod go.sum ./

    RUN go mod download
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

code:
    FROM +deps

    WORKDIR /app

    COPY --dir +protogen/pb ./
    COPY --dir cluster ./


    SAVE ARTIFACT /app /files

vendor:
    FROM +code

    WORKDIR /app

    RUN go mod tidy && go mod vendor

    SAVE ARTIFACT /app /files

lint:
    FROM +vendor

    COPY .golangci.yml ./

    # Runs golangci-lint with settings:
    RUN golangci-lint run -v

k8s-base:
    FROM +vendor

    COPY --dir example .
    RUN go mod tidy && go mod vendor
    RUN go build -o example ./example
    SAVE ARTIFACT ./example /

k8s-build:
    FROM alpine:3.13.6
    COPY +k8s-base/example .
    ENTRYPOINT ./example server
    SAVE IMAGE parti-example:dev
