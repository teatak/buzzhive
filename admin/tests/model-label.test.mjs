import assert from "node:assert/strict";
import test from "node:test";
import { displayNameFromModelID, modelDisplayName } from "../src/lib/model.ts";
import { modelIconOptions } from "../src/lib/model-icon.ts";
import { dict as en } from "../src/i18n/en.ts";
import { dict as zh } from "../src/i18n/zh-cn.ts";
import { dict as zhTW } from "../src/i18n/zh-tw.ts";

test("generated names and brand options retain official DeepSeek / MiMo casing", () => {
  for (const [id, expected] of [
    ["deepseek-flash", "DeepSeek Flash"],
    ["deepseek-v4-pro", "DeepSeek V4 Pro"],
    ["xiaomi/mimo-v2.6-flash", "MiMo V2.6 Flash"],
    ["models/mimo-v2.6-pro", "MiMo V2.6 Pro"],
  ]) {
    assert.equal(displayNameFromModelID(id), expected);
    assert.equal(modelDisplayName({ name: id, display_name: "" }), expected);
  }
  for (const [brand, expected] of [["deepseek", "DeepSeek"], ["mimo", "MiMo"]]) {
    assert.equal(modelIconOptions.find(item => item.value === brand).label, expected);
    for (const dict of [en, zh, zhTW]) assert.ok(dict["providers.preset_" + brand].includes(expected));
  }
});

test("custom names are preserved and a blank name remains unset", () => {
  for (const custom of ["Mimo custom", "Deepseek custom", "我的助手"]) {
    assert.equal(modelDisplayName({ name: "mimo-v2.6-flash", display_name: custom }), custom);
  }
  const model = { name: "mimo-v2.6-pro", display_name: "  " };
  assert.equal(modelDisplayName(model), "MiMo V2.6 Pro");
  assert.equal(model.display_name, "  ");
});
