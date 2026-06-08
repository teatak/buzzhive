import type { Model } from "../types/admin";

export function modelDisplayName(model: Pick<Model, "name" | "display_name">) {
  const explicit = model.display_name.trim();
  return explicit || displayNameFromModelID(model.name);
}

export function displayNameFromModelID(id: string) {
  const modelID = id.trim().replace(/^models\//i, "").split("/").pop() ?? id.trim();
  if (!modelID) return id;
  return modelID
    .replace(/[_:-]+/g, " ")
    .split(/\s+/)
    .filter(Boolean)
    .map(formatModelNamePart)
    .join(" ");
}

function formatModelNamePart(part: string) {
  const lower = part.toLowerCase();
  const acronyms: Record<string, string> = {
    api: "API",
    gpt: "GPT",
    glm: "GLM",
    json: "JSON",
    llm: "LLM",
    r1: "R1",
    vl: "VL",
  };
  if (acronyms[lower]) return acronyms[lower];
  if (/^\d+(?:\.\d+)*$/.test(part)) return part;
  return lower.charAt(0).toUpperCase() + lower.slice(1);
}
