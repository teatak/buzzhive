import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  KeyRound,
} from "lucide-react";
import { toast } from "sonner";
import { request, storageKey } from "./api/client";
import { LocaleToggle } from "./i18n/LocaleToggle";
import { tNow, useLocale } from "./i18n/locale";
import { fillUsageSeries, formatDate, modelUsagePath, naturalDayRange, usagePath } from "./lib/date";
import { hashForView, viewFromHash } from "./router/views";
import type {
  AdminConfig,
  AdminData,
  AdminKey,
  AppUser,
  GoogleAccount,
  Session,
  Stats,
  UsageSummary,
  ModelUsageSummary,
  UserAPIKey,
  View,
} from "./types/admin";
import { BrandLogo } from "./components/brand-logo";
import { Alert, AlertDescription } from "./components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "./components/ui/alert-dialog";
import { Badge } from "./components/ui/badge";
import { Button } from "./components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "./components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "./components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "./components/ui/field";
import { Input } from "./components/ui/input";
import { Label } from "./components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "./components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "./components/ui/tooltip";
import { ThemeToggle } from "./theme/ThemeToggle";
import { AdminLayout } from "./layout/AdminLayout";
import { DashboardPage } from "./views/DashboardPage";
import { MyApiKeysPage } from "./views/MyApiKeysPage";
import { UsersPage } from "./views/UsersPage";
import { RuntimePage } from "./views/RuntimePage";
import { AccountsPage } from "./views/AccountsPage";

type ConfirmAction = {
  title: string;
  description: string;
  confirmLabel: string;
  successLabel: string;
  onConfirm: () => Promise<void>;
};

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

