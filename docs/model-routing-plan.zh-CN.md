# 模型路由计划

BuzzHive 的目标是以用户可见模型为核心。用户只关心自己请求的模型，BuzzHive 在后台把这个模型路由到不同提供方和上游模型。

## 目标

- 第一版客户端只用 OpenAI-compatible 协议调用 BuzzHive。
- 内部保留 canonical request / response 层，为后续 Gemini、Anthropic 入站和跨协议转换做准备。
- 用户 API Key 只绑定 BuzzHive 用户，不暴露上游平台 Key。
- 同协议转发优先直通，跨协议调用才做协议转换。
- 模型路由、提供方、上游 Key、冷却、错误和统计都落库或进入短期运行态。
- 不再做 `auto` 模型降级；同一个用户模型只在自己的路由内轮询。

## 核心原则

1. 用户请求的是 `models.name`，不是 provider 或上游 key。
2. 模型下面配置 routes；route 指向 provider 和 upstream model。
3. provider 下面直接管理 provider keys，不再引入额外账号分组。
4. Key 冷却和错误状态按 `provider + upstream_model + key` 记录。
5. 页面拆分按一级实体来：Models、Providers、Provider Keys、Users、My API Keys、Runtime。

## 请求链路

```text
Client
  |
  | OpenAI-compatible protocol
  v
Inbound Protocol Layer
  |
  v
Auth + Model Router
  |
  | same protocol
  +--------------------> Raw Provider Proxy
  |
  | cross protocol
  v
Canonical Request Layer
  |
  v
Provider Adapter
  |
  v
Upstream Provider
```

示例：

```text
OpenAI request model=mimo-v2.5
  -> BuzzHive model=mimo-v2.5
  -> route: provider=openrouter, upstream_model=mimo/mimo-v2.5
  -> OpenRouter
```

## 数据模型

Postgres 是配置真相源：

```text
providers         提供方实例，例如 gemini、openai、openrouter、mimo
provider_keys     上游 API Key，直接归属 provider
models            用户可见模型和模型信息
model_routes      模型到 provider/upstream model 的路由
users             管理端用户
user_api_keys     用户对外使用的 BuzzHive API Key
sessions          管理端登录会话
usage_logs        通用 API 调用量日志
```

### providers

- id
- name
- type
- preset_id
- base_url
- enabled
- created_at
- updated_at

`type` 表示出站协议能力：

- `gemini`
- `openai`
- `anthropic`
- `openai-compatible`

### provider_keys

- id
- provider_id
- name
- secret
- secret_hint
- enabled
- priority
- weight
- labels
- disabled_status
- disabled_error_code
- disabled_error_message
- disabled_error_body
- disabled_at
- created_at
- updated_at

Key 不再挂到账号分组。需要表达来源、项目、订阅账号时，先用 `name` 和 `labels` 承载，后续如果确实需要资产分组，再单独设计，不回填到模型路由主路径。

### models

- id
- name
- display_name
- description
- context_window
- max_input_tokens
- max_output_tokens
- capabilities
- selection_policy
- enabled
- created_at
- updated_at

`models.name` 是用户请求时看到的模型名。

### model_routes

- id
- model_id
- provider_id
- upstream_model
- quota_family
- enabled
- priority
- weight
- created_at
- updated_at

route 是模型和 provider 的关系。一个模型可以有多个 route，例如：

```text
model: mimo-v2.5
  route 1 -> provider=mimo, upstream_model=mimo-v2.5, weight=8
  route 2 -> provider=openrouter, upstream_model=mimo/mimo-v2.5, weight=2
```

## Preset

Preset 只负责辅助创建，不保存密钥。

### Provider Preset

- 名称
- 类型
- 默认 Base URL

Provider preset 只生成 provider 基础信息。上游 Key 在 Provider Keys 页面单独导入。

当前约定：

- DeepSeek 使用官方推荐的 `https://api.deepseek.com`；它兼容 `/v1`，但 preset 不带 `/v1`。
- OpenRouter 是 provider，不是模型家族。
- Ollama / 无 Key 本地 provider 暂不作为第一版目标。

### Model Preset

- family
- name
- display_name
- description
- context_window
- max_input_tokens
- max_output_tokens
- capabilities
- selection_policy

Model preset 只保存 BuzzHive 需要的模型信息：模型名、展示名、简介、上下文、最大输出和能力。价格、周 token、OpenRouter slug 等外部展示字段不入库。

OpenRouter 不是模型家族，它是 provider。通过 OpenRouter 路由到的模型，模型 preset 应归属真实模型家族，例如 OpenAI、DeepSeek、Qwen、GLM。

## Key 选择

路由时按这个顺序执行：

1. 认证 BuzzHive 用户 API Key。
2. 解析入站协议和请求模型名。
3. 查 `models.name`。
4. 按模型的 `selection_policy` 选择 `model_routes`。
5. 根据 route 的 provider 和 upstream model 选择可用 provider key。
6. 同协议直通，跨协议转换。
7. 上游成功则记录用量；上游错误则记录 key 错误或冷却。

Key 选择不跨模型降级。`mimo-v2.5` 只能在 `mimo-v2.5` 的 routes 中轮询，不会自动切到 `gemini-3.5-flash`。

## 冷却

短期运行态可以放内存，后续可迁到 Redis。

```text
key_cooldown:
  provider_id + upstream_model + provider_key_id -> expires_at

rpd_like:
  provider_id + upstream_model + provider_key_id -> expires_at

route_session:
  user_api_key_id + model_id -> model_route_id
```

规则：

- 429：当前 `provider + upstream_model + key` 进入短冷却。
- 经过 120 秒冷却后首次请求仍然 429：标记为疑似 RPD，冷却 1 小时。
- 200：清除该 `provider + upstream_model + key` 的冷却和疑似 RPD。
- 401 / 403 / 明确 key 错误：停用该 provider key，并记录错误状态和错误正文。

## Sticky Routing

不要求客户端传自定义 header。第一版规则：

```text
同一个用户 API Key 请求同一个 model
  -> 在 TTL 内固定到同一个 model_route
  -> route 不可用时重新选择
```

这样可以减少同一段会话在多个 provider 间跳动，降低 provider 侧缓存失效的概率。

## Admin UI 信息架构

- Models：用户可见模型列表。点击模型进入详情页管理 routes。
- Providers：提供方实例列表，可从 preset 添加。
- Provider Keys：导入和管理上游 keys。
- Users：管理端用户。
- My API Keys：用户自己的 BuzzHive API Keys。
- Runtime：短期运行态、冷却、错误。

禁止把 Models、Providers、Keys、Routes 的 CRUD 全堆在一个页面。列表页只展示核心信息和高频操作；跨实体配置进入详情页或独立页面。

## 第一版验收

- Models 页面可从 preset 添加模型。
- Model 详情页可新增、编辑、停用、删除 routes。
- Providers 页面可从 preset 添加 provider。
- Provider Keys 页面可批量导入 keys，并可停用、启用、删除。
- OpenAI `/v1/models` 可返回用户可见模型。
- OpenAI `/v1/chat/completions` 可通过 route 转发到 OpenAI-compatible provider。
- OpenAI `/v1/chat/completions` 可通过 canonical 层转发到 Gemini provider。
- 运行态不依赖 YAML 中的模型列表。
