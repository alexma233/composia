# syntax=docker/dockerfile:1.24@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:d184d9be4c13614e28498d632eeaaac704d662f18ad357e1df74a44424236cea AS backend-builder-base

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

FROM golang:1.26-alpine@sha256:a6a091eac01ceac4b97496fe2957a49b6cdd83365337d5f46f6f73710424e805 AS dev

WORKDIR /workspace

ENV PATH="/go/bin:${PATH}"

RUN apk add --no-cache ca-certificates docker-cli docker-cli-buildx docker-cli-compose git && \
    go install github.com/air-verse/air@v1.65.3

CMD ["air", "-v"]

FROM alpine:3.24@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4 AS cli

WORKDIR /app

RUN apk add --no-cache ca-certificates
COPY --from=cli-builder /out/composia /usr/local/bin/composia

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia"]
CMD ["--help"]

FROM alpine:3.24@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4 AS controller

WORKDIR /app

RUN apk add --no-cache ca-certificates git tini
COPY --from=controller-builder /out/composia-controller /usr/local/bin/composia-controller

USER 65532:65532
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/composia-controller"]
CMD ["-config", "/app/config.yaml"]

FROM alpine:3.24@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4 AS agent

WORKDIR /app

RUN apk add --no-cache ca-certificates docker-cli docker-cli-buildx docker-cli-compose git tini
COPY --from=agent-builder /out/composia-agent /usr/local/bin/composia-agent

USER 65532:65532
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/composia-agent"]
CMD ["-config", "/app/config.yaml"]