export function App() {
  const { locale, t } = useLocale();
  const [view, setView] = useState<View>(viewFromHash());
  const [token, setToken] = useState(localStorage.getItem(storageKey) ?? "");
  const [loginForm, setLoginForm] = useState({ username: "", password: "" });
  const [setupRequired, setSetupRequired] = useState(false);
  const [booting, setBooting] = useState(true);
  const [session, setSession] = useState<Session | null>(null);
  const [config, setConfig] = useState<AdminConfig | null>(null);
  const [stats, setStats] = useState<Stats | null>(null);
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [modelUsage, setModelUsage] = useState<ModelUsageSummary | null>(null);
  const [users, setUsers] = useState<AppUser[]>([]);
  const [userAPIKeys, setUserAPIKeys] = useState<UserAPIKey[]>([]);
  const [accounts, setAccounts] = useState<GoogleAccount[]>([]);
  const [keys, setKeys] = useState<AdminKey[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [newUser, setNewUser] = useState({ username: "", password: "", role: "user" });
  const [passwordForm, setPasswordForm] = useState({ current_password: "", new_password: "" });
  const [newUserKey, setNewUserKey] = useState({ name: "", token: "" });
  const [generatedUserKey, setGeneratedUserKey] = useState<UserAPIKey | null>(null);
  const [revealedUserKeys, setRevealedUserKeys] = useState<Record<number, string>>({});
  const [copiedTarget, setCopiedTarget] = useState("");
  const copiedTimer = useRef<number | null>(null);
  const [showUserDialog, setShowUserDialog] = useState(false);
  const [showPasswordDialog, setShowPasswordDialog] = useState(false);
  const [confirmAction, setConfirmAction] = useState<ConfirmAction | null>(null);
  const [newAccount, setNewAccount] = useState({ email: "" });
  const [newKey, setNewKey] = useState({ account_id: "", key: "" });
  const [keyAccountFilter, setKeyAccountFilter] = useState("all");
  const [selectedKeyIds, setSelectedKeyIds] = useState<number[]>([]);
  const [usageFilter, setUsageFilter] = useState({
    key_id: "all",
    ...naturalDayRange(),
  });
  const [modelUsageFilter, setModelUsageFilter] = useState({
    key_id: "all",
    ...naturalDayRange(),
  });

  async function load(activeToken = token) {
    const [nextSession, data, nextStats, nextUsage, nextModelUsage] = await Promise.all([
      request<Session>("/admin/api/session", activeToken),
      request<AdminData>("/admin/api/data", activeToken),
      request<Stats>("/admin/api/stats", activeToken),
      request<UsageSummary>(usagePath(usageFilter), activeToken),
      request<ModelUsageSummary>(modelUsagePath(modelUsageFilter), activeToken),
    ]);
    setSession(nextSession);
    setConfig(data.config);
    setUsers(data.users);
    setUserAPIKeys(data.user_api_keys);
    setAccounts(data.accounts);
    setKeys(data.keys);
    setStats(nextStats);
    setUsage(nextUsage);
    setModelUsage(nextModelUsage);
  }

  async function loadUsage(activeToken = token) {
    setUsage(await request<UsageSummary>(usagePath(usageFilter), activeToken));
  }

  async function loadModelUsage(activeToken = token) {
    setModelUsage(await request<ModelUsageSummary>(modelUsagePath(modelUsageFilter), activeToken));
  }

  async function login() {
    setError("");
    setLoading(true);
    try {
      const path = setupRequired ? "/admin/api/setup" : "/admin/api/login";
      const result = await request<{ token: string; user: AppUser }>(path, "", {
        method: "POST",
        body: JSON.stringify(loginForm),
      });
      const nextToken = result.token;
      await load(nextToken);
      localStorage.setItem(storageKey, nextToken);
      setToken(nextToken);
    } catch {
      localStorage.removeItem(storageKey);
      setError(t("auth.invalid_key"));
    } finally {
      setLoading(false);
    }
  }

  async function refresh() {
    if (!token) return;
    setLoading(true);
    try {
      await load();
    } finally {
      setLoading(false);
    }
  }

  async function flushCooling() {
    await request("/admin/api/flush-exhausted", token, { method: "POST" });
    await refresh();
    toast.success(tNow("toast.cooling_cleared"));
  }

  async function createUser() {
    await request("/admin/api/users", token, {
      method: "POST",
      body: JSON.stringify(newUser),
    });
    setNewUser({ username: "", password: "", role: "user" });
    await refresh();
    toast.success(tNow("users.user_created"));
  }

  async function createUserAPIKey() {
    const created = await request<UserAPIKey>("/admin/api/user-api-keys", token, {
      method: "POST",
      body: JSON.stringify({
        name: newUserKey.name,
        valid: true,
      }),
    });
    setGeneratedUserKey(created);
    setNewUserKey({ name: "", token: "" });
    await refresh();
    toast.success(tNow("keys.generated"));
  }

  async function updateUserAPIKey(key: UserAPIKey, valid: boolean) {
    await request("/admin/api/user-api-keys", token, {
      method: "PUT",
      body: JSON.stringify({ id: key.id, valid }),
    });
    await refresh();
    toast.success(tNow(valid ? "keys.api_key_enabled" : "keys.api_key_disabled"));
  }

  async function toggleUserAPIKeyReveal(key: UserAPIKey) {
    if (revealedUserKeys[key.id]) {
      setRevealedUserKeys((current) => {
        const next = { ...current };
        delete next[key.id];
        return next;
      });
      return;
    }
    const full = await request<UserAPIKey>(`/admin/api/user-api-keys?id=${key.id}`, token);
    setRevealedUserKeys((current) => ({ ...current, [key.id]: full.token }));
  }

  async function copyUserAPIKey(key: UserAPIKey) {
    const value = revealedUserKeys[key.id]
      ? Promise.resolve(revealedUserKeys[key.id])
      : request<UserAPIKey>(`/admin/api/user-api-keys?id=${key.id}`, token).then((full) => full.token);
    await copyTextFromPromise(value, `user-key-${key.id}`);
  }

  async function deleteUserAPIKey(key: UserAPIKey) {
    await request(`/admin/api/user-api-keys?id=${key.id}`, token, { method: "DELETE" });
    await refresh();
  }

  async function createAccount() {
    await request("/admin/api/google-accounts", token, {
      method: "POST",
      body: JSON.stringify({ email: newAccount.email, enabled: true }),
    });
    setNewAccount({ email: "" });
    await refresh();
    toast.success(tNow("accounts.added"));
  }

  async function updateGoogleAccount(account: GoogleAccount, enabled: boolean) {
    await request("/admin/api/google-accounts", token, {
      method: "PUT",
      body: JSON.stringify({ ...account, enabled }),
    });
    await refresh();
    toast.success(tNow(enabled ? "accounts.enabled" : "accounts.disabled"));
  }

  async function deleteGoogleAccount(account: GoogleAccount) {
    await request(`/admin/api/google-accounts?id=${account.id}`, token, { method: "DELETE" });
    await refresh();
  }

  async function createKey() {
    const values = newKey.key.split(/\r?\n/).map((value) => value.trim()).filter(Boolean);
    for (const value of values) {
      await request("/admin/api/api-keys", token, {
        method: "POST",
        body: JSON.stringify({
          account_id: Number(newKey.account_id),
          key: value,
          enabled: true,
        }),
      });
    }
    setNewKey({ account_id: "", key: "" });
    await refresh();
    toast.success(tNow("keys.gemini_added", { count: values.length }));
  }

  async function deleteAPIKeys(ids: number[]) {
    if (!ids.length) return;
    await request(`/admin/api/api-keys?ids=${ids.join(",")}`, token, { method: "DELETE" });
    setSelectedKeyIds((current) => current.filter((id) => !ids.includes(id)));
    await refresh();
  }

  async function copyText(value: string, target = "copy") {
    await copyTextFromPromise(Promise.resolve(value), target);
  }

  async function copyTextFromPromise(valuePromise: Promise<string>, target = "copy") {
    try {
      if (window.navigator.clipboard?.write && window.ClipboardItem && window.isSecureContext) {
        await window.navigator.clipboard.write([
          new window.ClipboardItem({
            "text/plain": valuePromise.then((value) => new Blob([value], { type: "text/plain" })),
          }),
        ]);
      } else {
        const value = await valuePromise;
        if (window.navigator.clipboard?.writeText && window.isSecureContext) {
          await window.navigator.clipboard.writeText(value);
        } else {
          fallbackCopyText(value);
        }
      }
    } catch {
      try {
        const value = await valuePromise;
        fallbackCopyText(value);
      } catch {
        toast.error(tNow("common.copy_failed"));
        return;
      }
    }
    setCopiedTarget(target);
    if (copiedTimer.current) window.clearTimeout(copiedTimer.current);
    copiedTimer.current = window.setTimeout(() => setCopiedTarget(""), 1200);
  }

  async function runConfirmAction() {
    if (!confirmAction) return;
    try {
      await confirmAction.onConfirm();
      toast.success(confirmAction.successLabel);
      setConfirmAction(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : tNow("toast.action_failed"));
    }
  }

  async function logout() {
    if (token) {
      await request("/admin/api/logout", token, { method: "POST" }).catch(() => undefined);
    }
    localStorage.removeItem(storageKey);
    setToken("");
    setLoginForm({ username: "", password: "" });
    setSession(null);
    setConfig(null);
    setStats(null);
    setUsage(null);
    setModelUsage(null);
  }

  async function changePassword() {
    try {
      await request("/admin/api/password", token, {
        method: "PUT",
        body: JSON.stringify(passwordForm),
      });
      setPasswordForm({ current_password: "", new_password: "" });
      setShowPasswordDialog(false);
      toast.success(tNow("user.password_changed"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : tNow("user.password_change_failed"));
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), 5000);
    let active = true;

    async function boot() {
      try {
        const state = await request<{ setup_required: boolean }>("/admin/api/setup-state", "", { signal: controller.signal });
        if (!active) return;
        setSetupRequired(state.setup_required);
        if (token) await load(token).catch(() => logout());
      } catch {
        if (active) setError(tNow("auth.unavailable"));
      } finally {
        if (!active) return;
        window.clearTimeout(timer);
        setBooting(false);
      }
    }

    void boot();
    return () => {
      active = false;
      window.clearTimeout(timer);
      controller.abort();
    };
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  useEffect(() => () => {
    if (copiedTimer.current) window.clearTimeout(copiedTimer.current);
  }, []);

  useEffect(() => {
    const onHashChange = () => setView(viewFromHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  useEffect(() => {
    if (token && session) loadUsage().catch(() => undefined);
  }, [usageFilter.key_id, usageFilter.from, usageFilter.to]);

  useEffect(() => {
    if (token && session) loadModelUsage().catch(() => undefined);
  }, [modelUsageFilter.key_id, modelUsageFilter.from, modelUsageFilter.to]);

  function navigate(nextView: View) {
    const hash = hashForView(nextView);
    if (window.location.hash !== `#${hash}`) {
      window.location.hash = hash;
    }
    setView(nextView);
  }

  useEffect(() => {
    if (session && session.user.role !== "admin" && (view === "users" || view === "accounts")) {
      navigate("dashboard");
    }
  }, [session, view]);

  if (booting) {
    return (
      <main className="login-page">
        <div className="toolbar login-toolbar">
          <ThemeToggle />
          <LocaleToggle />
        </div>
        <div className="login-brand">
          <div className="mark login-mark"><BrandLogo className="size-5" /></div>
          <div>
            <h1>BuzzHive</h1>
            <p>{t("common.loading_admin")}</p>
          </div>
        </div>
      </main>
    );
  }

  if (!session || !config || !stats) {
    return (
      <main className="login-page">
        <div className="toolbar login-toolbar">
          <ThemeToggle />
          <LocaleToggle />
        </div>
        <div className="login-shell">
          <div className="login-brand">
            <div className="mark login-mark"><BrandLogo className="size-[35px]" /></div>
            <div>
              <h1>BuzzHive</h1>
              <p>{t("app.subtitle")}</p>
            </div>
          </div>
          <Card className="login-card gap-6">
            <CardHeader className="px-6 text-left">
              <CardTitle className="text-lg">{setupRequired ? t("auth.create_admin") : t("auth.login_title")}</CardTitle>
              <p className="text-sm text-muted-foreground">{setupRequired ? t("auth.create_admin_description") : t("auth.login_description")}</p>
            </CardHeader>
            <CardContent className="px-6">
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  void login();
                }}
              >
                <FieldGroup className="gap-7">
                  <Field>
                    <FieldLabel htmlFor="login-username">{t("auth.username")}</FieldLabel>
                    <Input
                      id="login-username"
                      autoFocus
                      autoComplete="username"
                      value={loginForm.username}
                      onChange={(event) => setLoginForm({ ...loginForm, username: event.target.value })}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="login-password">{t("auth.password")}</FieldLabel>
                    <Input
                      id="login-password"
                      type="password"
                      autoComplete={setupRequired ? "new-password" : "current-password"}
                      value={loginForm.password}
                      onChange={(event) => setLoginForm({ ...loginForm, password: event.target.value })}
                    />
                  </Field>
                  <Field>
                    <Button className="w-full" size="lg" type="submit" disabled={loading}>
                      <KeyRound size={16} /> {setupRequired ? t("auth.create_admin") : t("auth.login")}
                    </Button>
                    {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
                  </Field>
                </FieldGroup>
              </form>
            </CardContent>
          </Card>
        </div>
      </main>
    );
  }

  const coolingKeys = Object.entries(stats.exhausted ?? {});
  const byKey = stats.by_key ?? {};
  const modelUsageTotals = modelUsage?.total_by_model ?? [];
  const modelUsageSeries = modelUsage?.series ?? [];
  const modelUsageAccountTotals = modelUsage?.account_totals ?? [];
  const quotaSignals = modelUsage?.quota_signals ?? [];
  const recentModelErrors = modelUsage?.recent_errors ?? [];
  const filteredKeys = keyAccountFilter === "all" ? keys : keys.filter((key) => String(key.account_id) === keyAccountFilter);
  const filteredKeyIds = filteredKeys.map((key) => key.id);
  const allFilteredSelected = filteredKeyIds.length > 0 && filteredKeyIds.every((id) => selectedKeyIds.includes(id));
  const keyCountByAccount = keys.reduce<Record<number, number>>((acc, key) => {
    acc[key.account_id] = (acc[key.account_id] ?? 0) + 1;
    return acc;
  }, {});
  const usageSeries = fillUsageSeries(usage?.series ?? [], usageFilter.from, usageFilter.to);
  const ownActiveKeys = userAPIKeys.filter((key) => key.valid);
  const todayUsageRange = naturalDayRange();
  const usageIsToday = usageFilter.from === todayUsageRange.from && usageFilter.to === todayUsageRange.to;

  function selectUsageRange(from: string, to: string) {
    if (from === usageFilter.from && to === usageFilter.to) return;
    setUsageFilter({ ...usageFilter, from, to });
  }

  function resetUsageToToday() {
    setUsageFilter({ ...usageFilter, ...naturalDayRange() });
  }

  function toggleKeySelection(id: number, checked: boolean) {
    setSelectedKeyIds((current) => checked ? Array.from(new Set([...current, id])) : current.filter((item) => item !== id));
  }

  function toggleAllFilteredKeys(checked: boolean) {
    setSelectedKeyIds((current) => {
      const currentSet = new Set(current);
      for (const id of filteredKeyIds) {
        if (checked) currentSet.add(id);
        else currentSet.delete(id);
      }
      return Array.from(currentSet);
    });
  }

  function keyStatus(key: AdminKey): ReactNode {
    if (!key.enabled) {
      const detail = key.disabled_error_message || key.disabled_error_body || key.disabled_error_code;
      const label = key.disabled_status ? `${t("common.disabled")} ${key.disabled_status}` : t("common.disabled");
      if (!detail) return <Badge variant="secondary">{label}</Badge>;
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            <Badge variant="secondary" className="cursor-help">{label}</Badge>
          </TooltipTrigger>
          <TooltipContent className="max-w-96">
            <div className="grid gap-1 text-xs">
              {key.disabled_error_code && <div>{key.disabled_error_code}</div>}
              {key.disabled_at && <div>{formatDate(key.disabled_at)}</div>}
              <div className="mono [overflow-wrap:anywhere]">{detail}</div>
            </div>
          </TooltipContent>
        </Tooltip>
      );
    }
    const cooling = Object.entries(stats!.exhausted ?? {}).find(([name]) => name.endsWith(`::${key.name}`));
    if (cooling) return <Badge variant="outline" className="border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-300">{t("keys.cooling", { time: formatDate(cooling[1]) })}</Badge>;
    const error = Object.values(stats!.key_errors ?? {}).find((item) => item.key === key.name);
    if (error) {
      return (
        <Popover>
          <PopoverTrigger asChild>
            <Button variant="ghost" className="h-auto p-0" type="button">
              <Badge variant="destructive">{t("common.error")} {error.status || t("common.network")}</Badge>
            </Button>
          </PopoverTrigger>
          <PopoverContent align="start" className="w-96">
            <div className="grid gap-2 text-sm">
              <div>{t("model.model")}: <span className="mono">{error.model}</span></div>
              <div>{t("model.time")}: {formatDate(error.updated_at)}</div>
              <div className="mono [overflow-wrap:anywhere]">{error.message || "-"}</div>
            </div>
          </PopoverContent>
        </Popover>
      );
    }
    return <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge>;
  }
  const title = {
    dashboard: t("nav.dashboard"),
    users: t("nav.users"),
    myKeys: t("nav.my_keys"),
    accounts: t("nav.accounts"),
    runtime: t("nav.runtime"),
  }[view];

  return (
    <AdminLayout
      session={session}
      title={title}
      view={view}
      onNavigate={navigate}
      onChangePassword={() => setShowPasswordDialog(true)}
      onLogout={() => void logout()}
    >
        <div className="page-content flex flex-1 flex-col gap-4 p-4">
          {view === "dashboard" && (
            <DashboardPage
              usage={usage}
              usageFilter={usageFilter}
              usageIsToday={usageIsToday}
              usageSeries={usageSeries}
              userAPIKeys={userAPIKeys}
              ownActiveKeys={ownActiveKeys}
              onUsageFilterChange={setUsageFilter}
              onResetUsageToToday={resetUsageToToday}
              onSelectUsageRange={selectUsageRange}
            />
          )}

          {view === "users" && session.user.role === "admin" && (
            <UsersPage users={users} onNewUser={() => setShowUserDialog(true)} />
          )}

          {view === "myKeys" && (
            <MyApiKeysPage
              userAPIKeys={userAPIKeys}
              newUserKey={newUserKey}
              generatedUserKey={generatedUserKey}
              revealedUserKeys={revealedUserKeys}
              copiedTarget={copiedTarget}
              onNewUserKeyChange={setNewUserKey}
              onCreateUserAPIKey={() => void createUserAPIKey()}
              onCopyText={(value, target) => void copyText(value, target)}
              onCopyUserAPIKey={(key) => void copyUserAPIKey(key)}
              onToggleUserAPIKeyReveal={(key) => void toggleUserAPIKeyReveal(key)}
              onUpdateUserAPIKey={(key, valid) => void updateUserAPIKey(key, valid)}
              onRequestDeleteUserAPIKey={(key) => setConfirmAction({
                title: t("keys.delete_my_key_title"),
                description: t("keys.delete_my_key_body", { name: key.name }),
                confirmLabel: t("common.delete"),
                successLabel: t("keys.deleted"),
                onConfirm: () => deleteUserAPIKey(key),
              })}
            />
          )}

          {view === "accounts" && (
            <AccountsPage
              accounts={accounts}
              keys={keys}
              byKey={byKey}
              newAccount={newAccount}
              newKey={newKey}
              keyAccountFilter={keyAccountFilter}
              selectedKeyIds={selectedKeyIds}
              filteredKeys={filteredKeys}
              allFilteredSelected={allFilteredSelected}
              keyCountByAccount={keyCountByAccount}
              keyStatus={keyStatus}
              onNewAccountChange={setNewAccount}
              onNewKeyChange={setNewKey}
              onKeyAccountFilterChange={setKeyAccountFilter}
              onCreateAccount={() => void createAccount()}
              onCreateKey={() => void createKey()}
              onFlushCooling={() => void flushCooling()}
              onUpdateGoogleAccount={(account, enabled) => void updateGoogleAccount(account, enabled)}
              onRequestDeleteGoogleAccount={(account) => {
                const count = keys.filter((key) => key.account_id === account.id).length;
                setConfirmAction({
                  title: t("accounts.delete_title"),
                  description: t("accounts.delete_body", { email: account.email, count }),
                  confirmLabel: t("common.delete"),
                  successLabel: t("accounts.deleted"),
                  onConfirm: () => deleteGoogleAccount(account),
                });
              }}
              onRequestDeleteAPIKeys={(ids) => setConfirmAction({
                title: ids.length > 1 ? t("keys.delete_selected_title") : t("keys.delete_gemini_title"),
                description: ids.length > 1
                  ? t("keys.delete_selected_body", { count: ids.length })
                  : t("keys.delete_gemini_body", { name: keys.find((key) => key.id === ids[0])?.name ?? "" }),
                confirmLabel: t("common.delete"),
                successLabel: ids.length > 1 ? t("keys.gemini_deleted_many") : t("keys.gemini_deleted"),
                onConfirm: () => deleteAPIKeys(ids),
              })}
              onToggleKeySelection={toggleKeySelection}
              onToggleAllFilteredKeys={toggleAllFilteredKeys}
            />
          )}

          {view === "runtime" && (
            <RuntimePage
              stats={stats}
              config={config}
              coolingKeys={coolingKeys}
              keys={keys}
              modelUsageFilter={modelUsageFilter}
              modelUsageTotals={modelUsageTotals}
              modelUsageSeries={modelUsageSeries}
              modelUsageAccountTotals={modelUsageAccountTotals}
              quotaSignals={quotaSignals}
              recentModelErrors={recentModelErrors}
              onModelUsageFilterChange={setModelUsageFilter}
            />
          )}
        </div>
      <Dialog open={showPasswordDialog} onOpenChange={setShowPasswordDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("user.change_password")}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="field">
              <Label>{t("user.current_password")}</Label>
              <Input
                type="password"
                value={passwordForm.current_password}
                onChange={(event) => setPasswordForm({ ...passwordForm, current_password: event.target.value })}
              />
            </div>
            <div className="field">
              <Label>{t("user.new_password")}</Label>
              <Input
                type="password"
                value={passwordForm.new_password}
                onChange={(event) => setPasswordForm({ ...passwordForm, new_password: event.target.value })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" type="button" onClick={() => setShowPasswordDialog(false)}>{t("common.cancel")}</Button>
            <Button
              type="button"
              disabled={!passwordForm.current_password || !passwordForm.new_password}
              onClick={changePassword}
            >
              {t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={showUserDialog} onOpenChange={setShowUserDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("users.new_user")}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="field">
              <Label>{t("auth.username")}</Label>
              <Input value={newUser.username} onChange={(event) => setNewUser({ ...newUser, username: event.target.value })} />
            </div>
            <div className="field">
              <Label>{t("auth.password")}</Label>
              <Input type="password" value={newUser.password} onChange={(event) => setNewUser({ ...newUser, password: event.target.value })} />
            </div>
            <div className="field">
              <Label>{t("users.role")}</Label>
              <Select value={newUser.role} onValueChange={(value) => setNewUser({ ...newUser, role: value })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="user">user</SelectItem>
                  <SelectItem value="admin">admin</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" type="button" onClick={() => setShowUserDialog(false)}>{t("common.cancel")}</Button>
            <Button
              type="button"
              disabled={!newUser.username || !newUser.password}
              onClick={async () => {
                await createUser();
                setShowUserDialog(false);
              }}
            >
              {t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <AlertDialog open={!!confirmAction} onOpenChange={(open) => !open && setConfirmAction(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmAction?.title}</AlertDialogTitle>
            <AlertDialogDescription>{confirmAction?.description}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive/10 text-destructive hover:bg-destructive/20"
              onClick={(event) => {
                event.preventDefault();
                void runConfirmAction();
              }}
            >
              {confirmAction?.confirmLabel || t("common.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </AdminLayout>
  );
}
