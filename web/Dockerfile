# syntax=docker/dockerfile:1.25@sha256:0adf442eae370b6087e08edc7c50b552d80ddf261576f4ebd6421006b2461f12
FROM --platform=$BUILDPLATFORM denoland/deno:2.9.2@sha256:69a9f3033e3f381770452648b1cbf4ae96ea409f7ef24f30afeff3d380705f62 AS deps

WORKDIR /src

COPY deno.json deno.lock package.json ./
COPY web/package.json ./web/package.json

RUN --mount=type=cache,id=composia-web-deno-deps-v2,target=/deno-dir,sharing=locked \
    deno install --frozen

FROM deps AS builder

COPY web ./web

RUN deno task web:build

FROM denoland/deno:2.9.2@sha256:69a9f3033e3f381770452648b1cbf4ae96ea409f7ef24f30afeff3d380705f62 AS prod-deps

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
COPY --from=builder /src/web/server.mjs ./web/server.mjs

WORKDIR /app/web

EXPOSE 3000

USER 65532:65532

CMD ["run", "--allow-env", "--allow-net", "--allow-read=/app", "--no-prompt", "server.mjs"]
