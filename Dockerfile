# syntax=docker/dockerfile:1.23@sha256:2780b5c3bab67f1f76c781860de469442999ed1a0d7992a5efdf2cffc0e3d769
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:b54cbf583d390341599d7bcbc062425c081105cc5ef6d170ced98ef9d047c716 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/composia ./cmd/composia && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/composia-controller ./cmd/composia-controller && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/composia-agent ./cmd/composia-agent

FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS dev

WORKDIR /workspace

ENV PATH="/go/bin:${PATH}"

RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose git && \
    go install github.com/air-verse/air@v1.65.1

CMD ["air", "-v"]

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS cli

WORKDIR /app

RUN apk add --no-cache ca-certificates
COPY --from=builder /out/composia /usr/local/bin/composia

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia"]
CMD ["--help"]

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS controller

WORKDIR /app

RUN apk add --no-cache ca-certificates git
COPY --from=builder /out/composia-controller /usr/local/bin/composia-controller

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia-controller"]
CMD ["-config", "/app/config.yaml"]

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS agent

WORKDIR /app

RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose git
COPY --from=builder /out/composia-agent /usr/local/bin/composia-agent

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia-agent"]
CMD ["-config", "/app/config.yaml"]
