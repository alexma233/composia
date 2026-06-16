# syntax=docker/dockerfile:1.24@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:792443b89f65105abba56b9bd5e97f680a80074ac62fc844a584212f8c8102c3 AS backend-builder-base

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

FROM backend-builder-base AS cli-builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/composia ./cmd/composia

FROM backend-builder-base AS controller-builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/composia-controller ./cmd/composia-controller

FROM backend-builder-base AS agent-builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/composia-agent ./cmd/composia-agent

FROM golang:1.26-alpine@sha256:7a3e50096189ad57c9f9f865e7e4aa8585ed1585248513dc5cda498e2f41812c AS dev

WORKDIR /workspace

ENV PATH="/go/bin:${PATH}"

RUN apk add --no-cache ca-certificates docker-cli docker-cli-buildx docker-cli-compose git && \
    go install github.com/air-verse/air@v1.65.3

CMD ["air", "-v"]

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS cli

WORKDIR /app

RUN apk add --no-cache ca-certificates
COPY --from=cli-builder /out/composia /usr/local/bin/composia

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia"]
CMD ["--help"]

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS controller

WORKDIR /app

RUN apk add --no-cache ca-certificates git tini
COPY --from=controller-builder /out/composia-controller /usr/local/bin/composia-controller

USER 65532:65532
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/composia-controller"]
CMD ["-config", "/app/config.yaml"]

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS agent

WORKDIR /app

RUN apk add --no-cache ca-certificates docker-cli docker-cli-buildx docker-cli-compose git tini
COPY --from=agent-builder /out/composia-agent /usr/local/bin/composia-agent

USER 65532:65532
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/composia-agent"]
CMD ["-config", "/app/config.yaml"]
