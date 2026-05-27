# BuzzHive

BuzzHive is a local Gemini API proxy with multi-user API keys, Google account/key pooling, failover, usage stats, and a small web admin UI.

## Run

```bash
cp config.example.yaml config.yaml
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

## Admin Frontend

```bash
cd admin
pnpm install
pnpm build
```

The Go server serves the built frontend from `admin/dist` by default, with an embedded fallback for basic admin access.

## Notes

- SQLite is used for users, sessions, Google accounts, Gemini API keys, and usage logs.
- Admin sessions are persisted in SQLite for 7 days and renew back to 7 days when an authenticated request arrives with 3 days or less remaining.
- `config.yaml`, database files, and built frontend assets are intentionally ignored.
