import { useEffect, useMemo, useState } from "react";
import { CloudDownload } from "lucide-react";
import { toast } from "sonner";
import { request } from "../api/client";
import { tNow, useLocale } from "../i18n/locale";
import type { UpstreamModel } from "../types/admin";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "./ui/popover";

export function UpstreamModelPicker({ providerId, protocol, token, onSelect }: {
  providerId: number; protocol: string; token: string; onSelect: (model: UpstreamModel) => void;
}) {
  const { t } = useLocale();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [models, setModels] = useState<UpstreamModel[]>([]);
  const [search, setSearch] = useState("");

  useEffect(() => {
    if (!open || !providerId || !protocol) return;
    const controller = new AbortController();
    setModels([]);
    setLoading(true);
    request<UpstreamModel[]>(`/admin/api/providers/${providerId}/upstream-models?protocol=${encodeURIComponent(protocol)}`, token, { signal: controller.signal })
      .then((models) => { if (!controller.signal.aborted) setModels(models); })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) toast.error(error instanceof Error ? error.message : tNow("toast.action_failed"));
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [open, providerId, protocol, token]);

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    return models.filter((model) => `${model.id} ${model.name ?? ""}`.toLowerCase().includes(query));
  }, [models, search]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="outline" size="icon" disabled={!providerId || !protocol} aria-label={t("models.fetch_upstream")} title={t("models.fetch_upstream")}>
          <CloudDownload className="size-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 overflow-hidden p-0" align="end">
        <div className="border-b p-2"><Input placeholder={t("models.search_upstream")} value={search} onChange={(event) => setSearch(event.target.value)} className="h-8" /></div>
        <div className="max-h-64 overflow-y-auto" onWheel={(event) => event.stopPropagation()}>
          {loading ? <div className="p-4 text-sm text-muted-foreground">{t("models.loading_upstream")}</div> : filtered.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground">{t("models.no_upstream")}</div>
          ) : filtered.map((model) => (
            <button key={model.id} type="button" className="flex w-full flex-col gap-0.5 px-3 py-2 text-left text-sm hover:bg-muted" title={model.id} onClick={() => { onSelect(model); setOpen(false); }}>
              <span className="w-full truncate">{model.name || model.id}</span>
              {model.name && <span className="w-full truncate text-xs text-muted-foreground">{model.id}</span>}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}
