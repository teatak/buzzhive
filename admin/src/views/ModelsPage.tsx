import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type MouseEvent } from "react";
import {
  ArrowLeft,
  Braces,
  Brain,
  Check,
  Copy,
  Eye,
  Info,
  Mic,
  Pencil,
  Plus,
  Radio,
  Settings2,
  Trash2,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import { toast } from "sonner";
import { request } from "../api/client";
import { BrandIcon } from "../components/brand-icons";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Checkbox } from "../components/ui/checkbox";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "../components/ui/dialog";
import { Input } from "../components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "../components/ui/tooltip";
import { EnabledToggleButton } from "../components/enabled-toggle-button";
import { FormNumberField, FormSelectField, FormStaticField, FormTextareaField, FormTextField } from "../components/form-fields";
import { useLocale } from "../i18n/locale";
import { modelDisplayName } from "../lib/model";
import type { Model, ModelPreset, ModelRoute, ProviderRecord } from "../types/admin";

type ModelsPageProps = {
  token: string;
  providers: ProviderRecord[];
  models: Model[];
  modelPresets: ModelPreset[];
  modelRoutes: ModelRoute[];
  onReload: () => Promise<void>;
  onRequestDeleteItem: (onConfirm: () => Promise<void>) => void;
};

const defaultCapabilities = JSON.stringify({
  stream: true,
  tools: false,
  vision: false,
  json_schema: false,
  reasoning: false,
  audio_input: false,
}, null, 2);

const modelDefaults: Model = {
  id: 0,
  name: "",
  display_name: "",
  description: "",
  context_window: 0,
  max_input_tokens: 0,
  max_output_tokens: 0,
  capabilities: defaultCapabilities,
  selection_policy: "round_robin",
  enabled: true,
};

const capabilityOptions = [
  "stream",
  "tools",
  "vision",
  "json_schema",
  "reasoning",
  "audio_input",
] as const;

const routeDefaults = {
  id: 0,
  model_id: 0,
  provider_id: 0,
  upstream_model: "",
  enabled: true,
  priority: 0,
  weight: 1,
};

