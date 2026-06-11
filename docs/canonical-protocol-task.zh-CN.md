# Canonical 协议转换层任务

最后核对：2026-06-12

官方协议核对来源：

- OpenAI Chat Completions API：已核对 function tools、custom tools、allowed_tools、文本 / 图片 / 音频 / 文件输入。
- OpenAI Responses API：已核对 input item、output item、reasoning、usage details。
- Anthropic Messages API：已核对 Messages、system、vision、tools、usage。
- Gemini GenerateContent API：已核对 contents、systemInstruction、tools、toolConfig、thinkingConfig。

这份文档记录后续要建设的长期协议转换层。原则是：能同协议透传就透传，只有入口协议和出口协议不一致时才进入内部统一模型。

## 当前状态

整体完成度：约 94%。

主架构和核心跨协议能力已经完成。剩余工作主要是高级特性、边界兼容和测试覆盖，不影响普通文本、图片、基础工具调用和 usage 统计的主路径。

已完成：

- 四个协议基础请求 / 响应转换：
  - OpenAI Chat
  - OpenAI Responses
  - Anthropic Messages
  - Gemini GenerateContent
- 跨协议非流式路由：
  - OpenAI Chat -> Gemini
  - OpenAI Responses -> OpenAI Chat / Gemini
  - Anthropic Messages -> OpenAI Chat / Gemini
  - Gemini GenerateContent -> OpenAI Chat / OpenAI Responses / Anthropic Messages
- 文本流式转换：
  - OpenAI Chat / OpenAI Responses / Anthropic Messages / Gemini GenerateContent 之间的文本 delta 已能互转。
  - 最终 usage 已回收并记录。
- 同协议透传：
  - 同协议仍直接透传，不强行进入转换层。

剩余事项：

- 流式工具调用增强：
  - OpenAI `tool_calls` delta
  - OpenAI Responses `function_call_arguments` delta
  - Anthropic `input_json_delta`
  - Gemini `functionCall` stream
- 高级协议能力增强：
  - OpenAI custom tool
  - OpenAI Responses hosted tools / MCP / code interpreter
  - file input
  - reasoning / thinking 内容流式回传
- 测试补强：
  - 更完整的协议矩阵测试
  - 流式工具调用测试
  - 错误响应转换测试
  - usage 统计边界测试

## 目标

- 建立内部统一请求、响应和流式事件模型。
- 保留同协议透传能力，不让转换层影响已有直通链路。
- 把跨协议转换从业务 handler 中抽出来，避免 OpenAI、Gemini、Anthropic、Responses 之间两两手写转换。
- 为后续新增 Anthropic / Gemini / Responses 的跨协议转换打基础。

## 核心原则

1. 入口协议等于出口协议时，直接透传。
2. 入口协议不等于出口协议时，走 Canonical 转换。
3. 转换层只处理协议语义，不处理鉴权、选路、key 轮询、冷却和统计。
4. Provider 特有能力允许通过扩展字段表达，但不能污染主模型。
5. 不追求一次性覆盖所有 provider，先覆盖当前真实路径。
6. 跟不上 Canonical 主模型的新版协议能力必须显式报错，不能静默忽略。

## 请求路径

```text
Client
  |
  v
Inbound Protocol Handler
  |
  | same protocol
  +--> Raw Provider Proxy
  |
  | cross protocol
  v
Inbound Converter
  |
  v
Canonical Request
  |
  v
Outbound Converter
  |
  v
Provider
  |
  v
Response Converter
  |
  v
Client Protocol Response
```

## 第一阶段范围

第一阶段只迁移已有能力，不强行扩新协议面。直通链路继续透传，跨协议需要时才走 Canonical。

- OpenAI Chat -> OpenAI：继续透传
- OpenAI Responses -> OpenAI Responses：继续透传
- Anthropic Messages -> Anthropic：继续透传
- Gemini native -> Gemini：继续透传
- OpenAI Chat -> Gemini：迁入 Canonical 转换层
- OpenAI Responses -> OpenAI Chat / Gemini：迁入 Canonical 转换层
- Anthropic Messages -> OpenAI Chat / Gemini：迁入 Canonical 转换层
- Gemini GenerateContent -> OpenAI Chat / OpenAI Responses / Anthropic Messages：迁入 Canonical 转换层
- OpenAI Chat / Responses / Anthropic / Gemini 的基础请求、响应转换器：先在 `internal/protocol` 中准备好。

## 第一阶段不做

- 不把所有透传链路强行改造成 Canonical。
- 不支持所有 OpenAI / Gemini / Anthropic 参数。
- 不做 LiteLLM 级别 provider 兼容矩阵。
- 不改变现有路由、key 轮询、冷却和 usage 统计主流程。

## 当前显式不支持

