# 模型与供应商预设

核验日期：2026-09-14。实现位于 `internal/model_presets.go` 与 `internal/provider_presets.go`。

预设用于管理后台导入新模型、创建供应商。已有模型、路由、供应商端点及 Credits 费率由数据库保存；重复导入同名模型会跳过，不会覆盖已有配置。新增模型后仍需在模型详情中配置上游路由。更新源码后需要重新构建、部署服务，线上才会出现新预设。

## 本次清单

| 厂商 | 预设调整 | 上下文 / 最大输出（tokens） | 官方来源 |
| --- | --- | --- | --- |
| OpenAI | 新增 GPT-6 Astra；保留 GPT-5.6 Sol/Terra/Luna、5.5、5.4、5.4 Mini/Nano；修正音频能力与 5.5/5.4 上下文 | 1,050,000 / 128,000；Mini/Nano 为 400,000 / 128,000 | [模型目录](https://developers.openai.com/api/docs/models)、[GPT-5.5](https://developers.openai.com/api/docs/models/gpt-5.5)、[GPT-5.4](https://developers.openai.com/api/docs/models/gpt-5.4) |
| Anthropic | Fable 5 → Fable 5.1；Opus 4.8 → Opus 5；Sonnet 4.6 → Sonnet 5；保留 Haiku 4.5 | 前三者 1,000,000 / 128,000；Haiku 200,000 / 64,000 | [模型目录](https://platform.claude.com/docs/en/models/overview) |
| Gemini | Flash 3.5 → 3.8；新增 3.5 Flash-Lite；Pro ID 改为 `gemini-3.1-pro-preview` | 1,048,576 输入 / 65,536 输出 | [3.8 Flash](https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash)、[3.5 Flash-Lite](https://ai.google.dev/gemini-api/docs/models/gemini-3.5-flash-lite)、[3.1 Pro Preview](https://ai.google.dev/gemini-api/docs/models/gemini-3.1-pro-preview) |
| DeepSeek | 使用 `deepseek-flash` 官方名称，启用图像能力；保留 V4 Pro 文本模型；新增供应商 Responses 端点 | 1,000,000 / 384,000 | [模型与价格](https://api-docs.deepseek.com/zh-cn/quick_start/pricing/)、[Responses](https://api-docs.deepseek.com/zh-cn/guides/responses_api/) |
| Qwen | 新增 3.8 Flash；3.7 Max → `qwen3.8-max-0902` 快照；保留 3.7 Plus；均支持图像，不标记音频输入 | 1,000,000 / 131,072 | [3.8 Flash](https://help.aliyun.com/zh/model-studio/qwen3-8-flash)、[3.8 Max](https://help.aliyun.com/zh/model-studio/qwen3-8-max)、[3.7 Plus](https://help.aliyun.com/zh/model-studio/qwen3-7-plus) |
| Kimi | 新增 K2.7 Code、K2.7 Code HighSpeed；保留 K3、K2.6；修正 token 元数据 | K3 1,048,576 / 1,048,576；K2.6 262,144 / 262,144；K2.7 系列 262,144 / 未设置 | [模型目录](https://platform.kimi.com/docs/models)、[K3 参数](https://platform.kimi.com/docs/guide/kimi-k3-quickstart)、[K2.7 参数](https://platform.kimi.com/docs/guide/kimi-k2-7-code-quickstart)、[K2.6 基准参数](https://platform.kimi.com/docs/guide/benchmark-best-practice) |
| GLM | 新增 5.3 Flash（图像）、5.3（文本）；保留 5.2、5.1；修正 5.1 长度 | 5.3 系列、5.2 为 1,000,000 / 128,000；5.1 为 200,000 / 128,000 | [5.3 Flash](https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash)、[5.3](https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3)、[5.1](https://docs.z.ai/guides/llm/glm-5.1) |
| MiMo | 保留 V2.5 与 V2.5 Pro，修正上下文精确值 | 1,048,576 / 131,072 | [小米官方接入示例](https://github.com/XiaomiMiMo/awesome-mimo-agent/blob/main/docs/pi_mono.md) |

## 参数与协议边界

- `MaxInputTokens` 有独立官方上限时单独填写。Qwen 三款均为 991,808；思考模式的输入上限更低，为 983,616。其他共享上下文模型的输入与输出必须合计符合窗口约束，表格两列不能相加作为可用总长度。Gemini 官方提供独立输入上限，预设沿用该值作为上下文展示。
- Kimi K3 默认输出预算为 131,072，但官方允许 `max_completion_tokens` 最高设置为 1,048,576；请求输入加输出预算必须在上下文内。K2.6 官方基准参数支持 256K 输出预算，同样受共享窗口限制。
- K2.7 官方指南只明确默认输出 32,768，没有明确独立最大输出值；因此 Code 与 HighSpeed 的 `MaxOutputTokens` 暂为 `0`（未设置），不把默认值或第三方托管平台的限制当成官方上限。
- GPT-6 Astra 工具调用必须走 `openai-responses`；创建路由时可固定上游协议为 Responses。`auto` 仍按来访协议优先透传，不因预设名称自动切换协议。Astra 不接受 `temperature`、`top_p`、`top_logprobs`，具体要求见 [官方迁移说明](https://developers.openai.com/api/docs/guides/latest-model)。
- DeepSeek 新供应商预设包含 `openai` 与 `openai-responses`，Base URL 均为 `https://api.deepseek.com`。Responses 请求在自动路由下选 Responses 端点；Chat 请求选 Chat 端点。BuzzHive 保留入口 `/v1/responses` 路径；2026-09-14 无 Key 的空请求探测 `/responses` 与 `/v1/responses` 均返回 401，未进行付费推理验证。
- DeepSeek Responses 为无状态兼容接口，不提供 OpenAI 的完整持久化与内置工具能力；详见上方官方兼容性表。GLM 5.3 系列与 Kimi K3/K2.7、Claude Fable 5.1 均有始终开启思考的限制。预设描述提示模型用途，路由和参数转换仍使用既有配置与实现。

本次未改管理员设置的 Credits 费率，也未自动迁移线上模型或路由。
