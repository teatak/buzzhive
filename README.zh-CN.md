# BuzzHive

BuzzHive 是一个自托管 Gemini API 代理，支持多用户 API Key、Google 账号/Key 池、失败重试、故障切换、异常 Key 自动停用、用量统计和 Web 管理后台。

[English](README.md)

## 功能

- Web 管理后台：用户、用户 API Keys、Google 账号、Gemini Keys、运行状态、用量图表。
- 对外使用用户 API Key，通过 `Authorization: Bearer <api-key>` 访问。
- Google 账号和 Gemini Key 池：支持重试、冷却、故障切换和请求计数。
- 上游 Key 异常时自动停用：包括 400 API key invalid、401、403。
- 按自然日统计用量，支持从图表拖拽选择时间范围。
- Postgres 持久化用户、会话、账号、Keys 和用量日志。

## 快速安装

```bash
curl -fsSL https://raw.githubusercontent.com/teatak/buzzhive/main/install.sh | sh
```

然后打开：

```text
http://<服务器 IP>:9622/admin/
```

可选参数：

```bash
curl -fsSL https://raw.githubusercontent.com/teatak/buzzhive/main/install.sh | env INSTALL_DIR=/opt/buzzhive PORT=9622 IMAGE=teatak/buzzhive:latest sh
```

再次运行同一条命令会刷新安装文件。安装脚本会保留 `.env`、`config.yaml` 和 `./pgdata`。

安装脚本会在安装目录写入一个小 `makefile`：

```bash
make upgrade
make logs
make restart
make stop
```

## Docker Compose

创建 `config.yaml`：

```yaml
server:
  addr: 0.0.0.0:9622
```

创建 `docker-compose.yml`：

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

启动：

```bash
docker compose up -d
```

升级：

```bash
make upgrade
```

本地源码构建使用 [docker-compose.dev.yml](docker-compose.dev.yml)。

## 本地开发

```bash
make dev
```

管理后台：

```text
http://127.0.0.1:9622/admin/
```

代理接口示例：

```text
http://127.0.0.1:9622/v1beta/models/gemini-auto:generateContent
```

首次启动时，在管理后台创建初始管理员。之后在 UI 中创建用户 API Key，并这样调用：

```http
Authorization: Bearer <api-key>
```

## 前端管理后台

```bash
cd admin
pnpm install
pnpm build
```

前端开发：

```bash
make admin-dev
```

## 模型

`gemini-auto` 会按配置的模型列表依次尝试。默认：

```text
gemini-3.5-flash
gemini-3-flash-preview
gemini-3.1-flash-lite
```

在 `config.yaml` 中覆盖：

```yaml
models:
  auto:
    - gemini-3.5-flash
    - gemini-3-flash-preview
    - gemini-3.1-flash-lite
```

## 构建和发布

```bash
make docker-build
make docker-publish
```

`make docker-publish` 会递增 `VERSION` 的 patch 版本，并发布 `latest` 和当前版本，默认包含 `linux/amd64`、`linux/arm64`。

常用命令：

```bash
make version-patch
make version-minor
make version-major
make docker-publish-current
```

## 说明

- Go 服务默认提供 `admin/dist`，并内置一个基础管理后台作为 fallback。
- 管理员会话保存在数据库中，有效期 7 天；剩余 3 天以内时自动续期。
- `config.yaml`、数据库文件和前端构建产物已加入忽略列表。