- OpenAI Chat custom tool：Canonical 当前只有 function tool。
- OpenAI Chat / Responses file input：Canonical 当前只有 text / image / audio。
- OpenAI Responses hosted tools / MCP / shell / apply_patch 等工具项：Canonical 当前只有 function call。
- Gemini `thinkingBudget` / `includeThoughts`：当前按产品策略只映射 `thinkingLevel`，不默认回流思考内容。
- Anthropic URL / file image source：当前只映射 base64 image。
- OpenAI Responses / Anthropic / Gemini 跨协议 stream：当前覆盖文本 delta 和最终 usage。
- 工具调用的跨协议 stream：后续增强，当前非流式工具调用已覆盖。

## 建议目录

```text
internal/protocol/
  canonical.go
  chat.go
  passthrough.go
  openai_chat.go
  openai_chat_response.go
  openai_responses.go
  gemini.go
  gemini_response.go
  anthropic.go
```

## Canonical 类型

### CanonicalRequest

- `Model`
- `System`
- `Messages`
- `Tools`
- `ToolChoice`
- `Temperature`
- `TopP`
- `MaxOutputTokens`
- `Stream`
- `ReasoningEffort`
- `Metadata`
- `Raw`

### CanonicalMessage

- `Role`
- `Parts`
- `Name`
- `ToolCallID`

### CanonicalPart

- `Type`
- `Text`
- `Image`
- `ToolCall`
- `ToolResult`

### CanonicalResponse

- `ID`
- `Model`
- `Message`
- `FinishReason`
- `Usage`
- `Raw`

### CanonicalStreamEvent

- `Type`
- `TextDelta`
- `ToolCallDelta`
- `Usage`
- `Error`
- `Done`
- `Raw`

### CanonicalUsage

- `PromptTokens`
- `CompletionTokens`
- `TotalTokens`
- `CachedTokens`
- `ReasoningTokens`

## 接口

```go
type InboundConverter interface {
	ToCanonical(ctx context.Context, body []byte) (CanonicalRequest, error)
}

type OutboundConverter interface {
	FromCanonical(ctx context.Context, req CanonicalRequest) (ProviderRequest, error)
}

type ResponseConverter interface {
	ToClient(ctx context.Context, resp *http.Response) (*http.Response, error)
}
```

## 透传判断

```go
func ShouldPassthrough(inbound, outbound string) bool {
	return inbound == outbound
}
```

后续如果出现协议兼容别名，可以只在这里集中处理。

## 实施步骤

1. 新增 `internal/protocol` 包，只定义类型和 `ShouldPassthrough`。已完成。
2. 抽出 OpenAI Chat -> Canonical。已完成，主代码直接调用 `protocol.OpenAIChatToCanonical`。
3. 抽出 Canonical -> Gemini request。已完成，主代码直接调用 `protocol.CanonicalToGeminiGenerateRequest`。
4. 抽出 Gemini response / stream -> OpenAI Chat response。已完成。
5. 抽出 Canonical -> OpenAI Chat request。已完成。
6. 抽出 Gemini request -> Canonical。已完成。
7. 抽出 OpenAI Chat response / stream -> Canonical。已完成。
8. 抽出 Canonical -> Gemini response / stream。已完成。
9. 让现有 OpenAI Chat -> Gemini 路径改走新包。已完成。
10. 保持 OpenAI Chat -> OpenAI 仍走原始透传。已完成。
11. 抽出 Anthropic Messages 请求 / 响应双向转换。已完成。
12. 抽出 OpenAI Responses 请求 / 响应双向转换。已完成。
13. 接入 OpenAI Responses -> OpenAI Chat / Gemini 跨协议路由。已完成。
14. 接入 Anthropic Messages -> OpenAI Chat / Gemini 跨协议路由。已完成。
15. 接入 Gemini GenerateContent -> OpenAI Chat / OpenAI Responses / Anthropic Messages 跨协议路由。已完成。
16. 接入 OpenAI Responses / Anthropic 跨协议流式文本转换。已完成。
17. 补测试，确认透传路径不会进入转换层。

## 测试清单

- OpenAI Chat -> OpenAI 透传。
- OpenAI Chat -> Gemini 非流式转换。
- OpenAI Chat -> Gemini 流式转换。
- OpenAI tools -> Gemini function declarations。
- OpenAI allowed_tools -> Canonical tool choice。
- OpenAI tool result -> Gemini function response。
- Gemini function call -> OpenAI tool calls。
- usage details 包含 cached / reasoning token。
- Anthropic Messages 请求 / 响应双向转换。
- OpenAI Responses 请求 / 响应双向转换。
- OpenAI Responses -> OpenAI Chat 非流式转换。
- OpenAI Responses -> Gemini 非流式转换。
- Anthropic Messages -> OpenAI Chat 非流式转换。
- Anthropic Messages -> Gemini 非流式转换。
- Gemini GenerateContent -> OpenAI Chat / Responses / Anthropic 非流式转换。
- OpenAI Responses -> Gemini 流式文本转换。
- Anthropic Messages -> OpenAI Chat 流式文本转换。
- unsupported 参数策略明确：忽略或报错。

## 验收标准

- `go test ./internal/...` 通过。
- 已有同协议透传行为不变。
- OpenAI Chat -> Gemini 行为不回退。
- 新增 Anthropic 或 Responses 转换时，只需要新增 converter，不需要改核心路由逻辑。
