VERSION 0.7
PROJECT super-flat/tools

FROM tochemey/docker-go:1.20.4-0.8.0

# run a PR branch is created
pr:
  PIPELINE
  TRIGGER pr main
  BUILD +lint

# run on when a push to main is made
main:
  PIPELINE
  TRIGGER push main
  BUILD +lint

protogen:
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
    COPY --dir discovery ./
    COPY --dir log ./
    COPY --dir internal ./


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
    FROM alpine:3.20.3
    WORKDIR /app
    COPY +k8s-base/example .
    ENTRYPOINT ./example server
    SAVE IMAGE parti-example:dev

local-build:
    FROM alpine:3.20.3
    WORKDIR /app
    COPY ./.tmp/example .
    ENTRYPOINT ./example server
    SAVE IMAGE parti-example:dev
