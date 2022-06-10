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
    RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.46.2
    RUN ls -la $(which golangci-lint)

protogen:
    FROM +golang-base

    # copy the proto files to generate
    COPY --dir proto/ ./
    COPY buf.work.yaml buf.gen.yaml ./
    # generate the pbs
    RUN buf generate \
        --template buf.gen.yaml \
        --path proto/local/parti

    # save artifact to
    SAVE ARTIFACT partipb partipb AS LOCAL partipb
