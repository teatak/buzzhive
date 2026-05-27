# syntax=docker/dockerfile:1

FROM node:22-bookworm-slim AS admin-builder
WORKDIR /src/admin
RUN corepack enable && corepack prepare pnpm@9.15.4 --activate
COPY admin/package.json admin/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY admin/ ./
RUN pnpm build

FROM golang:1.25-bookworm AS go-builder
WORKDIR /src
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates gcc libc6-dev \
	&& rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/buzzhive ./cmd/local-proxy

FROM debian:bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl tzdata \
	&& rm -rf /var/lib/apt/lists/* \
	&& useradd --system --uid 10001 --home-dir /app buzzhive \
	&& mkdir -p /app/admin/dist /config \
	&& chown -R buzzhive:buzzhive /app
WORKDIR /app
COPY --from=go-builder /out/buzzhive /usr/local/bin/buzzhive
COPY --from=admin-builder /src/admin/dist /app/admin/dist
COPY config.example.yaml /app/config.example.yaml
USER buzzhive
EXPOSE 8787
CMD ["buzzhive", "-config", "/config/config.yaml", "-admin-dir", "/app/admin/dist"]
