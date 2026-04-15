# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.25@sha256:3760478c76cfe25533e06176e983e7808293895d48d15d0981c0cbb9623834e7 AS builder

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

FROM golang:1.25-alpine@sha256:7a00384194cf2cb68924bbb918d675f1517357433c8541bac0ab2f929b9d5447 AS dev

WORKDIR /workspace

ENV PATH="/go/bin:${PATH}"

RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose git && \
    go install github.com/air-verse/air@v1.61.7

CMD ["air", "-v"]

FROM alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS final

WORKDIR /app

RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose git

COPY --from=builder /composia /usr/local/bin/composia

RUN adduser -D -u 65532 composia && \
    mkdir -p /app && \
    chown -R composia:composia /app

USER composia

ENTRYPOINT ["/usr/local/bin/composia"]
CMD ["controller", "-config", "/app/config.yaml"]
