# syntax=docker/dockerfile:1.27@sha256:bde3983e9c939224420ddaf6b784cc30e09b035a4dea01f581230c50809f372e
FROM --platform=$BUILDPLATFORM denoland/deno:2.9.7@sha256:fa335acdf6b72106eda2cb6a8cb5f4187e7630e357467489db4b2e7352d5e432 AS deps

WORKDIR /src

COPY deno.json deno.lock package.json ./
COPY web/package.json ./web/package.json

RUN --mount=type=cache,id=composia-web-deno-deps-v2,target=/deno-dir,sharing=locked \
    deno install --frozen

FROM deps AS builder

COPY web ./web

RUN deno task web:build

FROM denoland/deno:2.9.7@sha256:fa335acdf6b72106eda2cb6a8cb5f4187e7630e357467489db4b2e7352d5e432 AS prod-deps

WORKDIR /app

COPY deno.json deno.lock package.json ./
COPY web/package.json ./web/package.json

RUN --mount=type=cache,id=composia-web-deno-prod-v2,target=/deno-dir,sharing=locked \
    deno install --frozen --prod

FROM denoland/deno:distroless-2.9.2@sha256:04cb192ac7445056d009530fbd228fb15fa42fbd29d38c95bbd099b8b902ec7a AS final

WORKDIR /app

ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=3000

COPY --from=prod-deps /app/package.json ./package.json
COPY --from=prod-deps /app/node_modules ./node_modules
COPY --from=prod-deps /app/web/package.json ./web/package.json
COPY --from=prod-deps /app/web/node_modules ./web/node_modules
COPY --from=builder /src/web/.deno-deploy ./web/.deno-deploy
COPY --from=builder /src/web/compression.mjs ./web/compression.mjs
COPY --from=builder /src/web/server.mjs ./web/server.mjs

WORKDIR /app/web

EXPOSE 3000

USER 65532:65532

CMD ["run", "--allow-env", "--allow-net", "--allow-read=/app", "--no-prompt", "server.mjs"]
