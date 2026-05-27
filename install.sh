#!/bin/sh
set -eu

INSTALL_DIR="${INSTALL_DIR:-$PWD}"
REQUESTED_IMAGE="${IMAGE:-}"
REQUESTED_PORT="${PORT:-}"
REQUESTED_POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-}"
IMAGE="${REQUESTED_IMAGE:-teatak/buzzhive:latest}"
PORT="${REQUESTED_PORT:-8787}"
POSTGRES_PASSWORD="$REQUESTED_POSTGRES_PASSWORD"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required. Install Docker first: https://docs.docker.com/engine/install/"
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose plugin is required. Install Docker Compose first."
  exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
  mkdir -p "$INSTALL_DIR"
  chown "$(id -u):$(id -g)" "$INSTALL_DIR"
elif mkdir -p "$INSTALL_DIR" 2>/dev/null; then
  :
elif command -v sudo >/dev/null 2>&1; then
  sudo mkdir -p "$INSTALL_DIR"
  sudo chown "$(id -u):$(id -g)" "$INSTALL_DIR"
else
  echo "sudo is required to create $INSTALL_DIR. Run as root or set INSTALL_DIR to a writable path."
  exit 1
fi

cd "$INSTALL_DIR"

if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi

if [ -n "$REQUESTED_IMAGE" ]; then
  IMAGE="$REQUESTED_IMAGE"
fi
if [ -n "$REQUESTED_PORT" ]; then
  PORT="$REQUESTED_PORT"
fi
if [ -n "$REQUESTED_POSTGRES_PASSWORD" ]; then
  POSTGRES_PASSWORD="$REQUESTED_POSTGRES_PASSWORD"
fi

if [ -z "$POSTGRES_PASSWORD" ]; then
  if command -v openssl >/dev/null 2>&1; then
    POSTGRES_PASSWORD="$(openssl rand -hex 16)"
  else
    POSTGRES_PASSWORD="buzzhive-change-me"
  fi
fi

cat > .env <<EOF
IMAGE=${IMAGE}
PORT=${PORT}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
EOF
chmod 600 .env

cat > config.yaml <<EOF
server:
  addr: 0.0.0.0:8787
EOF
chmod 644 config.yaml

cat > docker-compose.yml <<EOF
services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: buzzhive
      POSTGRES_USER: buzzhive
      POSTGRES_PASSWORD: \${POSTGRES_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U buzzhive -d buzzhive"]
      interval: 10s
      timeout: 5s
      retries: 5

  buzzhive:
    image: \${IMAGE}
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "\${PORT}:8787"
    environment:
      BUZZHIVE_DATABASE_URL: postgres://buzzhive:\${POSTGRES_PASSWORD}@postgres:5432/buzzhive?sslmode=disable
    volumes:
      - ./config.yaml:/config/config.yaml:ro

volumes:
  pgdata:
EOF

docker compose pull
docker compose up -d

echo
echo "BuzzHive is running."
echo "Install dir: $INSTALL_DIR"
echo "Open: http://<server-ip>:${PORT}/admin/"
