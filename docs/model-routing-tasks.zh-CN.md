# 模型路由任务清单

最后核对：2026-06-05

这份清单只记录当前仓库事实，不把计划当完成。

## 已完成

- [x] 以 `models` 作为用户可见模型，不再使用 `LLM Hub` 作为产品概念。
- [x] 数据结构已改为 `providers`、`provider_keys`、`models`、`model_routes`。
- [x] `provider_accounts` 已从业务模型、API、运行时、前端类型中移除。
- [x] `provider_keys` 直接归属 `providers`。
- [x] `model_routes` 直接关联 `models` 和 `providers`，不再经过 account。
- [x] Admin UI 已拆出 Models、Providers、Provider Keys、Users、My API Keys、Runtime。
- [x] Models 页面已改为模型列表；点击模型进入模型详情。
- [x] Model 详情页管理该模型自己的 routes。
- [x] Providers 页面支持从 provider preset 添加。
- [x] Provider Keys 页面支持导入、启用、停用、删除上游 keys。
- [x] Models 页面支持从 model preset 添加，并保存上下文、输出 token、能力等模型信息。
- [x] OpenRouter 只作为 provider preset，不再作为模型 preset 家族。
- [x] `auto` 跨模型降级已移除；请求只在当前用户模型自己的 routes 内轮询。
- [x] 运行时解析模型时必须命中数据库 `models -> model_routes`，不再隐式 fallback 到 Gemini。
- [x] 对外入口已收敛为 OpenAI-compatible：`/v1/models`、`/v1/chat/completions`。
- [x] 公开 Gemini native proxy 已移除；Gemini 只作为上游 provider 使用。
- [x] OpenAI chat 入口未找到模型路由时返回明确的 model not found。
- [x] Key 冷却按 `provider + upstream_model/quota_family + provider_key` 记录。
- [x] 401 / 403 / 明确 key 错误会停用 provider key，并记录错误信息。
- [x] 429 短冷却、疑似 RPD 冷却、200 恢复健康的基本规则已实现。
- [x] OpenAI `/v1/chat/completions` 已接入模型路由。
- [x] OpenAI-compatible provider 已支持同协议透传。
- [x] OpenAI -> OpenAI / OpenAI-compatible provider 走原始请求透传，并替换为上游 provider key。
- [x] Gemini provider 仍可作为上游 provider 使用。
- [x] OpenAI 文本 chat 已接入最小 canonical request / response / stream event 中间层。
- [x] canonical message 已从单一文本扩展为 parts 结构；当前先支持 text part。
- [x] OpenAI 文本 parts 到 Gemini 文本转换已补测试。
- [x] OpenAI `image_url` data URL part 到 Gemini `inlineData` 已补最小转换和测试。
- [x] Gemini SSE 到 OpenAI stream chunk 转换已补测试。
- [x] OpenAI-compatible provider 保持同协议原样透传，包括 `tools`。
- [x] OpenAI function `tools` 到 Gemini `functionDeclarations` 已补最小转换和测试。
- [x] OpenAI `tool_choice` 到 Gemini `toolConfig.functionCallingConfig` 已补最小转换和测试。
- [x] Gemini 非流式 `functionCall` 到 OpenAI `tool_calls` 已补最小转换和测试。
- [x] OpenAI `role=tool` 工具结果到 Gemini `functionResponse` 已补最小转换和测试。
- [x] Gemini 流式 `functionCall` 到 OpenAI stream `delta.tool_calls` 已补最小转换和测试。
- [x] OpenAI 错误响应层已按 401 / 403 / 429 / 5xx 映射常见 OpenAI-compatible error type / code。
- [x] `x-goog-api-key` 已从对外认证方式中移除；对外只保留 Bearer / query key 认证。
- [x] 管理端 API 已按页面拆分刷新，不再依赖一个大 `/data`。
- [x] 本地日志已从 `local Gemini proxy` 改为 `BuzzHive listening`。
- [x] Providers preset 已补齐第一版，并去掉 Ollama / xAI 等当前不做项。
- [x] Models preset 已补齐第一版，OpenRouter 不再作为模型家族。
- [x] DeepSeek provider preset 已按官方文档改为 `https://api.deepseek.com`。
- [x] Models 页面已按卡片列表 + 模型详情页方式整理，routes 只在模型详情中管理。
- [x] Models preset 添加弹窗支持多选和 shift 区间选择。
- [x] Provider Keys / Providers / Users 等带表格页面已统一到表格外层 card 风格，避免卡片套卡片。
- [x] 删除确认已统一使用 shadcn AlertDialog，不再使用浏览器默认 confirm。
- [x] 启用 / 停用操作已统一使用图标按钮。
- [x] 表单已统一到 shadcn `Field` 官方组件和共享业务包裹组件，移除了散落的 `FieldText` / `FieldNumber` / `FieldSelect`。
- [x] Dashboard API 用量图表保留为通用 `usage_logs` 统计。
- [x] OpenAI-compatible provider 透传错误已增加本地诊断日志，便于排查上游 `finish_reason=length` / token 限制。

