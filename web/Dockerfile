# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM --platform=$BUILDPLATFORM oven/bun:1.3.12@sha256:8956c7667fa17beb6e3c664115e66bdacfe502da5d99603626e74c197bdef160 AS deps

WORKDIR /src

COPY package.json bun.lock ./
COPY web/package.json ./web/package.json

RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

FROM deps AS builder

COPY web ./web

RUN bun run --cwd web build

FROM oven/bun:1.3.12@sha256:8956c7667fa17beb6e3c664115e66bdacfe502da5d99603626e74c197bdef160 AS final

WORKDIR /app

ENV HOST=0.0.0.0 \
    PORT=3000

COPY --from=builder /src/package.json ./package.json
COPY --from=builder /src/bun.lock ./bun.lock
COPY --from=builder /src/web/package.json ./web/package.json

RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile --production --filter @composia/web

COPY --from=builder /src/web/build ./web/build

WORKDIR /app/web

EXPOSE 3000

CMD ["node", "build"]
