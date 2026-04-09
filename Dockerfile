# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.25 AS builder

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
    go build -trimpath -ldflags="-s -w" -o /composia ./cmd/composia

FROM golang:1.25-alpine AS dev

WORKDIR /workspace

ENV PATH="/go/bin:${PATH}"

RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose git && \
    go install github.com/air-verse/air@v1.61.7

CMD ["air", "-v"]

FROM alpine:3.22 AS final

WORKDIR /app

RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose git

COPY --from=builder /composia /usr/local/bin/composia

RUN adduser -D -u 65532 composia && \
    mkdir -p /app && \
    chown -R composia:composia /app

USER composia

ENTRYPOINT ["/usr/local/bin/composia"]
CMD ["controller", "-config", "/app/config.yaml"]