## 当前完成程度

- 模型路由核心：基本完成。`models -> model_routes -> providers -> provider_keys` 主链路已落地。
- OpenAI 入站第一版：基本完成。文本、stream、data URL 图片、function tools、tool choice、tool result、错误模型都有最小实现和测试。
- 同协议透传：基本完成。OpenAI-compatible 上游可直接透传请求体和响应体。
- Admin 信息架构：基本完成。一级实体已拆页，Models 只展示用户可见模型，routes 进入模型详情。
- Admin 表单结构：基本完成。表单控件统一走 shadcn `Field` 或 `form-fields` 业务包裹组件。
- Preset：基本完成。已有 provider/model 第一版；后续只需要按真实平台继续校准参数和图标细节。
- 旧结构清理：基本完成。`provider_accounts`、`google_accounts`、`model_usage_logs` 等旧表已进入清理路径；`usage_logs` 保留为通用 API 调用量统计。
- 发布准备：未完成。还需要跑完整验证、重新打镜像。

## 部分完成

- [ ] OpenAI 协议转换目前覆盖文本 chat completions、data URL 图片 part、function tools、`tool_choice`、工具结果回传、Gemini 流式 tool calls；远程图片、多模态复杂 parts 还没完整支持。
- [ ] Provider / Model preset 已有第一版，但真实模型参数和图标来源还需要长期校准。
- [ ] Runtime 冷却和 route session 已有实现，但还需要用真实 provider 压测验证策略。

## 待完成

- [ ] 第一版 OpenAI 入站验收：继续用真实 OpenAI-compatible provider 跑透非流式、流式、错误、key 轮询。
- [ ] canonical layer 继续补远程图片、多模态复杂 content parts。
- [ ] 统一 provider 错误模型，区分 key 错误、RPM、TPM、RPD、服务不可用。
- [ ] Route 选择策略补齐：priority、weight、route 健康状态、不可用 route 跳过。
- [ ] Provider Keys 批量导入体验继续完善：预览、去重、错误提示。
- [ ] 清理旧迁移兼容代码；当前阶段不保留不必要的旧结构兼容。
- [ ] Docker 镜像重新发布前跑完整验证。
- [ ] 后续阶段再做 Anthropic / Gemini 入站协议和 Anthropic 出站 provider adapter。

## 当前不做

- [ ] 不恢复 `provider_accounts`。
- [ ] 不恢复 `auto` 跨模型降级。
- [ ] 不把 Models、Providers、Keys、Routes 的 CRUD 放到同一个页面。
- [ ] 不把 OpenRouter 当成模型家族；它只是 provider。
- [ ] 第一版不提供 Gemini / Anthropic 入站协议，只提供 OpenAI-compatible 入站。

## 下一步建议

1. 跑 OpenAI 入站真实 provider 验收：非流式、流式、错误、key 轮询。
2. 继续校准 provider/model preset 的真实能力、上下文和最大输出。
3. 验收通过后再打 Docker 镜像。
