package buzzhive

const adminHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>BuzzHive Admin</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f8fafc;
      --panel: #ffffff;
      --text: #0f172a;
      --muted: #64748b;
      --line: #d9e2ec;
      --strong-line: #cbd5e1;
      --accent: #0f766e;
      --accent-text: #ffffff;
      --warn: #b45309;
      --danger: #b91c1c;
      --shadow: 0 1px 2px rgba(15, 23, 42, .08);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font: 14px/1.45 ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    button, input { font: inherit; }
    .shell { min-height: 100vh; display: flex; flex-direction: column; }
    .topbar {
      height: 56px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      padding: 0 24px;
      border-bottom: 1px solid var(--line);
      background: rgba(255,255,255,.92);
      position: sticky;
      top: 0;
      z-index: 5;
      backdrop-filter: blur(10px);
    }
    .brand { display: flex; align-items: center; gap: 10px; min-width: 0; }
    .mark {
      width: 28px;
      height: 28px;
      border: 1px solid var(--strong-line);
      border-radius: 7px;
      display: grid;
      place-items: center;
      background: #ecfdf5;
      color: var(--accent);
      font-weight: 700;
    }
    h1 { font-size: 15px; line-height: 1; margin: 0; font-weight: 650; }
    .subtle { color: var(--muted); font-size: 12px; }
    .toolbar { display: flex; align-items: center; gap: 8px; }
    .wrap { width: min(1180px, 100%); margin: 0 auto; padding: 22px 24px 32px; }
    .grid { display: grid; gap: 14px; }
    .metrics { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    .cols { grid-template-columns: 1.1fr .9fr; align-items: start; margin-top: 14px; }
    .card {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
    }
    .card-head {
      min-height: 48px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 13px 14px;
      border-bottom: 1px solid var(--line);
    }
    .card-title { font-size: 13px; font-weight: 650; margin: 0; }
    .card-body { padding: 14px; }
    .metric { padding: 14px; }
    .metric-label { color: var(--muted); font-size: 12px; margin-bottom: 8px; }
    .metric-value { font-size: 26px; line-height: 1; font-weight: 700; letter-spacing: 0; }
    .row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      min-height: 38px;
      border-bottom: 1px solid #edf2f7;
    }
    .row:last-child { border-bottom: 0; }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 12px; }
    .pill {
      display: inline-flex;
      align-items: center;
      min-height: 24px;
      padding: 0 8px;
      border-radius: 999px;
      border: 1px solid var(--line);
      background: #f8fafc;
      color: #334155;
      white-space: nowrap;
      font-size: 12px;
    }
    .pill.ok { border-color: #99f6e4; background: #ecfdf5; color: #0f766e; }
    .pill.warn { border-color: #fed7aa; background: #fff7ed; color: var(--warn); }
    .btn {
      height: 34px;
      padding: 0 11px;
      border: 1px solid var(--strong-line);
      border-radius: 7px;
      background: #fff;
      color: var(--text);
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 7px;
    }
    .btn:hover { background: #f8fafc; }
    .btn.primary { border-color: var(--accent); background: var(--accent); color: var(--accent-text); }
    .btn.danger { color: var(--danger); }
    .btn:disabled { opacity: .55; cursor: default; }
    .login {
      min-height: calc(100vh - 56px);
      display: grid;
      place-items: center;
      padding: 24px;
    }
    .login-panel {
      width: min(420px, 100%);
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      padding: 18px;
    }
    .field { display: grid; gap: 8px; margin: 16px 0 12px; }
    label { font-size: 12px; color: var(--muted); }
    input {
      width: 100%;
      height: 38px;
      border: 1px solid var(--strong-line);
      border-radius: 7px;
      padding: 0 10px;
      background: #fff;
      color: var(--text);
    }
    input:focus { outline: 2px solid #99f6e4; border-color: var(--accent); }
    .error { color: var(--danger); min-height: 20px; font-size: 12px; margin-top: 8px; }
    .hidden { display: none !important; }
    .list { display: grid; gap: 4px; }
    .table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: 9px 8px; border-bottom: 1px solid #edf2f7; vertical-align: middle; }
    th { color: var(--muted); font-weight: 600; font-size: 12px; }
    .right { text-align: right; }
    .empty { color: var(--muted); padding: 10px 0; }
    @media (max-width: 860px) {
      .metrics, .cols { grid-template-columns: 1fr; }
      .topbar { padding: 0 14px; }
      .wrap { padding: 14px; }
      .metric-value { font-size: 22px; }
    }
  </style>
</head>
<body>
  <div class="shell">
    <header class="topbar">
      <div class="brand">
        <div class="mark">B</div>
        <div>
          <h1>BuzzHive</h1>
          <div class="subtle" id="serverLine">Local Gemini Proxy</div>
        </div>
      </div>
      <div class="toolbar">
        <span class="pill ok hidden" id="userPill"></span>
        <button class="btn hidden" id="refreshBtn" type="button" title="Refresh">↻</button>
        <button class="btn hidden" id="logoutBtn" type="button">Logout</button>
      </div>
    </header>

    <main id="loginView" class="login">
      <section class="login-panel">
        <h2 class="card-title" id="loginTitle">Admin Login</h2>
        <div class="field">
          <label for="usernameInput">Username</label>
          <input id="usernameInput" autocomplete="username" autofocus>
        </div>
        <div class="field">
          <label for="passwordInput">Password</label>
          <input id="passwordInput" type="password" autocomplete="current-password">
        </div>
        <button class="btn primary" id="loginBtn" type="button">Login</button>
        <div class="error" id="loginError"></div>
      </section>
    </main>

    <main id="appView" class="wrap hidden">
      <section class="grid metrics">
        <div class="card metric">
          <div class="metric-label">Requests</div>
          <div class="metric-value" id="requestsMetric">0</div>
        </div>
        <div class="card metric">
          <div class="metric-label">API Keys</div>
          <div class="metric-value" id="keysMetric">0</div>
        </div>
        <div class="card metric">
          <div class="metric-label">Cooling</div>
          <div class="metric-value" id="coolingMetric">0</div>
        </div>
        <div class="card metric">
          <div class="metric-label">Models</div>
          <div class="metric-value" id="modelsMetric">0</div>
        </div>
      </section>

      <section class="grid cols">
        <div class="grid">
          <section class="card">
            <div class="card-head">
              <h2 class="card-title">Auto Models</h2>
              <span class="pill" id="retryPill"></span>
            </div>
            <div class="card-body list" id="modelsList"></div>
          </section>

          <section class="card">
            <div class="card-head">
              <h2 class="card-title">Key Usage</h2>
              <button class="btn danger" id="flushBtn" type="button">Clear Cooling</button>
            </div>
            <div class="card-body">
              <table class="table">
                <thead><tr><th>Name</th><th>Key</th><th class="right">Requests</th></tr></thead>
                <tbody id="keysTable"></tbody>
              </table>
            </div>
          </section>
        </div>

        <div class="grid">
          <section class="card">
            <div class="card-head">
              <h2 class="card-title">Runtime</h2>
              <span class="pill ok" id="healthPill">online</span>
            </div>
            <div class="card-body list" id="runtimeList"></div>
          </section>

          <section class="card">
            <div class="card-head">
              <h2 class="card-title">Cooling Keys</h2>
              <span class="pill warn" id="coolingPill">0</span>
            </div>
            <div class="card-body list" id="coolingList"></div>
          </section>
        </div>
      </section>
    </main>
  </div>

  <script>
    const state = { token: localStorage.getItem('buzzhive-admin-key') || '', config: null, stats: null, setupRequired: false };
    const $ = (id) => document.getElementById(id);

    function headers() {
      return { 'Authorization': 'Bearer ' + state.token, 'Content-Type': 'application/json' };
    }
    async function api(path, options) {
      const res = await fetch(path, Object.assign({ headers: headers() }, options || {}));
      if (res.status === 401) throw new Error('unauthorized');
      if (!res.ok) throw new Error(await res.text());
      return res.json();
    }
    function showApp(show) {
      $('loginView').classList.toggle('hidden', show);
      $('appView').classList.toggle('hidden', !show);
      $('logoutBtn').classList.toggle('hidden', !show);
      $('refreshBtn').classList.toggle('hidden', !show);
      $('userPill').classList.toggle('hidden', !show);
    }
    function fmtDate(value) {
      if (!value || value.startsWith('0001-')) return '-';
      return new Date(value).toLocaleString();
    }
    function row(label, value) {
      return '<div class="row"><span class="subtle">' + label + '</span><span class="mono">' + value + '</span></div>';
    }
    async function login() {
      $('loginError').textContent = '';
      try {
        const path = state.setupRequired ? '/admin/api/setup' : '/admin/api/login';
        const result = await api(path, {
          method: 'POST',
          body: JSON.stringify({
            username: $('usernameInput').value.trim(),
            password: $('passwordInput').value
          })
        });
        state.token = result.token;
        localStorage.setItem('buzzhive-admin-key', result.token);
        $('userPill').textContent = result.user.username || 'admin';
        showApp(true);
        await load();
      } catch (err) {
        $('loginError').textContent = 'Login failed';
        localStorage.removeItem('buzzhive-admin-key');
      }
    }
    async function load() {
      const [config, stats] = await Promise.all([api('/admin/api/config'), api('/admin/api/stats')]);
      state.config = config;
      state.stats = stats;
      render();
    }
    function render() {
      const config = state.config;
      const stats = state.stats;
      const exhausted = stats.exhausted || {};
      $('serverLine').textContent = config.addr + ' -> ' + config.upstream_base_url;
      $('requestsMetric').textContent = stats.requests || 0;
      $('keysMetric').textContent = config.keys.length;
      $('coolingMetric').textContent = Object.keys(exhausted).length;
      $('modelsMetric').textContent = config.models.length;
      $('retryPill').textContent = config.max_attempts + ' attempts / ' + config.cooldown_seconds + 's';
      $('coolingPill').textContent = Object.keys(exhausted).length;

      $('modelsList').innerHTML = config.models.map((model, index) =>
        '<div class="row"><span><span class="pill">' + (index + 1) + '</span> <span class="mono">' + model + '</span></span><span class="subtle">fallback</span></div>'
      ).join('');

      const usage = stats.by_key || {};
      $('keysTable').innerHTML = config.keys.map((key) =>
        '<tr><td class="mono">' + key.name + '</td><td class="mono">' + key.key + '</td><td class="right mono">' + (usage[key.name] || 0) + '</td></tr>'
      ).join('');

      $('runtimeList').innerHTML =
        row('Started', fmtDate(stats.started_at)) +
        row('Last request', fmtDate(stats.last_updated)) +
        row('Timeout', config.timeout) +
        row('Tokens', config.tokens.join(', ') || '-');

      const coolingKeys = Object.keys(exhausted);
      $('coolingList').innerHTML = coolingKeys.length
        ? coolingKeys.map((key) => row(key, fmtDate(exhausted[key]))).join('')
        : '<div class="empty">No cooling keys</div>';
    }
    $('loginBtn').addEventListener('click', login);
    $('passwordInput').addEventListener('keydown', (event) => { if (event.key === 'Enter') login(); });
    $('refreshBtn').addEventListener('click', load);
    $('logoutBtn').addEventListener('click', () => {
      state.token = '';
      localStorage.removeItem('buzzhive-admin-key');
      showApp(false);
      $('usernameInput').focus();
    });
    $('flushBtn').addEventListener('click', async () => {
      await api('/admin/api/flush-exhausted', { method: 'POST' });
      await load();
    });

    api('/admin/api/setup-state')
      .then((setup) => {
        state.setupRequired = setup.setup_required;
        $('loginTitle').textContent = setup.setup_required ? 'Create Initial Admin' : 'Admin Login';
        $('loginBtn').textContent = setup.setup_required ? 'Create admin' : 'Login';
        if (!state.token) return;
        return api('/admin/api/session').then((session) => {
          $('userPill').textContent = session.user.username || 'admin';
          showApp(true);
          return load();
        });
      })
      .catch(() => showApp(false));
  </script>
</body>
</html>`
