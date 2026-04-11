# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM oven/bun:1.3.8 AS deps

WORKDIR /src

COPY package.json bun.lock ./
COPY web/package.json ./web/package.json

RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

FROM deps AS builder

COPY web ./web

RUN bun run --cwd web build

FROM oven/bun:1.3.8 AS final

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
