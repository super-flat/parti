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

deps:
	# add git to known hosts
	RUN mkdir -p /root/.ssh && \
		chmod 700 /root/.ssh && \
		ssh-keyscan github.com >> /root/.ssh/known_hosts
	RUN git config --global url."git@github.com:".insteadOf "https://github.com/"
	# add dependencies
	COPY go.mod go.sum .
	RUN --ssh go mod download -x

add-code:
	FROM +deps
	# add code
	COPY --dir app .

vendor:
	FROM +add-code
	RUN --ssh go mod vendor

compile:
	FROM +vendor
	ARG GOOS=linux
	ARG GOARCH
	ARG GOARM
	RUN go build -mod=vendor -o bin/server ./app/main.go
	SAVE ARTIFACT bin/server /bin/server
	# SAVE IMAGE --push raft-poc-cache:compile

build:
	FROM alpine:3.13.6
	RUN apk add --no-cache libc6-compat
	ARG VERSION=dev
	WORKDIR /app
	COPY +compile/bin/server .
	RUN chmod +x ./server
    # RUN mkdir .db && chown 777 ./.db
    # TODO: use nobody
	# USER nobody
    USER root
	# ENTRYPOINT ./server run
	SAVE IMAGE --push raft-poc:${VERSION}

build-arm-v7:
	# THIS TARGET IS ONLY FOR BUILDING ARM64 IMAGES FROM AN
	# AMD64 HOST (Github Actions)
	FROM --platform=linux/arm64 alpine:3.13.6
	RUN apk add --no-cache libc6-compat
	ARG VERSION=dev
	WORKDIR /app
	COPY --platform=linux/amd64 --build-arg GOARCH=arm --build-arg GOARM=7 +compile/bin/server .
	RUN chmod +x ./server
	USER nobody
	ENTRYPOINT ./server run
	SAVE IMAGE --push ghcr.io/troop-dev/files-api:${VERSION}
