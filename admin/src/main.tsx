import { StrictMode, useEffect, useState, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  BarChart3,
  CircleOff,
  Database,
  Gauge,
  KeyRound,
  LogOut,
  RefreshCw,
  ShieldCheck,
  Trash2,
  UserRound,
} from "lucide-react";
import "./styles.css";

type View = "dashboard" | "users" | "myKeys" | "accounts" | "runtime";

type Session = { user: AppUser };
type AppUser = { id: number; username: string; role: string; valid: boolean };
type UserAPIKey = { id: number; user_id: number; name: string; token: string; valid: boolean };
type GoogleAccount = { id: number; email: string; prefix: string; enabled: boolean };
type AdminKey = {
  id: number;
  account_id: number;
  name: string;
  key: string;
  enabled: boolean;
  account_email: string;
  account_prefix: string;
};
type AdminConfig = {
  addr: string;
  upstream_base_url: string;
  timeout: string;
  max_attempts: number;
  cooldown_seconds: number;
  models: string[];
};
type AdminData = {
  config: AdminConfig;
  users: AppUser[];
  user_api_keys: UserAPIKey[];
  accounts: GoogleAccount[];
  keys: AdminKey[];
};
type Stats = {
  started_at: string;
  requests: number;
  by_key: Record<string, number>;
  exhausted: Record<string, string>;
  last_updated: string;
};
type UsagePoint = { date: string; requests: number; errors: number; avg_latency_ms: number };
type UsageSummary = {
  requests: number;
  errors: number;
  avg_latency_ms: number;
  by_key: Record<string, number>;
  series: UsagePoint[];
};

const storageKey = "buzzhive-admin-key";
const viewFromHash = (): View => {
  switch (window.location.hash.replace("#", "")) {
    case "users":
      return "users";
    case "my-api-keys":
      return "myKeys";
    case "google-accounts":
      return "accounts";
    case "runtime":
      return "runtime";
    default:
      return "dashboard";
  }
};

const hashForView = (view: View) => {
  switch (view) {
    case "users":
      return "users";
    case "myKeys":
      return "my-api-keys";
    case "accounts":
      return "google-accounts";
    case "runtime":
      return "runtime";
    default:
      return "dashboard";
  }
};

function request<T>(path: string, token: string, options: RequestInit = {}): Promise<T> {
  return fetch(path, {
    ...options,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(options.headers ?? {}),
    },
  }).then(async (response) => {
    if (response.status === 401) throw new Error("unauthorized");
    if (!response.ok) throw new Error(await response.text());
    return response.json() as Promise<T>;
  });
}

function formatDate(value: string): string {
  if (!value || value.startsWith("0001-")) return "-";
  return new Date(value).toLocaleString();
}

function isoDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function isoMinute(date: Date): string {
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${isoDate(date)}T${hours}:${minutes}`;
}

function addMinutes(date: Date, minutes: number): Date {
  const next = new Date(date);
  next.setMinutes(next.getMinutes() + minutes);
  return next;
}

function usagePath(filter: { key_id: string; from: string; to: string }) {
  const params = new URLSearchParams({ from: filter.from, to: filter.to });
  if (filter.key_id !== "all") params.set("key_id", filter.key_id);
  return `/admin/api/usage?${params.toString()}`;
}

function fillUsageSeries(series: UsagePoint[], from: string, to: string): UsagePoint[] {
  const start = new Date(from);
  const end = new Date(to);
  const minuteCount = Math.floor((end.getTime() - start.getTime()) / 60000);
  if (!Number.isFinite(minuteCount) || minuteCount < 0) return series;
  if (minuteCount > 1440) {
    return series.length ? series : [
      { date: from, requests: 0, errors: 0, avg_latency_ms: 0 },
      { date: to, requests: 0, errors: 0, avg_latency_ms: 0 },
    ];
  }
  const byDate = new Map(series.map((point) => [point.date, point]));
  const out: UsagePoint[] = [];
  let cursor = start;
  while (cursor <= end) {
    const date = isoMinute(cursor);
    out.push(byDate.get(date) ?? { date, requests: 0, errors: 0, avg_latency_ms: 0 });
    cursor = addMinutes(cursor, 1);
  }
  return out;
}

function groupByAccount(keys: AdminKey[]) {
  const groups = new Map<string, AdminKey[]>();
  for (const key of keys) {
    const label = key.account_email || "Unmapped";
    groups.set(label, [...(groups.get(label) ?? []), key]);
  }
  return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
}

function App() {
  const [view, setView] = useState<View>(viewFromHash());
  const [token, setToken] = useState(localStorage.getItem(storageKey) ?? "");
  const [loginForm, setLoginForm] = useState({ username: "", password: "" });
  const [setupRequired, setSetupRequired] = useState(false);
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
  const [showUserDialog, setShowUserDialog] = useState(false);
  const [newAccount, setNewAccount] = useState({ email: "" });
  const [newKey, setNewKey] = useState({ account_id: "", key: "" });
  const [keyAccountFilter, setKeyAccountFilter] = useState("all");
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
    await request("/admin/api/user-api-keys", token, {
      method: "POST",
      body: JSON.stringify({
        name: newUserKey.name,
        token: newUserKey.token,
        valid: true,
      }),
    });
    setNewUserKey({ name: "", token: "" });
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
        if (token) load(token).catch(() => logout());
      });
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
  const accountGroups = groupByAccount(keys);
  const byKey = stats.by_key ?? {};
  const filteredKeys = keyAccountFilter === "all" ? keys : keys.filter((key) => String(key.account_id) === keyAccountFilter);
  const usageSeries = fillUsageSeries(usage?.series ?? [], usageFilter.from, usageFilter.to);
  const ownActiveKeys = userAPIKeys.filter((key) => key.valid);
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
                <input placeholder="API key" value={newUserKey.token} onChange={(event) => setNewUserKey({ ...newUserKey, token: event.target.value })} />
                <button className="button primary" type="button" onClick={createUserAPIKey} disabled={!newUserKey.token}>Add</button>
              </div>
              <table>
                <thead><tr><th>Name</th><th>API Key</th><th>Status</th></tr></thead>
                <tbody>{userAPIKeys.map((key) => (
                  <tr key={key.id}>
                    <td>{key.name}</td>
                    <td className="mono">{key.token}</td>
                    <td>{key.valid ? "active" : "disabled"}</td>
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
                <AccountCards groups={accountGroups} />
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
                </div>
                <table>
                  <thead><tr><th>Name</th><th>Account</th><th>Key</th><th className="right">Requests</th></tr></thead>
                  <tbody>{filteredKeys.map((key) => (
                    <tr key={key.name}>
                      <td className="mono">{key.name}</td>
                      <td>{key.account_email || <span className="muted">Unmapped</span>}</td>
                      <td className="mono">{key.key}</td>
                      <td className="right mono">{byKey[key.name] ?? 0}</td>
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

function AccountCards(props: { groups: Array<[string, AdminKey[]]> }) {
  return (
    <div className="account-grid">{props.groups.map(([email, keys]) => (
      <div className="account-card" key={email}>
        <div>
          <div className="account-email">{email}</div>
          <div className="muted">{keys[0]?.account_prefix || "-"} account id</div>
        </div>
        <div className="account-count">{keys.length}</div>
      </div>
    ))}</div>
  );
}

function UsageChart(props: { series: UsagePoint[] }) {
  const width = 720;
  const height = 220;
  const pad = 28;
  const max = Math.max(1, ...props.series.map((point) => point.requests));
  const points = props.series.map((point, index) => {
    const x = props.series.length <= 1 ? pad : pad + (index * (width - pad * 2)) / (props.series.length - 1);
    const y = height - pad - (point.requests / max) * (height - pad * 2);
    return { ...point, x, y };
  });
  const barWidth = Math.max(1, Math.min(18, ((width - pad * 2) / Math.max(points.length, 1)) * 0.6));
  const line = points.map((point) => `${point.x},${point.y}`).join(" ");
  return (
    <div className="chart-wrap">
      <svg className="usage-chart" viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Usage chart">
        <line x1={pad} y1={height - pad} x2={width - pad} y2={height - pad} />
        <line x1={pad} y1={pad} x2={pad} y2={height - pad} />
        {points.map((point) => (
          <g key={point.date}>
            <rect
              x={point.x - barWidth / 2}
              y={point.y}
              width={barWidth}
              height={height - pad - point.y}
              rx="3"
            />
          </g>
        ))}
        <polyline points={line} />
        {points.length <= 240 && points.map((point) => <circle key={`${point.date}-dot`} cx={point.x} cy={point.y} r="3.5" />)}
      </svg>
      <div className="chart-axis">
        <span>{props.series[0]?.date ?? "-"}</span>
        <span>{props.series[props.series.length - 1]?.date ?? "-"}</span>
      </div>
    </div>
  );
}

function UsageByKey(props: { usage: Record<string, number> }) {
  const rows = Object.entries(props.usage).sort((a, b) => b[1] - a[1]);
  if (!rows.length) return <div className="empty">No usage</div>;
  return (
    <div className="list">{rows.map(([key, count]) => (
      <div className="list-row" key={key}>
        <span className="mono">{key}</span>
        <span className="mono">{count}</span>
      </div>
    ))}</div>
  );
}

function Metric(props: { icon: ReactNode; label: string; value: ReactNode }) {
  return <section className="metric-card"><div className="metric-label">{props.icon}{props.label}</div><div className="metric-value">{props.value}</div></section>;
}

function Panel(props: { title: string; action?: ReactNode; children: ReactNode }) {
  return <section className="panel"><div className="panel-head"><h2>{props.title}</h2>{props.action}</div><div className="panel-body">{props.children}</div></section>;
}

function InfoRow(props: { label: string; value: string }) {
  return <div className="list-row"><span className="muted">{props.label}</span><span className="mono">{props.value}</span></div>;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
