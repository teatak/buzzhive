import { useEffect, useMemo, useState } from "react";
import { Activity, ArrowLeft, BarChart3, Check, CircleOff, Copy, KeyRound, Loader2, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { request } from "../api/client";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "../components/ui/alert-dialog";
import { Alert, AlertDescription } from "../components/ui/alert";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "../components/ui/dialog";
import { Input } from "../components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "../components/ui/tooltip";
import { EnabledToggleButton } from "../components/enabled-toggle-button";
import { tNow, useLocale } from "../i18n/locale";
import { fillUsageSeries, formatDate, naturalDayRange, usagePath } from "../lib/date";
import { formatCompactNumber } from "../lib/utils";
import type { AppUser, Model, UsageSummary, UserAPIKey, UserAPIKeyDetails } from "../types/admin";
import { UsageDashboard, type UsageFilter } from "./DashboardPage";

export function UserDetailPage(props: {
  token: string;
  userID: number;
  copiedTarget: string;
  onBack: () => void;
  onCopyText: (value: string, target: string) => void;
}) {
  const { t } = useLocale();
  const [user, setUser] = useState<AppUser | null>(null);
  const [keys, setKeys] = useState<UserAPIKeyDetails[] | null>(null);
  const [models, setModels] = useState<Model[]>([]);
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [usageFilter, setUsageFilter] = useState<UsageFilter>({ key_id: "all", model: "all", ...naturalDayRange() });
  const [createOpen, setCreateOpen] = useState(false);
  const [keyName, setKeyName] = useState("");
  const [generatedKey, setGeneratedKey] = useState<UserAPIKey | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<UserAPIKeyDetails | null>(null);
  const [loadError, setLoadError] = useState("");

  const basePath = `/admin/api/users/${props.userID}`;

  async function loadKeys() {
    setKeys((await request<UserAPIKeyDetails[] | null>(`${basePath}/api-keys`, props.token)) ?? []);
  }

  useEffect(() => {
    let active = true;
    setUser(null);
    setKeys(null);
    setUsage(null);
    setLoadError("");
    Promise.all([
      request<AppUser>(basePath, props.token),
      request<UserAPIKeyDetails[] | null>(`${basePath}/api-keys`, props.token),
      request<Model[] | null>("/admin/api/models", props.token),
    ]).then(([nextUser, nextKeys, nextModels]) => {
      if (!active) return;
      setUser(nextUser);
      setKeys(nextKeys ?? []);
      setModels(nextModels ?? []);
    }).catch((error) => {
      if (!active) return;
      const message = error instanceof Error ? error.message : tNow("toast.action_failed");
      setLoadError(message);
      toast.error(message);
    });
    return () => { active = false; };
  }, [props.token, props.userID]);

  useEffect(() => {
    if (!user || user.id !== props.userID) return;
    let active = true;
    setUsage(null);
    request<UsageSummary>(usagePath(usageFilter, `${basePath}/usage`), props.token)
      .then((next) => { if (active) setUsage(next); })
      .catch((error) => toast.error(error instanceof Error ? error.message : tNow("toast.action_failed")));
    return () => { active = false; };
  }, [props.token, props.userID, user?.id, usageFilter]);

  const usageSeries = useMemo(
    () => fillUsageSeries(usage?.series ?? [], usageFilter.from, usageFilter.to, usage?.bucket_minutes ?? 1),
    [usage?.series, usage?.bucket_minutes, usageFilter.from, usageFilter.to],
  );

  async function createKey() {
    try {
      const created = await request<UserAPIKey>(`${basePath}/api-keys`, props.token, {
        method: "POST",
        body: JSON.stringify({ name: keyName }),
      });
      setGeneratedKey(created);
      setKeyName("");
      await loadKeys();
      toast.success(tNow("keys.generated"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : tNow("toast.action_failed"));
    }
  }

  async function setKeyValid(key: UserAPIKeyDetails, valid: boolean) {
    try {
      await request(`${basePath}/api-keys/${key.id}`, props.token, {
        method: "PUT",
        body: JSON.stringify({ valid }),
      });
      await loadKeys();
      toast.success(tNow(valid ? "keys.api_key_enabled" : "keys.api_key_disabled"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : tNow("toast.action_failed"));
    }
  }

  async function deleteKey() {
    if (!deleteTarget) return;
    try {
      await request(`${basePath}/api-keys/${deleteTarget.id}`, props.token, { method: "DELETE" });
      if (usageFilter.key_id === String(deleteTarget.id)) setUsageFilter((current) => ({ ...current, key_id: "all" }));
      setDeleteTarget(null);
      await loadKeys();
      toast.success(tNow("keys.deleted"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : tNow("toast.action_failed"));
    }
  }

  function setCreateDialogOpen(open: boolean) {
    setCreateOpen(open);
    if (open) {
      setKeyName("");
      setGeneratedKey(null);
    }
  }

  if (loadError) {
    return (
      <div className="stack">
        <Button variant="ghost" className="w-fit" type="button" onClick={props.onBack}><ArrowLeft size={17} /> {t("users.back_to_users")}</Button>
        <Alert variant="destructive"><AlertDescription>{loadError}</AlertDescription></Alert>
      </div>
    );
  }

  if (!user || !keys) {
    return (
      <Card>
        <CardContent className="flex min-h-72 items-center justify-center">
          <Loader2 className="size-7 animate-spin text-muted-foreground" />
        </CardContent>
      </Card>
    );
  }

  const activeKeys = keys.filter((key) => key.valid);
  const today = naturalDayRange();
  const usageIsToday = usageFilter.from === today.from && usageFilter.to === today.to;

  return (
    <div className="stack">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <Button variant="ghost" size="icon" type="button" aria-label={t("users.back_to_users")} onClick={props.onBack}>
            <ArrowLeft size={17} />
          </Button>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="truncate text-2xl font-semibold tracking-tight">{user.username}</h2>
              {user.valid ? (
                <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge>
              ) : (
                <Badge variant="secondary">{t("common.disabled")}</Badge>
              )}
            </div>
            <p className="mt-1 text-sm text-muted-foreground">{user.role}</p>
          </div>
        </div>
        <Button type="button" onClick={() => setCreateDialogOpen(true)}><KeyRound size={15} /> {t("users.create_key")}</Button>
      </div>

      <Tabs defaultValue="overview" className="gap-4">
        <TabsList>
          <TabsTrigger value="overview">{t("users.overview")}</TabsTrigger>
          <TabsTrigger value="keys">{t("users.api_keys")}</TabsTrigger>
          <TabsTrigger value="usage">{t("users.usage")}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          {usage ? (
            <section className="metrics">
              <Card><CardContent className="metric-content"><div className="metric-label"><Activity size={17} /> {t("dashboard.requests")}</div><div className="metric-value">{usage.requests}</div></CardContent></Card>
              <Card><CardContent className="metric-content"><div className="metric-label"><CircleOff size={17} /> {t("dashboard.errors")}</div><div className="metric-value">{usage.errors}</div></CardContent></Card>
              <Card><CardContent className="metric-content"><div className="metric-label"><BarChart3 size={17} /> {t("usage.total_tokens")}</div><div className="metric-value" title={String(usage.total_tokens)}>{formatCompactNumber(usage.total_tokens)}</div></CardContent></Card>
              <Card><CardContent className="metric-content"><div className="metric-label"><KeyRound size={17} /> {t("users.active_keys")}</div><div className="metric-value">{activeKeys.length}</div></CardContent></Card>
            </section>
          ) : (
            <Card><CardContent className="flex min-h-48 items-center justify-center"><Loader2 className="size-7 animate-spin text-muted-foreground" /></CardContent></Card>
          )}
        </TabsContent>

        <TabsContent value="keys">
          <div className="data-table-card">
            <Table className="keys-table-inner">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("keys.name")}</TableHead>
                  <TableHead>{t("keys.api_key")}</TableHead>
                  <TableHead>{t("common.status")}</TableHead>
                  <TableHead>{t("keys.requests")}</TableHead>
                  <TableHead>{t("usage.total_tokens")}</TableHead>
                  <TableHead>{t("users.last_used")}</TableHead>
                  <TableHead className="right">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>{keys.length === 0 ? (
                <TableRow><TableCell colSpan={7} className="h-24 text-center text-muted-foreground">{t("users.no_keys")}</TableCell></TableRow>
              ) : keys.map((key) => (
                <TableRow key={key.id}>
                  <TableCell><div className="key-name-cell"><KeyRound size={15} /> {key.name}</div></TableCell>
                  <TableCell className="mono">{key.token}</TableCell>
                  <TableCell>{key.valid ? <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge> : <Badge variant="secondary">{t("common.disabled")}</Badge>}</TableCell>
                  <TableCell>{formatCompactNumber(key.requests)}</TableCell>
                  <TableCell>{formatCompactNumber(key.total_tokens)}</TableCell>
                  <TableCell>{key.last_used_at ? formatDate(key.last_used_at) : t("users.never_used")}</TableCell>
                  <TableCell className="right">
                    <div className="key-actions">
                      <EnabledToggleButton enabled={key.valid} onClick={() => void setKeyValid(key, !key.valid)} />
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="ghost" size="icon-sm" className="text-destructive hover:bg-destructive/10 hover:text-destructive" type="button" aria-label={t("common.delete")} onClick={() => setDeleteTarget(key)}>
                            <Trash2 size={15} />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t("common.delete")}</TooltipContent>
                      </Tooltip>
                    </div>
                  </TableCell>
                </TableRow>
              ))}</TableBody>
            </Table>
          </div>
        </TabsContent>

        <TabsContent value="usage">
          <UsageDashboard
            usage={usage}
            usageFilter={usageFilter}
            usageIsToday={usageIsToday}
            usageSeries={usageSeries}
            userAPIKeys={keys}
            models={models}
            ownActiveKeys={activeKeys}
            apiKeyMetricLabel={t("users.active_keys")}
            allKeysLabel={t("usage.all_user_keys")}
            onUsageFilterChange={setUsageFilter}
            onResetUsageToToday={() => setUsageFilter((current) => ({ ...current, ...naturalDayRange() }))}
            onSelectUsageRange={(from, to) => setUsageFilter((current) => ({ ...current, from, to }))}
          />
        </TabsContent>
      </Tabs>

      <Dialog open={createOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t("users.create_key_for", { username: user.username })}</DialogTitle></DialogHeader>
          <Input autoFocus placeholder={t("keys.key_name")} value={keyName} onChange={(event) => setKeyName(event.target.value)} />
          {generatedKey && (
            <Alert className="generated-key-alert">
              <AlertDescription className="mono [overflow-wrap:anywhere]">{generatedKey.token}</AlertDescription>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="ghost" size="icon-sm" type="button" aria-label={t("common.copy")} onClick={() => props.onCopyText(generatedKey.token, "generated-admin-user-key")}>
                    {props.copiedTarget === "generated-admin-user-key" ? <Check className="text-emerald-600 dark:text-emerald-400" size={15} /> : <Copy size={15} />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t("common.copy")}</TooltipContent>
              </Tooltip>
            </Alert>
          )}
          <DialogFooter>
            <Button variant="outline" type="button" onClick={() => setCreateDialogOpen(false)}>{t("common.cancel")}</Button>
            <Button type="button" disabled={!keyName.trim()} onClick={() => void createKey()}><KeyRound size={15} /> {t("keys.generate")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteTarget != null} onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("users.delete_key_title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("users.delete_key_body", { name: deleteTarget?.name ?? "" })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void deleteKey()}>{t("common.delete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
