# syntax=docker/dockerfile:1.24@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:87a41d2539e5671777734e91f467499ed5eafb1fb1f77221dff2744db7a51775 AS backend-builder-base

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

FROM alpine:3.24@sha256:f5064d3e5f88c467c714509f491853ab2d951932c5cad699c0cb969dcec6f3b4 AS cli

WORKDIR /app

RUN apk add --no-cache ca-certificates
COPY --from=cli-builder /out/composia /usr/local/bin/composia

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/composia"]
CMD ["--help"]

FROM alpine:3.24@sha256:f5064d3e5f88c467c714509f491853ab2d951932c5cad699c0cb969dcec6f3b4 AS controller

WORKDIR /app

RUN apk add --no-cache ca-certificates git tini
COPY --from=controller-builder /out/composia-controller /usr/local/bin/composia-controller

USER 65532:65532
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/composia-controller"]
CMD ["-config", "/app/config.yaml"]

FROM alpine:3.24@sha256:f5064d3e5f88c467c714509f491853ab2d951932c5cad699c0cb969dcec6f3b4 AS agent

WORKDIR /app

RUN apk add --no-cache ca-certificates docker-cli docker-cli-buildx docker-cli-compose git tini
COPY --from=agent-builder /out/composia-agent /usr/local/bin/composia-agent

USER 65532:65532
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/composia-agent"]
CMD ["-config", "/app/config.yaml"]
