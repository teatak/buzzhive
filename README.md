# BuzzHive

BuzzHive is a local Gemini API proxy with multi-user API keys, Google account/key pooling, failover, usage stats, and a small web admin UI.

## Linux Install

```bash
curl -fsSL https://raw.githubusercontent.com/teatak/buzzhive/main/install.sh | bash
```

Optional:

```bash
curl -fsSL https://raw.githubusercontent.com/teatak/buzzhive/main/install.sh | env PORT=8787 IMAGE=teatak/buzzhive:latest bash
```

Then open:

```text
http://<server-lan-ip>:8787/admin/
```

Upgrade:

```bash
curl -fsSL https://raw.githubusercontent.com/teatak/buzzhive/main/install.sh | bash
```

The installer reuses `/opt/buzzhive/.env` and the existing Postgres volume, then pulls the latest image and restarts the services.

## Run

```bash
cp config.example.yaml config.yaml
docker compose up -d postgres
go run ./cmd/local-proxy -config config.yaml
```

Admin UI:

```text
http://127.0.0.1:8787/admin/
```

Proxy endpoint example:

```text
http://127.0.0.1:8787/v1beta/models/gemini-auto:generateContent
```

On first launch, the admin UI asks you to create the initial admin user. User API keys are then created from the UI and sent as `Authorization: Bearer <api-key>`.
Google accounts and Gemini API keys are stored in Postgres and managed from the admin UI.

`gemini-auto` tries the default model list in order:

```text
gemini-3.5-flash
gemini-3-flash-preview
gemini-3.1-flash-lite-preview
gemini-2.5-flash
gemini-2.5-flash-lite
```

To override it, add:

```yaml
models:
  auto:
    - gemini-2.5-flash
    - gemini-2.5-flash-lite
```

## Admin Frontend

```bash
cd admin
pnpm install
pnpm build
```

The Go server serves the built frontend from `admin/dist` by default, with an embedded fallback for basic admin access.

## Docker

```bash
cp config.example.yaml config.yaml
docker compose up --build -d
```

Then open:

```text
http://<server-lan-ip>:8787/admin/
```

Postgres data is stored in the `pgdata` Docker volume.

Upgrade:

```bash
docker compose pull
docker compose up -d
```

Publish image:

```bash
make docker-publish
```

This bumps `VERSION` by patch, then publishes both `latest` and that version.

Optional:

```bash
make version-minor
make docker-publish-current
```

Version helpers:

```bash
make version-patch
make version-minor
make version-major
```

By default publishing includes both `linux/amd64` and `linux/arm64`.

## Notes

- Postgres is used for users, sessions, Google accounts, Gemini API keys, and usage logs.
- Admin sessions are persisted in the database for 7 days and renew back to 7 days when an authenticated request arrives with 3 days or less remaining.
- `config.yaml`, database files, and built frontend assets are intentionally ignored.
