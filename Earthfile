VERSION 0.6

FROM ghcr.io/troop-dev/go-kit:1.18.0-0.6.4

all:
    BUILD +lint
    BUILD +test
    BUILD +build
    BUILD +build-arm-v7
    BUILD +helm


install-deps:
	# add git to known hosts
	RUN mkdir -p /root/.ssh && \
		chmod 700 /root/.ssh && \
		ssh-keyscan github.com >> /root/.ssh/known_hosts
	RUN git config --global url."git@github.com:".insteadOf "https://github.com/"
	# add dependencies
	COPY go.mod go.sum .
	RUN --ssh go mod download -x

add-code:
	FROM +install-deps
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

test:
    FROM +vendor

	RUN go test -mod=vendor ./app/... -race -coverprofile=coverage.out -covermode=atomic -coverpkg=./app/...

    SAVE ARTIFACT coverage.out AS LOCAL coverage.out
    SAVE IMAGE --push ghcr.io/troop-dev/files-api-cache:test

lint:
	FROM +vendor
	# Runs golangci-lint with settings:
	RUN golangci-lint run --timeout 10m --skip-dirs-use-default

helm:
	FROM alpine/helm:3.7.2
	ARG VERSION="0.0.0"
	# add code
	WORKDIR /app
	RUN mkdir ./out
	COPY --dir helm .
	# build package in out dir
	RUN helm package ./helm/files-api \
		--version $VERSION \
		--app-version $VERSION \
		-d ./out/

	ENV HELM_EXPERIMENTAL_OCI=1
	ARG HELM_REPO=https://ghcr.io
	# login to helm repo
	RUN --push \
		--secret GH_USER=+secrets/GH_USER \
		--secret GH_TOKEN=+secrets/GH_TOKEN \
		echo $GH_TOKEN  | \
		helm registry login $HELM_REPO -u $GH_USER --password-stdin
	# push to repo
	RUN --push find ./out -name *.tgz | \
		xargs -I {} -n1 helm push {} oci://ghcr.io/troop-dev/helm

protogen:
	FROM +install-deps
	# copy the proto files to generate
	COPY --dir proto/ ./
	COPY buf.work.yaml buf.gen.yaml ./
	# generate the pbs
	RUN buf generate \
		--template buf.gen.yaml \
		--path proto/local/localpb
	# save artifact to
	SAVE ARTIFACT gen gen AS LOCAL gen
