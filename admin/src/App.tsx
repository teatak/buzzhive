import { useEffect, useState, type ReactNode } from "react";
import {
  Activity,
  BarChart3,
  CircleOff,
  Copy,
  Database,
  Gauge,
  KeyRound,
  LogOut,
  RefreshCw,
  ShieldCheck,
  Trash2,
  UserRound,
} from "lucide-react";
import { request, storageKey } from "./api/client";
import { addMinutes, fillUsageSeries, formatDate, isoMinute, usagePath } from "./lib/date";
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
  UserAPIKey,
  View,
} from "./types/admin";
import { InfoRow, Metric, Panel, UsageByKey, UsageChart } from "./components/common";

export function App() {
  const [view, setView] = useState<View>(viewFromHash());
  const [token, setToken] = useState(localStorage.getItem(storageKey) ?? "");
  const [loginForm, setLoginForm] = useState({ username: "", password: "" });
  const [setupRequired, setSetupRequired] = useState(false);
  const [booting, setBooting] = useState(true);
  const [session, setSession] = useState<Session | null>(null);
  const [config, setConfig] = useState<AdminConfig | null>(null);
  const [stats, setStats] = useState<Stats | null>(null);
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [users, setUsers] = useState<AppUser[]>([]);
  const [userAPIKeys, setUserAPIKeys] = useState<UserAPIKey[]>([]);
  const [accounts, setAccounts] = useState<GoogleAccount[]>([]);
  const [keys, setKeys] = useState<AdminKey[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [newUser, setNewUser] = useState({ username: "", password: "", role: "user" });
  const [newUserKey, setNewUserKey] = useState({ name: "", token: "" });
  const [generatedUserKey, setGeneratedUserKey] = useState<UserAPIKey | null>(null);
  const [showUserDialog, setShowUserDialog] = useState(false);
  const [newAccount, setNewAccount] = useState({ email: "" });
  const [newKey, setNewKey] = useState({ account_id: "", key: "" });
  const [keyAccountFilter, setKeyAccountFilter] = useState("all");
  const [selectedKeyIds, setSelectedKeyIds] = useState<number[]>([]);
  const [usageFilter, setUsageFilter] = useState({
    key_id: "all",
    from: isoMinute(addMinutes(new Date(), -360)),
    to: isoMinute(new Date()),
  });

  async function load(activeToken = token) {
    const [nextSession, data, nextStats, nextUsage] = await Promise.all([
      request<Session>("/admin/api/session", activeToken),
      request<AdminData>("/admin/api/data", activeToken),
      request<Stats>("/admin/api/stats", activeToken),
      request<UsageSummary>(usagePath(usageFilter), activeToken),
    ]);
    setSession(nextSession);
    setConfig(data.config);
    setUsers(data.users);
    setUserAPIKeys(data.user_api_keys);
    setAccounts(data.accounts);
    setKeys(data.keys);
    setStats(nextStats);
    setUsage(nextUsage);
  }

  async function loadUsage(activeToken = token) {
    setUsage(await request<UsageSummary>(usagePath(usageFilter), activeToken));
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
      setError("Key 无效");
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
  }

  async function createUser() {
    await request("/admin/api/users", token, {
      method: "POST",
      body: JSON.stringify(newUser),
    });
    setNewUser({ username: "", password: "", role: "user" });
    await refresh();
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
  }

  async function updateUserAPIKey(key: UserAPIKey, valid: boolean) {
    await request("/admin/api/user-api-keys", token, {
      method: "PUT",
      body: JSON.stringify({ id: key.id, valid }),
    });
    await refresh();
  }

  async function deleteUserAPIKey(key: UserAPIKey) {
    if (!window.confirm(`Delete My API Key "${key.name}"?`)) return;
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
  }

  async function updateGoogleAccount(account: GoogleAccount, enabled: boolean) {
    await request("/admin/api/google-accounts", token, {
      method: "PUT",
      body: JSON.stringify({ ...account, enabled }),
    });
    await refresh();
  }

  async function deleteGoogleAccount(account: GoogleAccount) {
    const count = keys.filter((key) => key.account_id === account.id).length;
    if (!window.confirm(`Delete Google account "${account.email}" and ${count} API keys?`)) return;
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
  }

  async function deleteAPIKeys(ids: number[]) {
    if (!ids.length) return;
    if (!window.confirm(`Delete ${ids.length} Gemini API key${ids.length === 1 ? "" : "s"}?`)) return;
    await request(`/admin/api/api-keys?ids=${ids.join(",")}`, token, { method: "DELETE" });
    setSelectedKeyIds((current) => current.filter((id) => !ids.includes(id)));
    await refresh();
  }

  async function copyText(value: string) {
    await navigator.clipboard?.writeText(value);
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
  }

  useEffect(() => {
    request<{ setup_required: boolean }>("/admin/api/setup-state", "")
      .then((state) => setSetupRequired(state.setup_required))
      .then(() => {
        if (token) return load(token).catch(() => logout());
      })
      .finally(() => setBooting(false));
  }, []);

  useEffect(() => {
    const onHashChange = () => setView(viewFromHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  useEffect(() => {
    if (token && session) loadUsage().catch(() => undefined);
  }, [usageFilter.key_id, usageFilter.from, usageFilter.to]);

  function navigate(nextView: View) {
    const hash = hashForView(nextView);
    if (window.location.hash !== `#${hash}`) {
      window.location.hash = hash;
    }
    setView(nextView);
  }

  useEffect(() => {
    if (session?.user.role !== "admin" && (view === "users" || view === "accounts")) {
      navigate("dashboard");
    }
  }, [session, view]);

  if (booting) {
    return <main className="login-page" />;
  }

  if (!session || !config || !stats) {
    return (
      <main className="login-page">
        <section className="login-panel">
          <div className="brand-line">
            <div className="mark"><ShieldCheck size={18} /></div>
            <div>
              <h1>BuzzHive</h1>
              <p>{setupRequired ? "Create initial admin" : "Gemini proxy admin"}</p>
            </div>
          </div>
          <label className="field">
            <span>Username</span>
            <input
              autoFocus
              value={loginForm.username}
              onChange={(event) => setLoginForm({ ...loginForm, username: event.target.value })}
            />
          </label>
          <label className="field">
            <span>Password</span>
            <input
              type="password"
              value={loginForm.password}
              onChange={(event) => setLoginForm({ ...loginForm, password: event.target.value })}
              onKeyDown={(event) => event.key === "Enter" && login()}
            />
          </label>
          <button className="button primary" type="button" onClick={login} disabled={loading}>
            <KeyRound size={16} /> {setupRequired ? "Create admin" : "Login"}
          </button>
          <div className="error">{error}</div>
        </section>
      </main>
    );
  }

  const coolingKeys = Object.entries(stats.exhausted ?? {});
  const byKey = stats.by_key ?? {};
  const filteredKeys = keyAccountFilter === "all" ? keys : keys.filter((key) => String(key.account_id) === keyAccountFilter);
  const filteredKeyIds = filteredKeys.map((key) => key.id);
  const allFilteredSelected = filteredKeyIds.length > 0 && filteredKeyIds.every((id) => selectedKeyIds.includes(id));
  const keyCountByAccount = keys.reduce<Record<number, number>>((acc, key) => {
    acc[key.account_id] = (acc[key.account_id] ?? 0) + 1;
    return acc;
  }, {});
  const usageSeries = fillUsageSeries(usage?.series ?? [], usageFilter.from, usageFilter.to);
  const ownActiveKeys = userAPIKeys.filter((key) => key.valid);

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
    const cooling = Object.entries(stats!.exhausted ?? {}).find(([name]) => name.endsWith(`::${key.name}`));
    if (cooling) return <span className="pill warn">Cooling {formatDate(cooling[1])}</span>;
    const error = Object.values(stats!.key_errors ?? {}).find((item) => item.key === key.name);
    if (error) {
      return (
        <details className="status-detail">
          <summary><span className="pill danger">Error {error.status || "network"}</span></summary>
          <div className="status-detail-body">
            <div>Model: <span className="mono">{error.model}</span></div>
            <div>Time: {formatDate(error.updated_at)}</div>
            <div className="mono">{error.message || "-"}</div>
          </div>
        </details>
      );
    }
    return <span className="pill success">active</span>;
  }
  const title = {
    dashboard: "Dashboard",
    users: "Users",
    myKeys: "My API Keys",
    accounts: "Google Accounts",
    runtime: "Runtime",
  }[view];

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand-line">
          <div className="mark"><ShieldCheck size={18} /></div>
          <div>
            <h1>BuzzHive</h1>
            <p>Gemini proxy admin</p>
          </div>
        </div>
        <nav className="nav-list">
          <NavButton active={view === "dashboard"} icon={<Gauge size={16} />} label="Dashboard" onClick={() => navigate("dashboard")} />
          <NavButton active={view === "myKeys"} icon={<KeyRound size={16} />} label="My API Keys" onClick={() => navigate("myKeys")} />
          {session.user.role === "admin" && <NavButton active={view === "users"} icon={<UserRound size={16} />} label="Users" onClick={() => navigate("users")} />}
          {session.user.role === "admin" && <NavButton active={view === "accounts"} icon={<Database size={16} />} label="Google Accounts" onClick={() => navigate("accounts")} />}
          <NavButton active={view === "runtime"} icon={<Activity size={16} />} label="Runtime" onClick={() => navigate("runtime")} />
        </nav>
      </aside>

      <section className="content-shell">
        <header className="topbar">
          <div>
            <h1>{title}</h1>
            <p>{config.addr} {"->"} {config.upstream_base_url}</p>
          </div>
          <div className="toolbar">
            <span className="pill success"><UserRound size={14} /> {session.user.username}</span>
            <button className="icon-button" type="button" onClick={refresh} disabled={loading} title="Refresh">
              <RefreshCw size={16} />
            </button>
            <button className="button" type="button" onClick={logout}>
              <LogOut size={16} /> Logout
            </button>
          </div>
        </header>

        <main className="workspace">
          {view === "dashboard" && (
            <div className="stack">
              <section className="metrics">
                <Metric icon={<Activity size={17} />} label="Requests" value={usage?.requests ?? 0} />
                <Metric icon={<KeyRound size={17} />} label="My API Keys" value={ownActiveKeys.length} />
                <Metric icon={<CircleOff size={17} />} label="Errors" value={usage?.errors ?? 0} />
                <Metric icon={<BarChart3 size={17} />} label="Avg Latency" value={`${Math.round(usage?.avg_latency_ms ?? 0)}ms`} />
              </section>
              <section className="layout wide-left">
                <Panel title="Usage" action={<span className="pill">{usageFilter.from} / {usageFilter.to}</span>}>
                  <div className="usage-filters">
                    <select value={usageFilter.key_id} onChange={(event) => setUsageFilter({ ...usageFilter, key_id: event.target.value })}>
                      <option value="all">All my API keys</option>
                      {userAPIKeys.map((key) => <option key={key.id} value={key.id}>{key.name}</option>)}
                    </select>
                    <input type="datetime-local" value={usageFilter.from} onChange={(event) => setUsageFilter({ ...usageFilter, from: event.target.value })} />
                    <input type="datetime-local" value={usageFilter.to} onChange={(event) => setUsageFilter({ ...usageFilter, to: event.target.value })} />
                  </div>
                  <UsageChart series={usageSeries} />
                </Panel>
                <Panel title="Usage By Key" action={<span className="pill">{Object.keys(usage?.by_key ?? {}).length} keys</span>}>
                  <UsageByKey usage={usage?.by_key ?? {}} />
                </Panel>
              </section>
            </div>
          )}

          {view === "users" && session.user.role === "admin" && (
            <Panel title="Users" action={<button className="button primary" type="button" onClick={() => setShowUserDialog(true)}>New User</button>}>
              <table>
                <thead><tr><th>Username</th><th>Role</th><th>Status</th></tr></thead>
                <tbody>{users.map((user) => (
                  <tr key={user.id}><td>{user.username}</td><td>{user.role}</td><td>{user.valid ? "active" : "disabled"}</td></tr>
                ))}</tbody>
              </table>
            </Panel>
          )}

          {view === "myKeys" && (
            <Panel title="My API Keys" action={<span className="pill">{userAPIKeys.length} keys</span>}>
              <div className="form-row key-form">
                <input placeholder="Key name" value={newUserKey.name} onChange={(event) => setNewUserKey({ ...newUserKey, name: event.target.value })} />
                <button className="button primary" type="button" onClick={createUserAPIKey}>Generate</button>
              </div>
              {generatedUserKey && (
                <div className="generated-key">
                  <span className="mono">{generatedUserKey.token}</span>
                  <button className="icon-button" type="button" title="Copy" onClick={() => copyText(generatedUserKey.token)}>
                    <Copy size={15} />
                  </button>
                </div>
              )}
              <table>
                <thead><tr><th>Name</th><th>API Key</th><th>Status</th><th></th></tr></thead>
                <tbody>{userAPIKeys.map((key) => (
                  <tr key={key.id}>
                    <td>{key.name}</td>
                    <td className="mono">{key.token}</td>
                    <td>{key.valid ? <span className="pill success">active</span> : <span className="pill">disabled</span>}</td>
                    <td className="right">
                      <button className="button" type="button" onClick={() => updateUserAPIKey(key, !key.valid)}>
                        {key.valid ? "Disable" : "Enable"}
                      </button>
                      <button className="icon-button" type="button" title="Delete" onClick={() => deleteUserAPIKey(key)}>
                        <Trash2 size={15} />
                      </button>
                    </td>
                  </tr>
                ))}</tbody>
              </table>
            </Panel>
          )}

          {view === "accounts" && (
            <div className="stack">
              <Panel title="Google Accounts" action={<span className="pill">{accounts.length} accounts</span>}>
                <div className="form-row">
                  <input placeholder="Email" value={newAccount.email} onChange={(event) => setNewAccount({ ...newAccount, email: event.target.value })} />
                  <button className="button primary" type="button" onClick={createAccount} disabled={!newAccount.email}>Add</button>
                </div>
                <table className="section-table">
                  <thead><tr><th>Email</th><th>Prefix</th><th>Status</th><th className="right">Keys</th><th></th></tr></thead>
                  <tbody>{accounts.map((account) => (
                    <tr key={account.id}>
                      <td>{account.email}</td>
                      <td className="mono">{account.prefix}</td>
                      <td>{account.enabled ? <span className="pill success">active</span> : <span className="pill">disabled</span>}</td>
                      <td className="right mono">{keyCountByAccount[account.id] ?? 0}</td>
                      <td className="right">
                        <button className="button" type="button" onClick={() => updateGoogleAccount(account, !account.enabled)}>
                          {account.enabled ? "Disable" : "Enable"}
                        </button>
                        <button className="icon-button" type="button" title="Delete" onClick={() => deleteGoogleAccount(account)}>
                          <Trash2 size={15} />
                        </button>
                      </td>
                    </tr>
                  ))}</tbody>
                </table>
              </Panel>

              <Panel title="Gemini API Keys" action={<button className="button danger" type="button" onClick={flushCooling}><Trash2 size={16} /> Clear Cooling</button>}>
                <div className="api-key-form">
                  <select value={newKey.account_id} onChange={(event) => setNewKey({ ...newKey, account_id: event.target.value })}>
                    <option value="">Google account</option>
                    {accounts.map((account) => <option key={account.id} value={account.id}>{account.email}</option>)}
                  </select>
                  <textarea
                    placeholder={"AIza...\nAIza..."}
                    rows={4}
                    value={newKey.key}
                    onChange={(event) => setNewKey({ ...newKey, key: event.target.value })}
                  />
                  <button className="button primary" type="button" onClick={createKey} disabled={!newKey.account_id || !newKey.key.trim()}>Add</button>
                </div>
                <div className="filter-row">
                  <select value={keyAccountFilter} onChange={(event) => setKeyAccountFilter(event.target.value)}>
                    <option value="all">All Google accounts</option>
                    {accounts.map((account) => <option key={account.id} value={account.id}>{account.email}</option>)}
                  </select>
                  <button className="button danger" type="button" onClick={() => deleteAPIKeys(selectedKeyIds)} disabled={!selectedKeyIds.length}>
                    <Trash2 size={16} /> Delete Selected
                  </button>
                </div>
                <table>
                  <thead><tr><th><input type="checkbox" checked={allFilteredSelected} onChange={(event) => toggleAllFilteredKeys(event.target.checked)} /></th><th>Name</th><th>Account</th><th>Key</th><th>Status</th><th className="right">Requests</th><th></th></tr></thead>
                  <tbody>{filteredKeys.map((key) => (
                    <tr key={key.name}>
                      <td><input type="checkbox" checked={selectedKeyIds.includes(key.id)} onChange={(event) => toggleKeySelection(key.id, event.target.checked)} /></td>
                      <td className="mono">{key.name}</td>
                      <td>{key.account_email || <span className="muted">Unmapped</span>}</td>
                      <td className="mono">{key.key}</td>
                      <td>{keyStatus(key)}</td>
                      <td className="right mono">{byKey[key.name] ?? 0}</td>
                      <td className="right">
                        <button className="icon-button" type="button" title="Delete" onClick={() => deleteAPIKeys([key.id])}>
                          <Trash2 size={15} />
                        </button>
                      </td>
                    </tr>
                  ))}</tbody>
                </table>
              </Panel>
            </div>
          )}

          {view === "runtime" && (
            <section className="layout">
              <Panel title="Runtime" action={<span className="pill success">online</span>}>
                <div className="list">
                  <InfoRow label="Started" value={formatDate(stats.started_at)} />
                  <InfoRow label="Last request" value={formatDate(stats.last_updated)} />
                  <InfoRow label="Timeout" value={config.timeout} />
                  <InfoRow label="Cooldown" value={`${config.cooldown_seconds}s`} />
                </div>
              </Panel>
              <Panel title="Cooling Keys" action={<span className="pill warn">{coolingKeys.length}</span>}>
                <div className="list">
                  {coolingKeys.length ? coolingKeys.map(([key, expires]) => (
                    <InfoRow key={key} label={key} value={formatDate(expires)} />
                  )) : <div className="empty">No cooling keys</div>}
                </div>
              </Panel>
            </section>
          )}
        </main>
      </section>
      {showUserDialog && (
        <div className="dialog-backdrop" role="presentation">
          <section className="dialog" role="dialog" aria-modal="true" aria-label="New user">
            <div className="dialog-head">
              <h2>New User</h2>
              <button className="icon-button" type="button" onClick={() => setShowUserDialog(false)}>x</button>
            </div>
            <div className="dialog-body">
              <label className="field">
                <span>Username</span>
                <input value={newUser.username} onChange={(event) => setNewUser({ ...newUser, username: event.target.value })} />
              </label>
              <label className="field">
                <span>Password</span>
                <input type="password" value={newUser.password} onChange={(event) => setNewUser({ ...newUser, password: event.target.value })} />
              </label>
              <label className="field">
                <span>Role</span>
                <select value={newUser.role} onChange={(event) => setNewUser({ ...newUser, role: event.target.value })}>
                  <option value="user">user</option>
                  <option value="admin">admin</option>
                </select>
              </label>
            </div>
            <div className="dialog-actions">
              <button className="button" type="button" onClick={() => setShowUserDialog(false)}>Cancel</button>
              <button
                className="button primary"
                type="button"
                disabled={!newUser.username || !newUser.password}
                onClick={async () => {
                  await createUser();
                  setShowUserDialog(false);
                }}
              >
                Create
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}

function NavButton(props: { active: boolean; icon: ReactNode; label: string; onClick: () => void }) {
  return <button className={props.active ? "nav-button active" : "nav-button"} type="button" onClick={props.onClick}>{props.icon}{props.label}</button>;
}
