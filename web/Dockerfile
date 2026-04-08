FROM --platform=$BUILDPLATFORM oven/bun:1.3.8 AS builder

WORKDIR /src

COPY package.json bun.lock ./
COPY web/package.json ./web/package.json

RUN bun install --frozen-lockfile

COPY web ./web

RUN bun run --cwd web build

FROM node:22-alpine AS final

WORKDIR /app

ENV HOST=0.0.0.0 \
    PORT=3000

COPY --from=builder /src/web/build ./build
COPY --from=builder /src/web/package.json ./package.json

EXPOSE 3000

CMD ["node", "build"]
