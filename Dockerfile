# syntax=docker/dockerfile:1.24@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:2d6c80227255c3112a4d08e67ba98e58efd3846daf15d9d7d4c389565d881b1a AS backend-builder-base

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

FROM golang:1.26-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS dev

WORKDIR /workspace

ENV PATH="/go/bin:${PATH}"

RUN apk add --no-cache ca-certificates docker-cli docker-cli-buildx docker-cli-compose git && \
    go install github.com/air-verse/air@v1.65.3

CMD ["air", "-v"]

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS cli

WORKDIR /app

RUN apk add --no-cache ca-certificates
COPY --from=cli-builder /out/composia /usr/local/bin/composia

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia"]
CMD ["--help"]

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS controller

WORKDIR /app

RUN apk add --no-cache ca-certificates git
COPY --from=controller-builder /out/composia-controller /usr/local/bin/composia-controller

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia-controller"]
CMD ["-config", "/app/config.yaml"]

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS agent

WORKDIR /app

RUN apk add --no-cache ca-certificates docker-cli docker-cli-buildx docker-cli-compose git
COPY --from=agent-builder /out/composia-agent /usr/local/bin/composia-agent

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia-agent"]
CMD ["-config", "/app/config.yaml"]
