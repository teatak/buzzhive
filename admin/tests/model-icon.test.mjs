import assert from "node:assert/strict";
import test from "node:test";
import { modelIconName } from "../src/lib/model-icon.ts";

const custom = { name: "work-assistant", display_name: "工作助手", icon: "" };

test("a custom model can select a preset brand independently of its name", () => {
  assert.equal(modelIconName(custom), "");
  assert.equal(modelIconName({ ...custom, icon: "deepseek" }), "deepseek");
  assert.equal(modelIconName({ ...custom, icon: "deepseek", name: "renamed" }), "deepseek");
});

test("explicit selection overrides the preset, clearing it restores automatic matching", () => {
  const presets = [{ name: custom.name, family: "OpenAI" }];
  assert.equal(modelIconName({ ...custom, icon: "claude" }, presets), "claude");
  assert.equal(modelIconName(custom, presets), "openai");
});

test("an exact preset matches even if the model ID contains no brand keyword", () => {
  assert.equal(modelIconName({ ...custom, name: "  HELPER-2 " }, [{ name: "helper-2", family: "Anthropic" }]), "anthropic");
});

test("the model ID takes precedence over an unrelated display name", () => {
  assert.equal(modelIconName({ ...custom, name: "deepseek-flash", display_name: "Gemini replacement" }), "deepseek");
});

test("new model versions and provider-qualified IDs reuse brand icons", () => {
  for (const [name, expected] of [["deepseek-next", "deepseek"], ["vendor/claude-next", "anthropic"], ["kimi-k3", "moonshot"], ["glm-5.3", "zhipu"], ["o3", "openai"], ["grok-code", "grok"]]) {
    assert.equal(modelIconName({ ...custom, name }), expected);
  }
  assert.equal(modelIconName({ ...custom, display_name: "DeepSeek 自定义模型" }), "deepseek");
});

// Use the same family and provider-prefix cases as Pudding's model-brand tests.
test("automatic recognition follows Pudding's model family rules", () => {
  for (const [name, expected] of [
    ["deepseek-chat", "deepseek"],
    ["deepseek-reasoner", "deepseek"],
    ["qwen3-max", "qwen"],
    ["z-ai/glm-5", "zhipu"],
    ["anthropic/claude-sonnet-4-6", "anthropic"],
    ["openai/gpt-5.4", "openai"],
    ["vendor/o3-mini", "openai"],
    ["o5-next", "openai"],
    ["gemma-3-27b", "gemini"],
    ["moonshot-v1-128k", "moonshot"],
    ["mimo-v2.5-terra", "mimo"],
    ["grok-4", "grok"],
    ["  QWEN3-NEXT  ", "qwen"],
  ]) {
    assert.equal(modelIconName({ ...custom, name }), expected, name);
  }
});

test("unknown names and embedded substrings do not invent a model brand", () => {
  for (const name of ["", "some-proxy/mystery-model", "nvidia/nemotron-4-340b", "mydeepseek", "notgpt", "openrouter/custom-model"]) {
    assert.equal(modelIconName({ ...custom, name }), "", name);
  }
});
