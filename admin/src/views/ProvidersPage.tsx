import { useMemo, useState, type KeyboardEvent, type MouseEvent } from "react";
import {
  ArrowLeft,
  Check,
  Copy,
  Eye,
  EyeOff,
  KeyRound,
  Pencil,
  Plus,
  Settings2,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import { request } from "../api/client";
import {
  ClaudeIcon,
  DeepSeekIcon,
  GeminiIcon,
  MimoIcon,
  MoonshotIcon,
  OpenAIIcon,
  OpenRouterIcon,
  QwenIcon,
  ZhipuIcon,
} from "../components/brand-icons";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "../components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "../components/ui/tooltip";
import { EnabledToggleButton } from "../components/enabled-toggle-button";
import { FormNumberField, FormSelectField, FormStaticField, FormTextareaField, FormTextField } from "../components/form-fields";
import { useLocale } from "../i18n/locale";
import type { ProviderKey, ProviderPreset, ProviderRecord } from "../types/admin";

type ProviderForm = ProviderRecord;

type ProviderKeyForm = {
  id: number;
  provider_id: number;
  name: string;
  secret: string;
  enabled: boolean;
  priority: number;
  weight: number;
  labels: string;
};

const providerTypes = ["gemini", "openai", "anthropic", "openai-compatible"];

const defaultBaseURL: Record<string, string> = {
  gemini: "https://generativelanguage.googleapis.com",
  openai: "https://api.openai.com/v1",
  anthropic: "https://api.anthropic.com",
  "openai-compatible": "",
  mimo: "",
};

const providerDefaults: ProviderForm = {
  id: 0,
  name: "",
  type: "openai-compatible",
  preset_id: "",
  base_url: "",
  enabled: true,
};

const keyDefaults: ProviderKeyForm = {
  id: 0,
  provider_id: 0,
  name: "",
  secret: "",
  enabled: true,
  priority: 0,
  weight: 1,
  labels: "",
};

export function ProvidersPage(props: {
  token: string;
  providers: ProviderRecord[];
  providerKeys: ProviderKey[];
  providerPresets: ProviderPreset[];
  onReload: () => Promise<void>;
  onRequestDeleteProvider: (provider: ProviderRecord, onConfirm: () => Promise<void>) => void;
  onRequestDeleteProviderKey: (key: ProviderKey, onConfirm: () => Promise<void>) => void;
}) {
  const { t } = useLocale();
  const [open, setOpen] = useState(false);
  const [presetOpen, setPresetOpen] = useState(false);
  const [keyOpen, setKeyOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editingProviderID, setEditingProviderID] = useState<number | null>(null);
  const [editingKeyID, setEditingKeyID] = useState<number | null>(null);
  const [revealedProviderKeys, setRevealedProviderKeys] = useState<Record<number, string>>({});
  const [copiedProviderKeyID, setCopiedProviderKeyID] = useState<number | null>(null);
  const [selectedProviderID, setSelectedProviderID] = useState<number | null>(null);
  const [form, setForm] = useState<ProviderForm>(providerDefaults);
  const [providerKeySecret, setProviderKeySecret] = useState("");
  const [presetKeySecret, setPresetKeySecret] = useState("");
  const [keyForm, setKeyForm] = useState<ProviderKeyForm>(keyDefaults);
  const [presetID, setPresetID] = useState("");
  const existingProviderNames = useMemo(() => new Set(props.providers.map((provider) => provider.name.toLowerCase())), [props.providers]);
  const existingProviderPresetIDs = useMemo(() => new Set(props.providers.map((provider) => provider.preset_id).filter(Boolean)), [props.providers]);
  const keysByProvider = useMemo(() => {
    const grouped = new Map<number, ProviderKey[]>();
    for (const key of props.providerKeys) {
      grouped.set(key.provider_id, [...(grouped.get(key.provider_id) ?? []), key]);
    }
    return grouped;
  }, [props.providerKeys]);
  const selectedPreset = props.providerPresets.find((preset) => preset.id === presetID);
  const selectedPresetExists = selectedPreset ? providerPresetExists(selectedPreset, existingProviderNames, existingProviderPresetIDs) : false;
  const selectedProvider = selectedProviderID == null ? null : props.providers.find((provider) => provider.id === selectedProviderID) ?? null;
  const selectedProviderKeys = selectedProvider ? keysByProvider.get(selectedProvider.id) ?? [] : [];
  const keyDialogProvider = props.providers.find((provider) => provider.id === keyForm.provider_id);

  function openProvider(provider?: ProviderRecord) {
    setEditingProviderID(provider?.id ?? null);
    setForm(provider ? { ...provider } : { ...providerDefaults });
    setProviderKeySecret("");
    setOpen(true);
  }

  function openKeyImport(providerID = props.providers[0]?.id ?? 0) {
    setEditingKeyID(null);
    setKeyForm({ ...keyDefaults, provider_id: providerID });
    setKeyOpen(true);
  }

  function openKeyEdit(key: ProviderKey) {
    setEditingKeyID(key.id);
    setKeyForm({
      id: key.id,
      provider_id: key.provider_id,
      name: key.name,
      secret: "",
      enabled: key.enabled,
      priority: key.priority,
      weight: key.weight,
      labels: key.labels ?? "",
    });
    setKeyOpen(true);
  }

  function changeType(type: string) {
    setForm({ ...form, type, preset_id: form.preset_id || type, base_url: form.base_url || defaultBaseURL[type] || "" });
  }

  async function saveProvider() {
    setSaving(true);
    try {
      const providerID = editingProviderID;
      const saved = await request<ProviderRecord>("/admin/api/providers", props.token, {
        method: providerID ? "PUT" : "POST",
        body: JSON.stringify(providerID ? { ...form, id: providerID } : form),
      });
      if (!providerID) await saveProviderKeys(saved.id, providerKeySecret);
      await props.onReload();
      if (!providerID) setSelectedProviderID(saved.id);
      setOpen(false);
      setEditingProviderID(null);
      setProviderKeySecret("");
      toast.success(t("common.save"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("toast.action_failed"));
    } finally {
      setSaving(false);
    }
  }

  async function savePresetProvider() {
    if (!presetID || selectedPresetExists) return;
    setSaving(true);
    try {
      const provider = await request<ProviderRecord>("/admin/api/provider-presets", props.token, {
        method: "POST",
        body: JSON.stringify({ id: presetID }),
      });
      await saveProviderKeys(provider.id, presetKeySecret);
      await props.onReload();
      setSelectedProviderID(provider.id);
      setPresetOpen(false);
      setPresetKeySecret("");
      toast.success(t("common.save"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("toast.action_failed"));
    } finally {
      setSaving(false);
    }
  }

  async function toggleProvider(provider: ProviderRecord) {
    setSaving(true);
    try {
      await request("/admin/api/providers", props.token, {
        method: "PUT",
        body: JSON.stringify({ ...provider, enabled: !provider.enabled }),
      });
      await props.onReload();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("toast.action_failed"));
    } finally {
      setSaving(false);
    }
  }

  async function saveProviderKeys(providerID: number, secretText: string, options?: Partial<ProviderKeyForm>) {
    const secrets = secretText.split(/\r?\n/).map((value) => value.trim()).filter(Boolean);
    if (!providerID || secrets.length === 0) return;
    await request("/admin/api/provider-keys", props.token, {
      method: "POST",
      body: JSON.stringify({
        provider_id: providerID,
        name: "",
        secret: secrets.length === 1 ? secrets[0] : "",
        secrets: secrets.length > 1 ? secrets : [],
        enabled: options?.enabled ?? true,
        priority: options?.priority ?? 0,
        weight: options?.weight ?? 1,
        labels: options?.labels ?? "",
      }),
    });
  }

  async function saveKeyForm() {
    if (!keyForm.provider_id) return;
    if (!editingKeyID && !keyForm.secret.trim()) return;
    setSaving(true);
    try {
      if (editingKeyID) {
        await request("/admin/api/provider-keys", props.token, {
          method: "PUT",
          body: JSON.stringify({
            id: editingKeyID,
            provider_id: keyForm.provider_id,
            name: keyForm.name,
            secret: keyForm.secret.trim(),
            enabled: keyForm.enabled,
            priority: keyForm.priority,
            weight: keyForm.weight,
            labels: keyForm.labels,
          }),
        });
      } else {
        await saveProviderKeys(keyForm.provider_id, keyForm.secret, keyForm);
      }
      await props.onReload();
      setKeyOpen(false);
      setEditingKeyID(null);
      toast.success(t("common.save"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("toast.action_failed"));
    } finally {
      setSaving(false);
    }
  }

  async function toggleKey(key: ProviderKey) {
    setSaving(true);
    try {
      await request("/admin/api/provider-keys", props.token, {
        method: "PUT",
        body: JSON.stringify({
          id: key.id,
          provider_id: key.provider_id,
          name: key.name,
          enabled: !key.enabled,
          priority: key.priority,
          weight: key.weight,
          labels: key.labels ?? "",
        }),
      });
      await props.onReload();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("toast.action_failed"));
    } finally {
      setSaving(false);
    }
  }

  async function deleteKey(key: ProviderKey) {
    setSaving(true);
    try {
      await request(`/admin/api/provider-keys?id=${key.id}`, props.token, { method: "DELETE" });
      await props.onReload();
    } finally {
      setSaving(false);
    }
  }

  async function toggleProviderKeyReveal(key: ProviderKey) {
    if (revealedProviderKeys[key.id]) {
      setRevealedProviderKeys((current) => {
        const next = { ...current };
        delete next[key.id];
        return next;
      });
      return;
    }
    const full = await request<ProviderKey>(`/admin/api/provider-keys?id=${key.id}&reveal=1`, props.token);
    setRevealedProviderKeys((current) => ({ ...current, [key.id]: full.secret }));
  }

  async function copyProviderKey(key: ProviderKey) {
    try {
      const value = revealedProviderKeys[key.id] || (await request<ProviderKey>(`/admin/api/provider-keys?id=${key.id}&reveal=1`, props.token)).secret;
      copyText(value);
      setCopiedProviderKeyID(key.id);
      window.setTimeout(() => setCopiedProviderKeyID((current) => current === key.id ? null : current), 1200);
    } catch {
      toast.error(t("common.copy_failed"));
    }
  }

  async function deleteProvider(provider: ProviderRecord) {
    setSaving(true);
    try {
      await request(`/admin/api/providers?id=${provider.id}`, props.token, { method: "DELETE" });
      if (selectedProviderID === provider.id) setSelectedProviderID(null);
      await props.onReload();
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="stack">
      {selectedProvider ? (
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="flex min-w-0 items-start gap-3">
                <Button type="button" variant="ghost" size="icon" onClick={() => setSelectedProviderID(null)} aria-label={t("providers.back_to_providers")}>
                  <ArrowLeft />
                </Button>
                <ProviderPresetIcon presetID={selectedProvider.preset_id || selectedProvider.type} className="h-11 w-11" />
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <CardTitle>{selectedProvider.name}</CardTitle>
                    <StatusBadge enabled={selectedProvider.enabled} />
                  </div>
                  <div className="mt-1 truncate text-sm text-muted-foreground">{providerTypeLabel(selectedProvider.type, t)}</div>
                  <div className="mt-2 max-w-3xl text-sm text-muted-foreground [overflow-wrap:anywhere]">{selectedProvider.base_url || "-"}</div>
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Button type="button" variant="outline" onClick={() => openKeyImport(selectedProvider.id)}><Plus />{t("provider_keys.import")}</Button>
                <ProviderActions
                  provider={selectedProvider}
                  saving={saving}
                  onEdit={() => openProvider(selectedProvider)}
                  onToggle={() => void toggleProvider(selectedProvider)}
                  onDelete={() => props.onRequestDeleteProvider(selectedProvider, () => deleteProvider(selectedProvider))}
                />
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <ProviderStat label={t("providers.type")} value={providerTypeLabel(selectedProvider.type, t)} />
              <ProviderStat label={t("providers.preset")} value={selectedProvider.preset_id || "-"} mono />
              <ProviderStat label={t("nav.provider_keys")} value={String(selectedProviderKeys.length)} />
            </div>
            <div className="flex items-center gap-2">
              <h3 className="text-lg font-semibold">{t("nav.provider_keys")}</h3>
              <Badge variant="secondary">{selectedProviderKeys.length}</Badge>
            </div>
            {selectedProviderKeys.length === 0 ? (
              <div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">{t("provider_keys.no_keys")}</div>
            ) : (
              <div className="data-table-card">
                <Table className="keys-table-inner">
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("providers.name")}</TableHead>
                      <TableHead>{t("keys.key")}</TableHead>
                      <TableHead className="right">{t("models.priority")}</TableHead>
                      <TableHead className="right">{t("models.weight")}</TableHead>
                      <TableHead>{t("common.status")}</TableHead>
                      <TableHead className="right">{t("common.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {selectedProviderKeys.map((key) => (
                      <TableRow key={key.id}>
                        <TableCell><div className="key-name-cell"><KeyRound size={15} /> {key.name}</div></TableCell>
                        <TableCell>
                          <div className="key-token-cell">
                            <div className="key-token mono">
                              <span>{revealedProviderKeys[key.id] ?? key.secret ?? key.secret_hint ?? "-"}</span>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button className="key-token-copy" variant="ghost" size="icon-sm" type="button" aria-label={t("common.copy")} onClick={() => void copyProviderKey(key)}>
                                    {copiedProviderKeyID === key.id ? <Check className="text-emerald-600 dark:text-emerald-400" size={15} /> : <Copy size={15} />}
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>{t("common.copy")}</TooltipContent>
                              </Tooltip>
                            </div>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button variant="ghost" size="icon-sm" type="button" aria-label={revealedProviderKeys[key.id] ? t("common.hide") : t("common.show")} onClick={() => void toggleProviderKeyReveal(key)}>
                                  {revealedProviderKeys[key.id] ? <EyeOff size={15} /> : <Eye size={15} />}
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>{revealedProviderKeys[key.id] ? t("common.hide") : t("common.show")}</TooltipContent>
                            </Tooltip>
                          </div>
                        </TableCell>
                        <TableCell className="right mono">{key.priority}</TableCell>
                        <TableCell className="right mono">{key.weight}</TableCell>
                        <TableCell><ProviderKeyStatusBadge item={key} /></TableCell>
                        <TableCell className="right">
                          <div className="flex justify-end gap-1">
                            <Button type="button" variant="ghost" size="icon-sm" onClick={() => openKeyEdit(key)} aria-label={t("common.edit")}><Pencil /></Button>
                            <EnabledToggleButton enabled={key.enabled} disabled={saving} onClick={() => void toggleKey(key)} />
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-sm"
                              className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                              disabled={saving}
                              onClick={() => props.onRequestDeleteProviderKey(key, () => deleteKey(key))}
                              aria-label={t("common.delete")}
                            >
                              <Trash2 />
                            </Button>
                          </div>
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
              <h2 className="text-2xl font-semibold tracking-tight">{t("nav.providers")}</h2>
              <Badge variant="secondary">{props.providers.length}</Badge>
            </div>
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={() => {
                setPresetID(props.providerPresets.find((preset) => !providerPresetExists(preset, existingProviderNames, existingProviderPresetIDs))?.id ?? props.providerPresets[0]?.id ?? "");
                setPresetKeySecret("");
                setPresetOpen(true);
              }}>
                <Plus />{t("providers.add_from_preset")}
              </Button>
              <Button type="button" onClick={() => openProvider()}><Plus />{t("providers.new_provider")}</Button>
            </div>
          </div>

          <div className="grid gap-3">
            {props.providers.map((provider) => {
              const keys = keysByProvider.get(provider.id) ?? [];
              return (
                <Card
                  key={provider.id}
                  role="button"
                  tabIndex={0}
                  onClick={() => setSelectedProviderID(provider.id)}
                  onKeyDown={(event) => activateCard(event, () => setSelectedProviderID(provider.id))}
                  className="cursor-pointer transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  <CardContent className="px-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="flex min-w-0 items-start gap-3">
                        <ProviderPresetIcon presetID={provider.preset_id || provider.type} className="h-10 w-10" />
                        <div className="min-w-0">
                          <div className="flex min-w-0 items-center gap-2">
                            <div className="truncate text-base font-semibold">{provider.name}</div>
                            <StatusBadge enabled={provider.enabled} />
                          </div>
                          <div className="mt-1 flex min-w-0 flex-wrap items-center gap-2">
                            <ProviderChip label={t("providers.preset")} value={provider.preset_id || "-"} mono />
                            <ProviderChip label={t("nav.provider_keys")} value={String(keys.length)} />
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2" onClick={stopCardAction}>
                        <ProviderActions
                          provider={provider}
                          saving={saving}
                          onEdit={() => openProvider(provider)}
                          onToggle={() => void toggleProvider(provider)}
                          onDelete={() => props.onRequestDeleteProvider(provider, () => deleteProvider(provider))}
                        />
                      </div>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </div>
      )}

      <Dialog open={open} onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        if (!nextOpen) setEditingProviderID(null);
      }}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader><DialogTitle>{editingProviderID ? t("providers.edit_provider") : t("providers.new_provider")}</DialogTitle></DialogHeader>
          <div className="grid gap-4 py-4">
            <FormTextField label={t("providers.name")} value={form.name} onChange={(name) => setForm({ ...form, name })} />
            <FormSelectField
              label={t("providers.type")}
              value={form.type}
              options={providerTypes.map((type) => ({ value: type, label: providerTypeLabel(type, t) }))}
              onChange={changeType}
            />
            <FormTextField label={t("providers.preset")} value={form.preset_id} onChange={(preset_id) => setForm({ ...form, preset_id })} />
            <FormTextField label={t("providers.base_url")} value={form.base_url} onChange={(base_url) => setForm({ ...form, base_url })} />
            {!editingProviderID && (
              <FormTextareaField label={t("provider_keys.optional_keys")} className="mono min-h-28" value={providerKeySecret} onChange={setProviderKeySecret} />
            )}
            <FormSelectField
              label={t("common.status")}
              value={form.enabled ? "1" : "0"}
              options={[
                { value: "1", label: t("common.active") },
                { value: "0", label: t("common.disabled") },
              ]}
              onChange={(value) => setForm({ ...form, enabled: value === "1" })}
            />
          </div>
          <DialogFooter><Button disabled={saving || !form.name || !form.type} onClick={() => void saveProvider()}>{t("common.save")}</Button></DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={presetOpen} onOpenChange={(nextOpen) => {
        setPresetOpen(nextOpen);
        if (!nextOpen) setPresetKeySecret("");
      }}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader><DialogTitle>{t("providers.add_from_preset")}</DialogTitle></DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid max-h-[min(460px,45dvh)] gap-2 overflow-y-auto pr-1 sm:grid-cols-2 lg:grid-cols-3">
              {props.providerPresets.map((preset) => {
                const exists = providerPresetExists(preset, existingProviderNames, existingProviderPresetIDs);
                const selected = preset.id === presetID;
                return (
                  <button
                    key={preset.id}
                    type="button"
                    disabled={exists}
                    onClick={() => setPresetID(preset.id)}
                    className={`flex min-w-0 items-center gap-3 rounded-xl border px-4 py-3 text-left transition-colors hover:bg-muted/50 disabled:cursor-not-allowed disabled:opacity-50 ${selected ? "border-primary bg-primary/5" : "border-border bg-background"}`}
                  >
                    <ProviderPresetIcon presetID={preset.id} />
                    <span className="min-w-0 flex-1 truncate font-medium">{providerPresetDisplayName(preset, t)}</span>
                    {exists && <Badge variant="outline">{t("providers.already_exists")}</Badge>}
                  </button>
                );
              })}
              <button
                type="button"
                onClick={() => {
                  setPresetOpen(false);
                  openProvider();
                }}
                className="flex min-w-0 items-center gap-3 rounded-xl border border-dashed bg-background px-4 py-3 text-left transition-colors hover:bg-muted/50"
              >
                <ProviderPresetIcon presetID="custom" />
                <span className="min-w-0 flex-1 truncate font-medium">{t("providers.custom")}</span>
              </button>
            </div>
            {selectedPreset && (
              <div className="rounded-md border p-3 text-sm">
                <div className="flex items-center gap-2">
                  <strong>{providerPresetDisplayName(selectedPreset, t)}</strong>
                  <Badge variant="secondary">{selectedPreset.type}</Badge>
                  {selectedPresetExists && <Badge variant="outline">{t("providers.already_exists")}</Badge>}
                </div>
                <div className="mt-2 text-muted-foreground">{selectedPreset.description}</div>
                <div className="mt-3 mono text-xs text-muted-foreground [overflow-wrap:anywhere]">{selectedPreset.base_url || "-"}</div>
              </div>
            )}
            <FormTextareaField label={t("provider_keys.optional_keys")} className="mono min-h-28" value={presetKeySecret} onChange={setPresetKeySecret} />
          </div>
          <DialogFooter>
            <Button disabled={saving || !presetID || selectedPresetExists} onClick={() => void savePresetProvider()}>{t("providers.add_provider")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={keyOpen} onOpenChange={(nextOpen) => {
        setKeyOpen(nextOpen);
        if (!nextOpen) setEditingKeyID(null);
      }}>
        <DialogContent className="overflow-hidden">
          <DialogHeader><DialogTitle>{editingKeyID ? t("provider_keys.edit_key") : t("provider_keys.import")}</DialogTitle></DialogHeader>
          <div className="grid gap-4 py-4">
            <FormStaticField label={t("models.provider")}>
              <div className="rounded-md border bg-muted px-3 py-2 text-sm">{keyDialogProvider?.name ?? "-"}</div>
            </FormStaticField>
            <FormTextareaField label={editingKeyID ? t("provider_keys.replace_key") : t("keys.key")} className="mono min-h-32" value={keyForm.secret} onChange={(secret) => setKeyForm({ ...keyForm, secret })} />
            <div className="grid gap-4 md:grid-cols-2">
              <FormNumberField label={t("models.priority")} tip={t("provider_keys.tip_priority")} value={keyForm.priority} onChange={(priority) => setKeyForm({ ...keyForm, priority })} />
              <FormNumberField label={t("models.weight")} tip={t("provider_keys.tip_weight")} value={keyForm.weight} onChange={(weight) => setKeyForm({ ...keyForm, weight })} />
            </div>
            <FormTextField label={t("provider_keys.labels")} tip={t("provider_keys.tip_labels")} value={keyForm.labels} onChange={(labels) => setKeyForm({ ...keyForm, labels })} />
            <FormSelectField
              label={t("common.status")}
              value={keyForm.enabled ? "1" : "0"}
              options={[
                { value: "1", label: t("common.active") },
                { value: "0", label: t("common.disabled") },
              ]}
              onChange={(value) => setKeyForm({ ...keyForm, enabled: value === "1" })}
            />
          </div>
          <DialogFooter><Button disabled={saving || !keyForm.provider_id || (!editingKeyID && !keyForm.secret.trim())} onClick={() => void saveKeyForm()}>{t("common.save")}</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function stopCardAction(event: MouseEvent<HTMLDivElement>) {
  event.stopPropagation();
}

function activateCard(event: KeyboardEvent<HTMLElement>, onActivate: () => void) {
  if (event.key !== "Enter" && event.key !== " ") return;
  event.preventDefault();
  onActivate();
}

function ProviderActions(props: {
  provider: ProviderRecord;
  saving: boolean;
  onEdit: () => void;
  onToggle: () => void;
  onDelete: () => void;
}) {
  const { t } = useLocale();
  return (
    <div className="flex justify-end gap-1">
      <Button type="button" variant="ghost" size="icon-sm" onClick={props.onEdit} aria-label={t("common.edit")}><Pencil /></Button>
      <EnabledToggleButton enabled={props.provider.enabled} disabled={props.saving} size="icon-sm" onClick={props.onToggle} />
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        className="text-destructive hover:bg-destructive/10 hover:text-destructive"
        disabled={props.saving}
        onClick={props.onDelete}
        aria-label={t("common.delete")}
      >
        <Trash2 />
      </Button>
    </div>
  );
}

function ProviderStat({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="min-w-0 rounded-lg bg-muted/50 px-3 py-2">
      <div className="truncate text-xs text-muted-foreground">{label}</div>
      <div className={`mt-1 truncate text-sm font-medium ${mono ? "mono" : ""}`}>{value || "-"}</div>
    </div>
  );
}

function ProviderChip({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <Badge variant="secondary" className="shrink-0 gap-1">
      <span className="text-muted-foreground">{label}</span>
      <span className={mono ? "mono" : ""}>{value || "-"}</span>
    </Badge>
  );
}

type BrandIconComponent = (props: { className?: string }) => ReturnType<typeof GeminiIcon>;

function ProviderPresetIcon({ presetID, className = "h-8 w-8" }: { presetID: string; className?: string }) {
  const Icon = providerPresetIcon(presetID);
  if (Icon) return <Icon className={`${className} shrink-0`} />;

  return (
    <span className={`${className} flex shrink-0 items-center justify-center rounded-lg border border-dashed text-muted-foreground`}>
      <Settings2 className="h-1/2 w-1/2" strokeWidth={2} />
    </span>
  );
}

function providerPresetIcon(presetID: string): BrandIconComponent | null {
  switch (presetID) {
    case "gemini":
      return GeminiIcon;
    case "openai":
      return OpenAIIcon;
    case "anthropic":
      return ClaudeIcon;
    case "mimo":
    case "mimo-plan":
      return MimoIcon;
    case "deepseek":
      return DeepSeekIcon;
    case "qwen":
      return QwenIcon;
    case "moonshot":
      return MoonshotIcon;
    case "zhipu":
      return ZhipuIcon;
    case "openrouter":
      return OpenRouterIcon;
    default:
      return null;
  }
}

function providerTypeLabel(type: string, t: (key: string) => string) {
  switch (type) {
    case "gemini":
      return t("providers.type_gemini");
    case "openai":
      return t("providers.type_openai");
    case "anthropic":
      return t("providers.type_anthropic");
    case "openai-compatible":
      return t("providers.type_openai_compatible");
    default:
      return type;
  }
}

function providerPresetDisplayName(preset: ProviderPreset, t: (key: string) => string) {
  const key = `providers.preset_${preset.id}`;
  const label = t(key);
  return label === key ? preset.name : label;
}

function providerPresetExists(preset: ProviderPreset, names: Set<string>, presetIDs: Set<string>) {
  return presetIDs.has(preset.id) || names.has(preset.name.toLowerCase());
}

function StatusBadge({ enabled }: { enabled: boolean }) {
  const { t } = useLocale();
  return enabled
    ? <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge>
    : <Badge variant="secondary">{t("common.disabled")}</Badge>;
}

function ProviderKeyStatusBadge({ item }: { item: ProviderKey }) {
  const { t } = useLocale();
  if (item.enabled) {
    return <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge>;
  }
  const reason = item.disabled_error_message || item.disabled_error_body || item.disabled_error_code;
  const label = item.disabled_status ? `${t("common.disabled")} ${item.disabled_status}` : t("common.disabled");
  if (!reason) return <Badge variant="secondary">{label}</Badge>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant="secondary" className="cursor-help">{label}</Badge>
      </TooltipTrigger>
      <TooltipContent className="max-w-96">
        <div className="mono [overflow-wrap:anywhere]">{reason}</div>
      </TooltipContent>
    </Tooltip>
  );
}

function copyText(value: string) {
  if (window.navigator.clipboard?.writeText && window.isSecureContext) {
    return window.navigator.clipboard.writeText(value);
  }
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  const ok = document.execCommand("copy");
  document.body.removeChild(textarea);
  if (!ok) throw new Error("copy failed");
  return Promise.resolve();
}