export function ModelsPage(props: ModelsPageProps) {
  const { t } = useLocale();
  const [modelOpen, setModelOpen] = useState(false);
  const [presetOpen, setPresetOpen] = useState(false);
  const [routeOpen, setRouteOpen] = useState(false);
  const [modelForm, setModelForm] = useState<Model>(modelDefaults);
  const [presetIDs, setPresetIDs] = useState<string[]>([]);
  const [anchorIndex, setAnchorIndex] = useState<number | null>(null);
  const [lastShiftRange, setLastShiftRange] = useState<string[]>([]);
  const [routeForm, setRouteForm] = useState(routeDefaults);
  const [saving, setSaving] = useState(false);
  const [selectedModelID, setSelectedModelID] = useState<number | null>(null);
  const [copiedTarget, setCopiedTarget] = useState("");
  const copiedTimer = useRef<number | null>(null);

  const providerName = useMemo(() => new Map(props.providers.map((provider) => [provider.id, provider.name])), [props.providers]);
  const existingModelNames = useMemo(() => new Set(props.models.map((model) => model.name)), [props.models]);
  const routesByModel = useMemo(() => {
    const routes = new Map<number, ModelRoute[]>();
    for (const route of props.modelRoutes) {
      routes.set(route.model_id, [...(routes.get(route.model_id) ?? []), route]);
    }
    return routes;
  }, [props.modelRoutes]);
  const selectedPresets = props.modelPresets.filter((preset) => presetIDs.includes(preset.id));
  const selectedPresetCount = selectedPresets.length;
  const selectedRouteModel = props.models.find((model) => model.id === routeForm.model_id);
  const selectedModel = selectedModelID == null ? null : props.models.find((model) => model.id === selectedModelID) ?? null;
  const selectedModelRoutes = selectedModel ? routesByModel.get(selectedModel.id) ?? [] : [];

  useEffect(() => () => {
    if (copiedTimer.current) window.clearTimeout(copiedTimer.current);
  }, []);

  async function save(path: string, body: unknown, method = "POST") {
    setSaving(true);
    try {
      await request(path, props.token, { method, body: JSON.stringify(body) });
      await props.onReload();
      toast.success(t("common.save"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("toast.action_failed"));
    } finally {
      setSaving(false);
    }
  }

  async function remove(path: string) {
    setSaving(true);
    try {
      await request(path, props.token, { method: "DELETE" });
      await props.onReload();
    } finally {
      setSaving(false);
    }
  }

  async function copyModelName(name: string, target: string) {
    try {
      if (window.navigator.clipboard?.writeText && window.isSecureContext) {
        await window.navigator.clipboard.writeText(name);
      } else {
        fallbackCopyText(name);
      }
      setCopiedTarget(target);
      if (copiedTimer.current) window.clearTimeout(copiedTimer.current);
      copiedTimer.current = window.setTimeout(() => setCopiedTarget(""), 1200);
    } catch {
      toast.error(t("common.copy_failed"));
    }
  }

  function openModel(model?: Model) {
    setModelForm(model ? { ...model, capabilities: model.capabilities || defaultCapabilities } : modelDefaults);
    setModelOpen(true);
  }

  function openRoute(modelID: number, route?: ModelRoute) {
    const providerID = props.providers[0]?.id ?? 0;
    const model = props.models.find((item) => item.id === modelID);
    const upstreamModel = model?.name ?? "";
    setRouteForm(route ? { ...route } : {
      ...routeDefaults,
      model_id: modelID,
      provider_id: providerID,
      upstream_model: upstreamModel,
    });
    setRouteOpen(true);
  }

  async function submitModel() {
    await save("/admin/api/models", { ...modelForm, max_input_tokens: modelForm.context_window }, modelForm.id ? "PUT" : "POST");
    setModelOpen(false);
  }

  async function submitPreset() {
    if (presetIDs.length === 0) return;
    await save("/admin/api/model-presets", { ids: presetIDs });
    setPresetOpen(false);
    setPresetIDs([]);
    setAnchorIndex(null);
    setLastShiftRange([]);
  }

  function togglePreset(preset: ModelPreset, index: number, shiftKey: boolean) {
    if (existingModelNames.has(preset.name)) return;

    if (shiftKey && anchorIndex !== null) {
      const [start, end] = [anchorIndex, index].sort((a, b) => a - b);
      const newRangeIDs = props.modelPresets
        .slice(start, end + 1)
        .filter((item) => !existingModelNames.has(item.name))
        .map((item) => item.id);

      setPresetIDs((current) => {
        const next = current.filter((id) => !(lastShiftRange.includes(id) && !newRangeIDs.includes(id)));
        newRangeIDs.forEach((id) => {
          if (!next.includes(id)) {
            next.push(id);
          }
        });
        return next;
      });
      setLastShiftRange(newRangeIDs);
    } else {
      setPresetIDs((current) => {
        return current.includes(preset.id) ? current.filter((id) => id !== preset.id) : [...current, preset.id];
      });
      setAnchorIndex(index);
      setLastShiftRange([]);
    }
  }

  async function submitRoute() {
    await save("/admin/api/model-routes", routeForm, routeForm.id ? "PUT" : "POST");
    setRouteOpen(false);
  }

  return (
    <div className="stack">
      {selectedModel ? (
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="flex min-w-0 items-start gap-3">
                <Button type="button" variant="ghost" size="icon" onClick={() => setSelectedModelID(null)} aria-label={t("models.back_to_models")}>
                  <ArrowLeft />
                </Button>
                <ModelIcon model={selectedModel} className="mt-0.5 h-10 w-10" />
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <CardTitle>{modelDisplayName(selectedModel)}</CardTitle>
                    <ModelDescriptionInfo description={selectedModel.description} />
                    <StatusBadge enabled={selectedModel.enabled} />
                  </div>
                  <div className="mt-1 flex min-w-0 items-center gap-1.5">
                    <div className="truncate mono text-sm text-muted-foreground">{selectedModel.name}</div>
                    <CopyModelNameButton
                      copied={copiedTarget === `model-name-${selectedModel.id}`}
                      onCopy={() => copyModelName(selectedModel.name, `model-name-${selectedModel.id}`)}
                    />
                  </div>
                  <ModelCapabilityIcons capabilities={selectedModel.capabilities} className="mt-2" />
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Button type="button" variant="outline" onClick={() => openRoute(selectedModel.id)}><Plus />{t("models.new_route")}</Button>
                <RowActions
                  onEdit={() => openModel(selectedModel)}
                  onToggle={() => void save("/admin/api/models", { ...selectedModel, enabled: !selectedModel.enabled }, "PUT")}
                  onDelete={() => props.onRequestDeleteItem(() => remove(`/admin/api/models?id=${selectedModel.id}`))}
                  enabled={selectedModel.enabled}
                />
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <ModelStat label={t("models.context_window")} value={formatModelNumber(selectedModel.context_window)} />
              <ModelStat label={t("models.max_output_tokens")} value={formatModelNumber(selectedModel.max_output_tokens)} />
              <ModelStat label={t("models.policy")} value={policyLabel(t, selectedModel.selection_policy)} />
            </div>
            <div className="flex items-center gap-2">
              <h3 className="text-lg font-semibold">{t("models.routes")}</h3>
              <Badge variant="secondary">{selectedModelRoutes.length}</Badge>
            </div>
            {selectedModelRoutes.length === 0 ? (
              <div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">{t("models.no_routes")}</div>
            ) : (
              <div className="data-table-card">
                <Table className="keys-table-inner">
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("models.provider")}</TableHead>
                      <TableHead>{t("models.upstream_model")}</TableHead>
                      <TableHead className="right">{t("models.priority")}</TableHead>
                      <TableHead className="right">{t("models.weight")}</TableHead>
                      <TableHead>{t("common.status")}</TableHead>
                      <TableHead className="right">{t("common.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {selectedModelRoutes.map((route) => (
                      <TableRow key={route.id}>
                        <TableCell className="mono">{route.provider_name ?? providerName.get(route.provider_id) ?? route.provider_id}</TableCell>
                        <TableCell className="mono">{route.upstream_model}</TableCell>
                        <TableCell className="right mono">{route.priority}</TableCell>
                        <TableCell className="right mono">{route.weight}</TableCell>
                        <TableCell><StatusBadge enabled={route.enabled} /></TableCell>
                        <TableCell className="right">
                          <RowActions
                            onEdit={() => openRoute(selectedModel.id, route)}
                            onToggle={() => void save("/admin/api/model-routes", { ...route, enabled: !route.enabled }, "PUT")}
                            onDelete={() => props.onRequestDeleteItem(() => remove(`/admin/api/model-routes?id=${route.id}`))}
                            enabled={route.enabled}
                          />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      ) : (
        <div className="stack">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <h2 className="text-2xl font-semibold tracking-tight">{t("models.public_models")}</h2>
              <Badge variant="secondary">{props.models.length}</Badge>
            </div>
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={() => {
                setPresetIDs([]);
                setAnchorIndex(null);
                setLastShiftRange([]);
                setPresetOpen(true);
              }}>
                <Plus />{t("models.add_from_preset")}
              </Button>
              <Button type="button" onClick={() => openModel()}><Plus />{t("models.new_model")}</Button>
            </div>
          </div>

          <div className="grid items-start gap-3 xl:grid-cols-2 2xl:grid-cols-3">
            {props.models.map((model) => {
              const routes = routesByModel.get(model.id) ?? [];
              return (
                <Card
                  key={model.id}
                  role="button"
                  tabIndex={0}
                  onClick={() => setSelectedModelID(model.id)}
                  onKeyDown={(event) => activateCard(event, () => setSelectedModelID(model.id))}
                  className="self-start cursor-pointer transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  <CardContent className="px-4">
                    <div className="grid gap-3">
                      <div className="flex items-start justify-between gap-3">
                        <div className="flex min-w-0 items-start gap-3">
                          <ModelIcon model={model} />
                          <div className="min-w-0">
                            <div className="flex min-w-0 items-center gap-2">
                              <div className="truncate text-base font-semibold">{modelDisplayName(model)}</div>
                              <ModelDescriptionInfo description={model.description} />
                            </div>
                            <div className="mt-0.5 flex min-w-0 items-center gap-2">
                              <div className="truncate mono text-xs text-muted-foreground">{model.name}</div>
                              <CopyModelNameButton
                                copied={copiedTarget === `model-name-${model.id}`}
                                onCopy={() => copyModelName(model.name, `model-name-${model.id}`)}
                              />
                              <ModelValueChip value={formatModelNumber(model.context_window)} />
                            </div>
                            <ModelCapabilityIcons capabilities={model.capabilities} className="mt-2" />
                          </div>
                        </div>
                        <StatusBadge enabled={model.enabled} />
                      </div>
                      <div className="flex items-center justify-between gap-3 border-t pt-3">
                        <ModelRouteChip label={t("models.routes")} count={routes.length} />
                        <div className="flex justify-end" onClick={stopCardAction}>
                          <RowActions
                            onEdit={() => openModel(model)}
                            onToggle={() => void save("/admin/api/models", { ...model, enabled: !model.enabled }, "PUT")}
                            onDelete={() => props.onRequestDeleteItem(() => remove(`/admin/api/models?id=${model.id}`))}
                            enabled={model.enabled}
                          />
                        </div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </div>
      )}

      <Dialog open={modelOpen} onOpenChange={setModelOpen}>
        <DialogContent className="max-h-[min(880px,calc(100dvh-2rem))] gap-3 overflow-y-auto sm:max-w-2xl">
          <DialogHeader><DialogTitle>{modelForm.id ? t("models.edit_model") : t("models.new_model")}</DialogTitle></DialogHeader>
          <div className="grid gap-4 py-3">
            <div className="grid gap-4 sm:grid-cols-2">
              <FormTextField label={t("model.model")} value={modelForm.name} onChange={(name) => setModelForm({ ...modelForm, name })} />
              <FormTextField label={t("models.display_name")} value={modelForm.display_name} onChange={(display_name) => setModelForm({ ...modelForm, display_name })} />
            </div>
            <FormTextareaField label={t("models.description")} className="min-h-16" value={modelForm.description} onChange={(description) => setModelForm({ ...modelForm, description })} />
            <div className="grid gap-4 sm:grid-cols-2">
              <TokenNumberField label={t("models.context_window")} value={modelForm.context_window} onChange={(context_window) => setModelForm({ ...modelForm, context_window })} />
              <TokenNumberField label={t("models.max_output_tokens")} value={modelForm.max_output_tokens} onChange={(max_output_tokens) => setModelForm({ ...modelForm, max_output_tokens })} />
            </div>
            <CapabilityField value={modelForm.capabilities} onChange={(capabilities) => setModelForm({ ...modelForm, capabilities })} />
            <div className="grid gap-4 sm:grid-cols-2">
              <FormSelectField
                label={t("models.policy")}
                tip={t("models.tip_policy")}
                value={modelForm.selection_policy}
                options={["round_robin", "weighted"].map((value) => ({ value, label: policyLabel(t, value) }))}
                onChange={(selection_policy) => setModelForm({ ...modelForm, selection_policy })}
              />
              <EnabledSelect value={modelForm.enabled} onChange={(enabled) => setModelForm({ ...modelForm, enabled })} />
            </div>
          </div>
          <DialogFooter>
            <Button disabled={saving} onClick={() => void submitModel()}>{t("common.save")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={presetOpen} onOpenChange={(open) => {
        setPresetOpen(open);
        if (!open) {
          setPresetIDs([]);
          setAnchorIndex(null);
          setLastShiftRange([]);
        }
      }}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader><DialogTitle>{t("models.add_from_preset")}</DialogTitle></DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid max-h-[min(460px,45dvh)] gap-2 overflow-y-auto pr-1 sm:grid-cols-2 lg:grid-cols-3">
              {props.modelPresets.map((preset, index) => {
                const exists = existingModelNames.has(preset.name);
                const selected = presetIDs.includes(preset.id);
                return (
                  <button
                    key={preset.id}
                    type="button"
                    disabled={exists}
                    onClick={(event) => togglePreset(preset, index, event.shiftKey)}
                    className={`flex min-w-0 items-center gap-3 rounded-xl border px-4 py-3 text-left transition-colors enabled:hover:bg-muted/50 disabled:cursor-not-allowed disabled:opacity-50 ${selected ? "border-primary bg-primary/5" : "border-border bg-background"}`}
                  >
                    <ModelPresetIcon preset={preset} />
                    <span className="grid min-w-0 flex-1 gap-0.5">
                      <span className="truncate font-medium">{preset.display_name}</span>
                      <span className="truncate mono text-muted-foreground">{preset.family} / {preset.name}</span>
                    </span>
                    {exists && <Badge variant="outline">{t("models.already_exists")}</Badge>}
                  </button>
                );
              })}
            </div>
            <div className="h-28">
              {selectedPresetCount === 1 && selectedPresets[0] && (
                <div className="flex h-full min-w-0 flex-col justify-between overflow-hidden rounded-md border p-3 text-sm">
                  <div className="flex min-w-0 items-center gap-3">
                    <ModelPresetIcon preset={selectedPresets[0]} />
                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 items-center gap-2">
                        <strong className="truncate">{selectedPresets[0].display_name}</strong>
                        <Badge variant="secondary">{selectedPresets[0].family}</Badge>
                        <span className="truncate mono text-xs text-muted-foreground">{selectedPresets[0].name}</span>
                      </div>
                      <div className="mt-1 truncate text-muted-foreground">{selectedPresets[0].description}</div>
                    </div>
                  </div>
                  <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2">
                    <div><span>{t("models.context_window")}: </span><span className="mono">{formatModelNumber(selectedPresets[0].context_window)}</span></div>
                    <div><span>{t("models.max_output_tokens")}: </span><span className="mono">{formatModelNumber(selectedPresets[0].max_output_tokens)}</span></div>
                  </div>
                </div>
              )}
              {selectedPresetCount > 1 && (
                <div className="flex h-full items-center rounded-md border bg-muted/40 p-4 text-sm font-medium">
                  {t("models.selected_models", { count: selectedPresetCount })}
                </div>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button disabled={saving || selectedPresetCount === 0} onClick={() => void submitPreset()}>
              {selectedPresetCount > 0 ? `${t("models.add_model")} ${selectedPresetCount}` : t("models.add_model")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={routeOpen} onOpenChange={setRouteOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>{routeForm.id ? t("models.edit_route") : t("models.new_route")}</DialogTitle></DialogHeader>
          <div className="grid gap-4 py-4">
            <FormStaticField label={t("model.model")}>
              <div className="rounded-md border bg-muted px-3 py-2 mono text-sm">{selectedRouteModel?.name ?? routeForm.model_id}</div>
            </FormStaticField>
            <FormSelectField
              label={t("models.provider")}
              tip={t("models.tip_provider")}
              value={String(routeForm.provider_id)}
              options={props.providers.map((provider) => ({ value: String(provider.id), label: provider.name }))}
              onChange={(value) => {
                const provider_id = Number(value);
                setRouteForm({ ...routeForm, provider_id });
              }}
            />
            <FormTextField
              label={t("models.upstream_model")}
              tip={t("models.tip_upstream_model")}
              value={routeForm.upstream_model}
              onChange={(upstream_model) => setRouteForm({
                ...routeForm,
                upstream_model,
              })}
            />
            <FormNumberField label={t("models.priority")} tip={t("models.tip_priority")} value={routeForm.priority} onChange={(priority) => setRouteForm({ ...routeForm, priority })} />
            <FormNumberField label={t("models.weight")} tip={t("models.tip_weight")} value={routeForm.weight} onChange={(weight) => setRouteForm({ ...routeForm, weight })} />
            <EnabledSelect value={routeForm.enabled} onChange={(enabled) => setRouteForm({ ...routeForm, enabled })} />
          </div>
          <DialogFooter><Button disabled={saving || !routeForm.model_id || !routeForm.provider_id || !routeForm.upstream_model} onClick={() => void submitRoute()}>{t("common.save")}</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function stopCardAction(event: MouseEvent<HTMLDivElement>) {
  event.stopPropagation();
}

function fallbackCopyText(value: string) {
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.left = "0";
  textarea.style.top = "0";
  textarea.style.opacity = "0";
  textarea.style.pointerEvents = "none";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  textarea.setSelectionRange(0, value.length);
  const ok = document.execCommand("copy");
  document.body.removeChild(textarea);
  if (!ok) throw new Error("copy failed");
}

function activateCard(event: KeyboardEvent<HTMLElement>, onActivate: () => void) {
  if (event.key !== "Enter" && event.key !== " ") return;
  event.preventDefault();
  onActivate();
}

function ModelIcon({ model, className = "h-10 w-10" }: { model: Model; className?: string }) {
  const family = modelFamily(model);
  const brand = modelBrandForFamily(family);
  if (brand) return <BrandIcon className={`${className} rounded-[10px]`} name={brand} />;

  return (
    <div className={`${className} flex shrink-0 items-center justify-center rounded-[10px] border border-dashed text-muted-foreground`}>
      <Settings2 className="h-1/2 w-1/2" strokeWidth={2} />
    </div>
  );
}

function modelFamily(model: Model) {
  const text = `${model.name} ${model.display_name}`.toLowerCase();
  if (text.includes("gemini") || text.includes("gemma")) return "gemini";
  if (text.includes("claude") || text.includes("anthropic")) return "anthropic";
  if (text.includes("deepseek")) return "deepseek";
  if (text.includes("qwen")) return "qwen";
  if (text.includes("kimi") || text.includes("moonshot")) return "moonshot";
  if (text.includes("glm") || text.includes("zhipu")) return "zhipu";
  if (text.includes("openrouter")) return "openrouter";
  if (text.includes("gpt") || text.includes("openai")) return "openai";
  if (text.includes("mimo")) return "mimo";
  return "generic";
}

function policyLabel(t: (key: string) => string, policy: string) {
  if (policy === "weighted") return t("models.policy_weighted");
  if (policy === "round_robin") return t("models.policy_round_robin");
  return policy;
}

function modelBrandForFamily(family: string) {
  const normalized = family.toLowerCase();
  if (normalized === "anthropic") return "claude";
  if (normalized === "mimo") return "mimo";
  if (normalized === "moonshot") return "moonshot";
  if (normalized === "zhipu") return "zhipu";
  if (["gemini", "openai", "deepseek", "qwen", "openrouter"].includes(normalized)) return normalized;
  return "";
}

function ModelPresetIcon({ preset }: { preset: ModelPreset }) {
  const brand = modelBrandForFamily(preset.family);
  if (brand) return <BrandIcon className="h-8 w-8" name={brand} />;

  return (
    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px] border border-dashed text-muted-foreground">
      <Settings2 className="h-5 w-5" strokeWidth={2} />
    </span>
  );
}

function ModelStat({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="min-w-0 rounded-lg bg-muted/50 px-3 py-2">
      <div className="truncate text-xs text-muted-foreground">{label}</div>
      <div className={`mt-1 truncate text-sm font-medium ${mono ? "mono" : ""}`}>{value || "-"}</div>
    </div>
  );
}

function ModelChip({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <Badge variant="secondary" className="h-6 shrink-0 gap-1 rounded-full px-2 text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span className={mono ? "mono" : ""}>{value || "-"}</span>
    </Badge>
  );
}

function ModelRouteChip({ label, count }: { label: string; count: number }) {
  if (count === 0) {
    return (
      <Badge variant="outline" className="h-6 shrink-0 gap-1 rounded-full border-dashed px-2 text-xs text-muted-foreground">
        <span>{label}</span>
        <span>0</span>
      </Badge>
    );
  }
  return <ModelChip label={label} value={String(count)} />;
}

function ModelValueChip({ value }: { value: string }) {
  return <Badge variant="secondary" className="h-5 shrink-0 rounded-full px-2 mono text-[11px]">{value || "-"}</Badge>;
}

function CopyModelNameButton({ copied, onCopy }: { copied: boolean; onCopy: () => void }) {
  const { t } = useLocale();
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="h-6 w-6 shrink-0 text-muted-foreground hover:text-foreground"
          onClick={(event) => {
            event.stopPropagation();
            onCopy();
          }}
          aria-label={t("common.copy")}
        >
          {copied
            ? <Check className="text-emerald-600 dark:text-emerald-400" size={15} />
            : <Copy size={15} />}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{copied ? t("common.copied") : t("common.copy")}</TooltipContent>
    </Tooltip>
  );
}

const capabilityIconMap: Record<(typeof capabilityOptions)[number], LucideIcon> = {
  stream: Radio,
  tools: Wrench,
  vision: Eye,
  json_schema: Braces,
  reasoning: Brain,
  audio_input: Mic,
};

function ModelDescriptionInfo({ description }: { description?: string }) {
  const { t } = useLocale();
  const text = description?.trim();
  if (!text) return null;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground"
          aria-label={t("models.description")}
        >
          <Info className="h-3.5 w-3.5" strokeWidth={2.2} />
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-80 whitespace-normal text-left leading-relaxed">{text}</TooltipContent>
    </Tooltip>
  );
}

function ModelCapabilityIcons({ capabilities, className = "" }: { capabilities: string; className?: string }) {
  const { t } = useLocale();
  const parsed = parseCapabilities(capabilities);
  const enabledCapabilities = capabilityOptions.filter((name) => parsed[name]);
  if (enabledCapabilities.length === 0) return null;

  return (
    <div className={`flex flex-wrap items-center gap-1 ${className}`}>
      {enabledCapabilities.map((name) => {
        const Icon = capabilityIconMap[name];
        const label = t(`models.capability_${name}`);
        return (
          <Tooltip key={name}>
            <TooltipTrigger asChild>
              <span
                className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-lg bg-muted/70 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                aria-label={label}
              >
                <Icon className="h-3.5 w-3.5" strokeWidth={2} />
              </span>
            </TooltipTrigger>
            <TooltipContent>{label}</TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}

function formatModelNumber(value: number) {
  if (!value) return "-";
  if (value >= 1_000_000) return `${Number((value / 1_000_000).toFixed(2))}M`;
  if (value >= 1_000) return `${Number((value / 1_000).toFixed(1))}K`;
  return String(value);
}

function RowActions(props: { onEdit: () => void; onToggle: () => void; onDelete: () => void; enabled: boolean }) {
  const { t } = useLocale();
  return (
    <div className="flex justify-end gap-1">
      <Button type="button" variant="ghost" size="icon-sm" onClick={props.onEdit} aria-label={t("common.edit")}><Pencil /></Button>
      <EnabledToggleButton enabled={props.enabled} size="icon-sm" onClick={props.onToggle} />
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        className="text-destructive hover:bg-destructive/10 hover:text-destructive"
        onClick={props.onDelete}
        aria-label={t("common.delete")}
      >
        <Trash2 />
      </Button>
    </div>
  );
}

function EnabledSelect(props: { value: boolean; onChange: (value: boolean) => void }) {
  const { t } = useLocale();
  return (
    <FormSelectField
      label={t("common.status")}
      value={props.value ? "1" : "0"}
      options={[
        { value: "1", label: t("common.active") },
        { value: "0", label: t("common.disabled") },
      ]}
      onChange={(value) => props.onChange(value === "1")}
    />
  );
}

function TokenNumberField(props: { label: string; value: number; onChange: (value: number) => void }) {
  const [draft, setDraft] = useState(formatTokenDraft(props.value));

  useEffect(() => {
    setDraft(formatTokenDraft(props.value));
  }, [props.value]);

  function commit() {
    const parsed = parseTokenNumber(draft);
    if (parsed == null) {
      setDraft(formatTokenDraft(props.value));
      return;
    }
    props.onChange(parsed);
    setDraft(formatTokenDraft(parsed));
  }

  return (
    <FormStaticField label={props.label}>
      <div className="flex min-w-0 items-center gap-2">
        <Input
          className="min-w-0 flex-1"
          inputMode="numeric"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          onBlur={commit}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.currentTarget.blur();
            }
          }}
        />
        <span className="shrink-0 text-sm tabular-nums text-muted-foreground">
          {formatTokenHint(parseTokenNumber(draft) ?? props.value)}
        </span>
      </div>
    </FormStaticField>
  );
}

function CapabilityField(props: { value: string; onChange: (value: string) => void }) {
  const { t } = useLocale();
  const capabilities = parseCapabilities(props.value);

  function setCapability(name: string, enabled: boolean) {
    props.onChange(JSON.stringify({ ...capabilities, [name]: enabled }, null, 2));
  }

  return (
    <FormStaticField label={t("models.capabilities")}>
      <div className="grid grid-cols-2 gap-2 rounded-md border bg-muted/20 p-2 sm:grid-cols-4">
        {capabilityOptions.map((name) => (
          <label key={name} className="flex items-center gap-3 rounded-md px-2 py-1.5 text-sm hover:bg-muted">
            <Checkbox checked={Boolean(capabilities[name])} onCheckedChange={(checked) => setCapability(name, checked === true)} />
            <span>{t(`models.capability_${name}`)}</span>
          </label>
        ))}
      </div>
    </FormStaticField>
  );
}

function parseCapabilities(value: string): Record<string, boolean> {
  try {
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    return Object.fromEntries(Object.entries(parsed).map(([key, enabled]) => [key, Boolean(enabled)]));
  } catch {
    return {};
  }
}

function parseTokenNumber(value: string) {
  const normalized = value.trim().replace(/,/g, "");
  if (!normalized) return 0;
  if (!/^\d+$/.test(normalized)) return null;
  const amount = Number(normalized);
  if (!Number.isSafeInteger(amount)) return null;
  return amount;
}

function formatTokenDraft(value: number) {
  return String(value);
}

function formatTokenHint(value: number) {
  if (!value) return "0";
  if (value >= 1_000_000) return `${trimNumber(value / 1_000_000, 2)}M`;
  if (value >= 1_000) return `${trimNumber(value / 1_000, 1)}K`;
  return String(value);
}

function trimNumber(value: number, digits: number) {
  return String(Number(value.toFixed(digits)));
}

function StatusBadge({ enabled }: { enabled: boolean }) {
  const { t } = useLocale();
  return enabled
    ? <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge>
    : <Badge variant="secondary">{t("common.disabled")}</Badge>;
}
