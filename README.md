# BuzzHive

BuzzHive is a self-hosted Gemini API proxy with multi-user API keys, Google account/key pooling, failover, automatic bad-key disabling, usage analytics, and a web admin UI.

[简体中文](README.zh-CN.md)

## Features

- Web admin UI for users, user API keys, Google accounts, Gemini keys, runtime status, and usage charts.
- User-facing API keys sent with `Authorization: Bearer <api-key>`.
- Google account and Gemini key pool with retry, cooldown, failover, and request counting.
- Automatically disables invalid/suspended upstream keys on 400 API key invalid, 401, and 403 responses.
- Natural-day usage views, draggable chart range selection, and per-key/model statistics.
- Postgres-backed users, sessions, accounts, keys, and usage logs.

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/teatak/buzzhive/main/install.sh | sh
```

Then open:

```text
http://<server-ip>:9622/admin/
```

Optional:

```bash
curl -fsSL https://raw.githubusercontent.com/teatak/buzzhive/main/install.sh | env INSTALL_DIR=/opt/buzzhive PORT=9622 IMAGE=teatak/buzzhive:latest sh
```

Run the same command again to refresh the installer files. The installer keeps `.env`, `config.yaml`, and `./pgdata`.

The installer writes a small `makefile` into the install directory:

```bash
make upgrade
make logs
make restart
make stop
```

## Docker Compose

Create `config.yaml`:

```yaml
server:
  addr: 0.0.0.0:9622
```

Create `docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: buzzhive
      POSTGRES_USER: buzzhive
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-buzzhive-change-me}
    volumes:
      - ./pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U buzzhive -d buzzhive"]
      interval: 10s
      timeout: 5s
      retries: 5

  buzzhive:
    image: ${IMAGE:-teatak/buzzhive:latest}
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "${PORT:-9622}:9622"
    environment:
      TZ: ${TZ:-Asia/Singapore}
      BUZZHIVE_DATABASE_URL: postgres://buzzhive:${POSTGRES_PASSWORD:-buzzhive-change-me}@postgres:5432/buzzhive?sslmode=disable
    volumes:
      - ./config.yaml:/config/config.yaml:ro
```

Start:

```bash
docker compose up -d
```

Upgrade:

```bash
make upgrade
```

For local source builds, use [docker-compose.dev.yml](docker-compose.dev.yml).

## Local Development

```bash
make dev
```

Admin UI:

```text
http://127.0.0.1:9622/admin/
```

Proxy endpoint example:

```text
http://127.0.0.1:9622/v1beta/models/gemini-auto:generateContent
```

On first launch, create the initial admin user in the admin UI. Then create user API keys in the UI and pass them as:

```http
Authorization: Bearer <api-key>
```

## Admin Frontend

```bash
cd admin
pnpm install
pnpm build
```

Frontend dev server:

```bash
make admin-dev
```

## Models

`gemini-auto` tries the configured model list in order. Default:

```text
gemini-3.5-flash
gemini-3-flash-preview
gemini-3.1-flash-lite
```

Override in `config.yaml`:

```yaml
models:
  auto:
    - gemini-3.5-flash
    - gemini-3-flash-preview
    - gemini-3.1-flash-lite
```

## Build And Publish

```bash
make docker-build
make docker-publish
```

`make docker-publish` bumps `VERSION` by patch and publishes both `latest` and that version for `linux/amd64` and `linux/arm64`.

Useful helpers:

```bash
make version-patch
make version-minor
make version-major
make docker-publish-current
```

## Notes

- The Go server serves `admin/dist` by default.
- Admin sessions are stored in the database for 7 days and renew when they have 3 days or less remaining.
- `config.yaml`, database files, and built frontend assets are intentionally ignored.
