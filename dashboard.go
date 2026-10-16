package main

const dashboardHTML = "<!DOCTYPE html>\n" +
	`<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>socks5-failover 控制台</title>
<style>
:root {
  --bg: #eef1f6; --card: #ffffff; --line: #e3e8ef; --line2: #eef1f5;
  --text: #161c2b; --muted: #6b7684;
  --brand: #4c5fd6; --brand2: #6d3fb0;
  --ok: #00a86b; --warn: #d97706; --bad: #dc2626;
  --shadow: 0 1px 3px rgba(16,24,40,.06), 0 1px 2px rgba(16,24,40,.04);
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0e131c; --card: #171e2b; --line: #263042; --line2: #202a3a;
    --text: #dbe3ee; --muted: #8b97a8;
    --shadow: 0 1px 3px rgba(0,0,0,.4);
  }
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Microsoft YaHei', sans-serif; background: var(--bg); color: var(--text); min-height: 100vh; }

.topbar { position: sticky; top: 0; z-index: 50; background: var(--card); border-bottom: 1px solid var(--line); box-shadow: var(--shadow); }
.topbar-inner { max-width: 1180px; margin: 0 auto; padding: 14px 24px 0; }
.brand-row { display: flex; align-items: center; gap: 14px; margin-bottom: 12px; flex-wrap: wrap; }
.logo { width: 38px; height: 38px; border-radius: 11px; background: linear-gradient(135deg, var(--brand), var(--brand2)); display: flex; align-items: center; justify-content: center; color: #fff; font-weight: 800; font-size: 15px; flex-shrink: 0; }
.brand-name { font-size: 17px; font-weight: 700; }
.brand-sub { font-size: 12px; color: var(--muted); margin-top: 1px; }
.spacer { flex: 1; }
.active-badge { display: inline-flex; align-items: center; gap: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: 20px; padding: 6px 14px; font-size: 12.5px; }
.active-badge .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--ok); animation: pulse 2s infinite; }
.active-badge .dot.dead { background: var(--bad); animation: none; }
.gh-link { display: inline-flex; align-items: center; color: var(--muted); margin-right: -6px; transition: color .12s; }
.gh-link:hover { color: var(--text); }
.gh-link svg { width: 22px; height: 22px; fill: currentColor; }
.tabs { display: flex; gap: 4px; }
.tab-btn { background: none; border: none; border-bottom: 2px solid transparent; padding: 10px 16px; font-size: 14px; cursor: pointer; color: var(--muted); }
.tab-btn:hover { color: var(--text); }
.tab-btn.active { color: var(--brand); border-bottom-color: var(--brand); font-weight: 600; }

.wrap { max-width: 1180px; margin: 0 auto; padding: 24px; }
.panel { display: none; }
.panel.active { display: block; }

.cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(350px, 1fr)); gap: 18px; }
.card { background: var(--card); border-radius: 14px; border: 1px solid var(--line); padding: 20px; box-shadow: var(--shadow); transition: all .2s; }
.card:hover { border-color: var(--brand); transform: translateY(-2px); box-shadow: 0 8px 22px rgba(76,95,214,0.16); }
.card.inactive { opacity: .55; }
.card-top { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.status-dot2 { width: 11px; height: 11px; border-radius: 50%; flex-shrink: 0; }
.status-dot2.healthy { background: var(--ok); box-shadow: 0 0 0 4px rgba(0,168,107,.14); }
.status-dot2.unhealthy { background: var(--bad); box-shadow: 0 0 0 4px rgba(220,38,38,.14); }
.status-dot2.disabled { background: var(--muted); }
.card-name { font-size: 16px; font-weight: 700; }
.card-addr { font-size: 12px; color: var(--muted); margin-left: auto; font-family: ui-monospace, Consolas, monospace; }
.meta { display: flex; gap: 14px; font-size: 12px; color: var(--muted); margin-bottom: 12px; flex-wrap: wrap; }
.latency-display { display: flex; align-items: baseline; gap: 5px; margin-bottom: 8px; }
.latency-num { font-size: 34px; font-weight: 800; letter-spacing: -1px; }
.latency-unit { font-size: 12px; color: var(--muted); }
.chart-wrap { background: var(--bg); border-radius: 10px; padding: 10px; }
.chart-label { font-size: 10.5px; color: var(--muted); margin-bottom: 6px; text-transform: uppercase; letter-spacing: 1px; }
canvas.lat { width: 100% !important; height: 84px !important; }
.traffic-table { width: 100%; border-collapse: collapse; margin-top: 12px; font-size: 12px; }
.traffic-table th, .traffic-table td { border-bottom: 1px solid var(--line2); padding: 5px 0 5px 8px; text-align: right; white-space: nowrap; }
.traffic-table tr:last-child th, .traffic-table tr:last-child td { border-bottom: none; }
.traffic-table tbody tr:hover { background: none; }
.traffic-table th { color: var(--muted); font-weight: 500; font-size: 11px; text-transform: none; letter-spacing: 0; }
.traffic-table th:first-child, .traffic-table td:first-child { text-align: left; padding-left: 0; color: var(--muted); font-weight: 400; }
.traffic-table td { font-family: ui-monospace, Consolas, monospace; font-variant-numeric: tabular-nums; color: var(--text); }
.cycle-range { font-size: 10.5px; color: var(--muted); margin-left: 4px; }
.refresh-info { text-align: center; margin-top: 22px; font-size: 12px; color: var(--muted); }

.box { background: var(--card); border: 1px solid var(--line); border-radius: 14px; padding: 20px; box-shadow: var(--shadow); }
.box + .box { margin-top: 18px; }
.box-head { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; flex-wrap: wrap; }
.box-title { font-size: 15px; font-weight: 700; }
.btn { border: 1px solid var(--line); background: var(--card); color: var(--text); border-radius: 8px; padding: 7px 13px; font-size: 12.5px; cursor: pointer; transition: all .12s; }
.btn:hover { border-color: var(--brand); color: var(--brand); }
.btn.primary { background: var(--brand); border-color: var(--brand); color: #fff; font-weight: 600; }
.btn.primary:hover { background: #3d4fc0; color: #fff; }
.btn.danger:hover { border-color: var(--bad); color: var(--bad); }
.btn.small { padding: 4px 9px; font-size: 12px; border-radius: 6px; }
.table-scroll { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; font-size: 13px; }
th, td { text-align: left; padding: 11px 12px; border-bottom: 1px solid var(--line2); white-space: nowrap; }
th { color: var(--muted); font-weight: 600; font-size: 11.5px; text-transform: uppercase; letter-spacing: .5px; }
tbody tr:hover { background: var(--bg); }
tr:last-child td { border-bottom: none; }
.badge { display: inline-block; padding: 2px 9px; border-radius: 10px; font-size: 11.5px; font-weight: 600; }
.badge.on { background: rgba(0,168,107,.12); color: var(--ok); }
.badge.warn { background: rgba(217,119,6,.12); color: var(--warn); }
.badge.off { background: rgba(107,118,132,.14); color: var(--muted); }
.badge.bad { background: rgba(220,38,38,.12); color: var(--bad); }
.mono { font-family: ui-monospace, Consolas, monospace; font-size: 12px; color: var(--muted); }
.muted { color: var(--muted); font-size: 13px; }

.modal-mask { display: none; position: fixed; inset: 0; background: rgba(10,15,25,.55); z-index: 100; align-items: center; justify-content: center; padding: 20px; }
.modal-mask.show { display: flex; }
.modal { background: var(--card); border-radius: 16px; width: 520px; max-width: 100%; padding: 22px 24px; box-shadow: 0 20px 60px rgba(0,0,0,.35); max-height: 90vh; overflow-y: auto; }
.modal h3 { font-size: 16px; margin-bottom: 16px; }
.field { margin-bottom: 12px; }
.field label { display: block; font-size: 12px; color: var(--muted); margin-bottom: 5px; }
.field input, .field select { width: 100%; border: 1px solid var(--line); border-radius: 8px; padding: 9px 11px; font-size: 13.5px; background: var(--bg); color: var(--text); }
.field input:focus, .field select:focus { outline: none; border-color: var(--brand); background: var(--card); }
.field-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.modal-actions { display: flex; gap: 10px; margin-top: 18px; align-items: center; }
.msg { font-size: 13px; min-height: 18px; }
.msg.ok { color: var(--ok); } .msg.err { color: var(--bad); }

.btn.danger-confirm { background: var(--bad); border-color: var(--bad); color: #fff; font-weight: 600; }
.btn.danger-confirm:hover { background: #b91c1c; border-color: #b91c1c; color: #fff; }
.confirm-desc { font-size: 13.5px; margin-bottom: 12px; }
.confirm-info { background: var(--bg); border: 1px solid var(--line); border-radius: 10px; padding: 10px 14px; font-size: 13px; margin-bottom: 12px; }
.confirm-info div { padding: 4px 0; display: flex; justify-content: space-between; gap: 18px; }
.confirm-info b { font-weight: 600; }
.confirm-warn { color: var(--bad); font-size: 12.5px; margin-bottom: 4px; }

.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 14px; }
.toolbar { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
select.inline { border: 1px solid var(--line); background: var(--bg); color: var(--text); border-radius: 8px; padding: 7px 10px; font-size: 13px; }
.tf-block { margin-bottom: 24px; }
.tf-block:last-child { margin-bottom: 0; }
.tf-title { font-size: 14px; font-weight: 700; margin-bottom: 8px; display: flex; gap: 10px; align-items: baseline; }
.tf-title .mono { font-weight: 400; }
.empty { padding: 28px; text-align: center; color: var(--muted); font-size: 13px; }
</style>
</head>
<body>
<div class="topbar">
  <div class="topbar-inner">
    <div class="brand-row">
      <div class="logo">SF</div>
      <div>
        <div class="brand-name">socks5-failover 控制台</div>
        <div class="brand-sub">SOCKS5 链路探测 · 节点管理 · 流量统计</div>
      </div>
      <div class="spacer"></div>
      <a class="gh-link" href="https://github.com/goodryb/socks5-failover" target="_blank" rel="noopener" title="GitHub 仓库">
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 0c4.42 0 8 3.58 8 8a8.013 8.013 0 0 1-5.45 7.59c-.4.08-.55-.17-.55-.38 0-.27.01-1.13.01-2.2 0-.75-.25-1.23-.54-1.48 1.78-.2 3.65-.88 3.65-3.95 0-.88-.31-1.59-.82-2.15.08-.2.36-1.02-.08-2.12 0 0-.67-.22-2.2.82-.64-.18-1.32-.27-2-.27-.68 0-1.36.09-2 .27-1.53-1.03-2.2-.82-2.2-.82-.44 1.1-.16 1.92-.08 2.12-.51.56-.82 1.28-.82 2.15 0 3.06 1.86 3.75 3.64 3.95-.23.2-.44.55-.51 1.07-.46.21-1.61.55-2.33-.66-.15-.24-.6-.83-1.23-.82-.67.01-.27.38.01.53.34.19.73.55.82 1.13.16.45.68 1.31 2.69.94 0 .67.01 1.3.01 1.49 0 .21-.15.45-.55.38A7.995 7.995 0 0 1 0 8c0-4.42 3.58-8 8-8Z"/></svg>
      </a>
      <div class="active-badge"><span class="dot" id="active-dot"></span><span id="active-name">加载中...</span></div>
    </div>
    <div class="tabs">
      <button class="tab-btn active" data-tab="monitor" onclick="switchTab('monitor')">监控</button>
      <button class="tab-btn" data-tab="nodes" onclick="switchTab('nodes')">节点管理</button>
      <button class="tab-btn" data-tab="traffic" onclick="switchTab('traffic')">流量统计</button>
      <button class="tab-btn" data-tab="settings" onclick="switchTab('settings')">设置</button>
    </div>
  </div>
</div>

<div class="wrap">
  <div class="panel active" id="panel-monitor">
    <div class="cards" id="cards"></div>
    <p class="refresh-info">每 <span id="interval">10</span> 秒自动刷新 · 上次更新: <span id="last-update">-</span></p>
  </div>

  <div class="panel" id="panel-nodes">
    <div class="box">
      <div class="box-head">
        <span class="box-title">节点列表</span>
        <div class="spacer"></div>
        <button class="btn primary" onclick="openNodeForm()">＋ 添加节点</button>
        <button class="btn" onclick="reloadNodes()">刷新</button>
      </div>
      <div class="table-scroll">
        <table>
          <thead><tr><th>#</th><th>名称</th><th>地址</th><th>优先级</th><th>认证</th><th>状态</th><th>今日流量</th><th>月度周期</th><th>操作</th></tr></thead>
          <tbody id="nodes-body"><tr><td colspan="9" class="muted">加载中...</td></tr></tbody>
        </table>
      </div>
    </div>
  </div>

  <div class="panel" id="panel-traffic">
    <div class="box">
      <div class="box-head">
        <span class="box-title">按日流量</span>
        <div class="spacer"></div>
        <select class="inline" id="traffic-days" onchange="reloadTraffic()">
          <option value="cycle">月度周期</option><option value="7" selected>最近 7 天</option><option value="14">最近 14 天</option><option value="30">最近 30 天</option><option value="90">最近 90 天</option>
        </select>
        <button class="btn" onclick="reloadTraffic()">刷新</button>
      </div>
      <div id="traffic-body"><span class="muted">加载中...</span></div>
    </div>
  </div>

  <div class="panel" id="panel-settings">
    <div class="box">
      <div class="box-head"><span class="box-title">全局设置</span></div>
      <div class="form-grid">
        <div class="field"><label>SOCKS5 监听地址 (重启生效)</label><input id="st-listen"></div>
        <div class="field"><label>控制台监听地址 (重启生效)</label><input id="st-status_listen"></div>
        <div class="field"><label>探测历史记录条数 (重启生效)</label><input id="st-history_size" type="number"></div>
        <div class="field"><label>探测 URL</label><input id="st-test_url"></div>
      </div>
    </div>
    <div class="box">
      <div class="box-head"><span class="box-title">节点探测设置</span></div>
      <div class="form-grid">
        <div class="field"><label>探测间隔 (秒)</label><input id="st-check_interval" type="number"></div>
        <div class="field"><label>探测超时 (秒)</label><input id="st-check_timeout" type="number"></div>
        <div class="field"><label>失败阈值 (次)</label><input id="st-fail_threshold" type="number"></div>
      </div>
      <div class="toolbar" style="margin-top:6px">
        <button class="btn primary" onclick="saveSettings()">保存设置</button>
        <span id="settings-msg" class="msg"></span>
      </div>
    </div>
  </div>
</div>

<div class="modal-mask" id="node-modal" onclick="if(event.target===this)closeNodeForm()">
  <div class="modal">
    <h3 id="nm-title">添加节点</h3>
    <input type="hidden" id="nf-id">
    <div class="field-row">
      <div class="field"><label>名称 *</label><input id="nf-name" placeholder="主代理"></div>
      <div class="field"><label>SOCKS5 地址 * (IP:Port)</label><input id="nf-addr" placeholder="127.0.0.1:7890"></div>
    </div>
    <div class="field-row">
      <div class="field"><label>优先级 (越小越优先)</label><input id="nf-priority" type="number" value="1"></div>
      <div class="field"><label>启用</label><select id="nf-enabled"><option value="1">是</option><option value="0">否</option></select></div>
    </div>
    <div class="field-row">
      <div class="field"><label>月度周期起始日 (1-31)</label><input id="nf-billing_day" type="number" min="1" max="31" value="1"></div>
      <div class="field"><label>&nbsp;</label><div class="muted" style="font-size:12px;line-height:1.5">每月自该日起统计月度流量；起始日大于当月天数时按月末计算</div></div>
    </div>
    <div class="field-row">
      <div class="field"><label>用户名</label><input id="nf-username" placeholder="可选"></div>
      <div class="field"><label>密码</label><input id="nf-password" type="password" placeholder="可选"></div>
    </div>
    <div class="modal-actions">
      <button class="btn primary" onclick="submitNode()">保存</button>
      <button class="btn" onclick="closeNodeForm()">取消</button>
      <span id="node-msg" class="msg"></span>
    </div>
  </div>
</div>

<div class="modal-mask" id="confirm-modal" onclick="if(event.target===this)closeConfirm()">
  <div class="modal" style="width:440px">
    <h3 id="cm-title"></h3>
    <div id="cm-desc"></div>
    <div class="modal-actions">
      <button class="btn danger-confirm" id="cm-ok" onclick="onConfirmOk()">确认</button>
      <button class="btn" onclick="closeConfirm()">取消</button>
    </div>
  </div>
</div>

<script>
function fmtBytes(n) {
  if (n === undefined || n === null) return '-';
  var u = ['B','KB','MB','GB','TB'], i = 0, v = n;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return (i === 0 ? v : v.toFixed(2)) + ' ' + u[i];
}
function esc(s) { return String(s === undefined ? '' : s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'); }

function api(method, path, body) {
  var headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  return fetch(path, { method: method, headers: headers, body: body !== undefined ? JSON.stringify(body) : undefined })
    .then(function(resp) {
      return resp.json().catch(function(){ return {}; }).then(function(data){
        if (!resp.ok) throw new Error(data.error || ('HTTP ' + resp.status));
        return data;
      });
    });
}
function switchTab(name) {
  document.querySelectorAll('.tab-btn').forEach(function(b){ b.classList.toggle('active', b.dataset.tab === name); });
  document.querySelectorAll('.panel').forEach(function(p){ p.classList.remove('active'); });
  document.getElementById('panel-' + name).classList.add('active');
  if (name === 'nodes') reloadNodes();
  if (name === 'traffic') reloadTraffic();
  if (name === 'settings') loadSettings();
}

function drawChart(canvas, history) {
  var ctx = canvas.getContext('2d');
  var w = canvas.width = canvas.offsetWidth * 2;
  var h = canvas.height = canvas.offsetHeight * 2;
  ctx.scale(2, 2);
  var rw = w / 2, rh = h / 2;
  ctx.clearRect(0, 0, rw, rh);
  if (!history || history.length < 2) {
    ctx.fillStyle = '#9aa5b1'; ctx.font = '12px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('数据积累中...', rw / 2, rh / 2); return;
  }
  var maxVal = 5000;
  var padding = { top: 8, bottom: 16, left: 4, right: 4 };
  var chartW = rw - padding.left - padding.right, chartH = rh - padding.top - padding.bottom;
  var step = chartW / (history.length - 1);
  ctx.beginPath();
  ctx.moveTo(padding.left, padding.top + chartH);
  history.forEach(function(r, i) {
    ctx.lineTo(padding.left + i * step, padding.top + chartH - Math.min(r.val / maxVal, 1) * chartH);
  });
  ctx.lineTo(padding.left + (history.length - 1) * step, padding.top + chartH);
  ctx.closePath();
  var grad = ctx.createLinearGradient(0, 0, 0, rh);
  grad.addColorStop(0, 'rgba(76,95,214,0.25)'); grad.addColorStop(1, 'rgba(76,95,214,0.03)');
  ctx.fillStyle = grad; ctx.fill();
  ctx.beginPath();
  history.forEach(function(r, i) {
    var x = padding.left + i * step, y = padding.top + chartH - Math.min(r.val / maxVal, 1) * chartH;
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  });
  ctx.strokeStyle = '#4c5fd6'; ctx.lineWidth = 2; ctx.stroke();
  history.forEach(function(r, i) {
    if (!r.ok) {
      ctx.beginPath();
      ctx.arc(padding.left + i * step, padding.top + chartH - Math.min(r.val / maxVal, 1) * chartH, 3, 0, Math.PI * 2);
      ctx.fillStyle = '#dc2626'; ctx.fill();
    }
  });
  ctx.fillStyle = '#6b7684'; ctx.font = '10px sans-serif'; ctx.textAlign = 'left';
  ctx.fillText(maxVal + 'ms', padding.left, padding.top + 10);
}

function renderCards(data) {
  var container = document.getElementById('cards');
  container.innerHTML = '';
  (data.upstreams || []).forEach(function(u) {
    var statusClass = !u.enabled ? 'disabled' : (u.healthy ? 'healthy' : 'unhealthy');
    var isActive = data.active_upstream === u.name;
    var latency = u.latency_ms || 0;
    var latencyColor = latency < 200 ? '#00a86b' : latency < 1000 ? '#d97706' : '#dc2626';
    var t = u.traffic || {};
    var card = document.createElement('div');
    card.className = 'card' + (!u.enabled ? ' inactive' : '');
    card.innerHTML =
      '<div class="card-top">' +
        '<span class="status-dot2 ' + statusClass + '"></span>' +
        '<span class="card-name">' + esc(u.name) + (isActive ? ' 🚀' : '') + '</span>' +
        '<span class="card-addr">' + esc(u.addr) + '</span>' +
      '</div>' +
      '<div class="meta">' +
        '<span>优先级 ' + u.priority + '</span><span>失败 ' + (u.fail_count||0) + ' 次</span>' +
      '</div>' +
      '<div class="latency-display"><span class="latency-num" style="color:' + latencyColor + '">' + latency + '</span><span class="latency-unit">ms</span></div>' +
      '<div class="chart-wrap"><div class="chart-label">延迟趋势 · 最近 ' + (u.history||[]).length + ' 次</div><canvas class="lat"></canvas></div>' +
      '<table class="traffic-table">' +
        '<thead><tr><th>流量</th><th>↑ 上行</th><th>↓ 下行</th></tr></thead>' +
        '<tbody>' +
          '<tr><td>今日</td><td>' + fmtBytes(t.today_up) + '</td><td>' + fmtBytes(t.today_down) + '</td></tr>' +
          '<tr><td>月度周期<span class="cycle-range">' + esc(t.cycle_start || '') + ' ~ ' + esc(t.cycle_end || '') + '</span></td><td>' + fmtBytes(t.month_up) + '</td><td>' + fmtBytes(t.month_down) + '</td></tr>' +
          '<tr><td>累计</td><td>' + fmtBytes(t.total_up) + '</td><td>' + fmtBytes(t.total_down) + '</td></tr>' +
        '</tbody>' +
      '</table>';
    container.appendChild(card);
    requestAnimationFrame(function(){ drawChart(card.querySelector('canvas'), u.history); });
  });
}

var lastStatus = null;
function fetchStatus() {
  return api('GET', '/api/status').then(function(data) {
    lastStatus = data;
    var badge = document.getElementById('active-name');
    var dot = document.getElementById('active-dot');
    badge.textContent = '活跃: ' + (data.active_upstream || '无可用节点');
    dot.classList.toggle('dead', !data.active_upstream);
    document.getElementById('last-update').textContent = new Date().toLocaleTimeString();
    if (document.getElementById('panel-monitor').classList.contains('active')) renderCards(data);
  }).catch(function(e){ console.error('获取状态失败', e); });
}

// ---------- 节点管理 ----------
function reloadNodes() {
  var body = document.getElementById('nodes-body');
  body.innerHTML = '<tr><td colspan="9" class="muted">加载中...</td></tr>';
  Promise.all([api('GET','/api/upstreams'), lastStatus ? Promise.resolve(lastStatus) : api('GET','/api/status')])
    .then(function(res) {
      var rows = res[0], st = res[1];
      var statusMap = {};
      (st.upstreams||[]).forEach(function(u){ statusMap[u.id] = u; });
      if (!rows.length) {
        body.innerHTML = '<tr><td colspan="9"><div class="empty">暂无节点，点击右上角「＋ 添加节点」创建</div></td></tr>';
        return;
      }
      body.innerHTML = rows.map(function(u) {
        var s = statusMap[u.id] || {};
        var t = s.traffic || {};
        var health = !u.enabled ? '<span class="badge off">禁用</span>'
          : (s.healthy ? '<span class="badge on">健康</span>' : '<span class="badge bad">故障</span>');
        var nameAttr = esc(u.name).replace(/'/g, '&#39;');
        var bd = u.billing_day || 1;
        return '<tr>' +
          '<td class="mono">' + u.id + '</td>' +
          '<td>' + esc(u.name) + '</td>' +
          '<td class="mono">' + esc(u.addr) + '</td>' +
          '<td>' + u.priority + '</td>' +
          '<td>' + (u.username ? esc(u.username) : '<span class="muted">无</span>') + '</td>' +
          '<td>' + health + '</td>' +
          '<td>↑' + fmtBytes(t.today_up) + ' ↓' + fmtBytes(t.today_down) + '</td>' +
          '<td>每月 ' + bd + ' 日起<div class="mono" style="font-size:11px">' + (t.cycle_start ? esc(t.cycle_start) + ' ~ ' + esc(t.cycle_end) : '') + '</div></td>' +
          '<td>' +
            '<button class="btn small" onclick="editNode(' + u.id + ')">编辑</button> ' +
            '<button class="btn small danger" onclick="confirmResetTraffic(' + u.id + ',\'' + nameAttr + '\')">清全部历史</button> ' +
            '<button class="btn small danger" onclick="confirmDeleteNode(' + u.id + ',\'' + nameAttr + '\')">删除</button>' +
          '</td></tr>';
      }).join('');
    })
    .catch(function(e){ body.innerHTML = '<tr><td colspan="9" style="color:#dc2626">' + esc(e.message) + '</td></tr>'; });
}

function openNodeForm(u) {
  document.getElementById('nm-title').textContent = u ? '编辑节点 #' + u.id : '添加节点';
  document.getElementById('nf-id').value = u ? u.id : '';
  document.getElementById('nf-name').value = u ? u.name : '';
  document.getElementById('nf-addr').value = u ? u.addr : '';
  document.getElementById('nf-priority').value = u ? u.priority : 1;
  document.getElementById('nf-billing_day').value = u ? (u.billing_day || 1) : 1;
  document.getElementById('nf-username').value = u ? (u.username || '') : '';
  document.getElementById('nf-password').value = u ? (u.password || '') : '';
  document.getElementById('nf-enabled').value = (!u || u.enabled) ? '1' : '0';
  document.getElementById('node-modal').classList.add('show');
}
function closeNodeForm() { document.getElementById('node-modal').classList.remove('show'); }

function editNode(id) {
  api('GET','/api/upstreams').then(function(rows){
    var u = rows.find(function(x){ return x.id === id; });
    if (u) openNodeForm(u);
  }).catch(function(e){ flash('node-msg', e.message, true); });
}

function submitNode() {
  var id = document.getElementById('nf-id').value;
  var bd = parseInt(document.getElementById('nf-billing_day').value || '1', 10);
  if (isNaN(bd) || bd < 1 || bd > 31) { flash('node-msg', '月度周期起始日需为 1~31', true); return; }
  var payload = {
    name: document.getElementById('nf-name').value.trim(),
    addr: document.getElementById('nf-addr').value.trim(),
    username: document.getElementById('nf-username').value,
    password: document.getElementById('nf-password').value,
    priority: parseInt(document.getElementById('nf-priority').value || '1', 10),
    billing_day: bd,
    enabled: document.getElementById('nf-enabled').value === '1'
  };
  if (!payload.name || !payload.addr) { flash('node-msg', '名称和地址必填', true); return; }
  var p = id ? api('PUT', '/api/upstreams/' + id, payload) : api('POST', '/api/upstreams', payload);
  p.then(function(){
    closeNodeForm(); reloadNodes(); fetchStatus();
  }).catch(function(e){ flash('node-msg', e.message, true); });
}

// ---------- 危险操作二次确认 ----------
var confirmCb = null;
function showConfirm(title, descHTML, okText, cb) {
  document.getElementById('cm-title').textContent = title;
  document.getElementById('cm-desc').innerHTML = descHTML;
  document.getElementById('cm-ok').textContent = okText;
  confirmCb = cb;
  document.getElementById('confirm-modal').classList.add('show');
}
function closeConfirm() {
  document.getElementById('confirm-modal').classList.remove('show');
  confirmCb = null;
}
function onConfirmOk() {
  var cb = confirmCb;
  closeConfirm();
  if (cb) cb();
}

// 拉取单节点流量概况，用于确认框展示影响范围
function nodeTrafficInfo(id) {
  return api('GET', '/api/traffic?days=1').then(function(data) {
    var list = data.traffic || [];
    for (var i = 0; i < list.length; i++) if (list[i].id === id) return list[i];
    return { days: [], total_up: 0, total_down: 0, month_up: 0, month_down: 0, history_days: 0 };
  });
}

function confirmDeleteNode(id, name) {
  nodeTrafficInfo(id).then(function(nt) {
    var html = '<div class="confirm-desc">即将删除节点 <b>' + esc(name) + ' (#' + id + ')</b>，将一并清除：</div>' +
      '<div class="confirm-info">' +
        '<div><span>流量历史记录</span><b>' + (nt.history_days || 0) + ' 天</b></div>' +
        '<div><span>累计流量</span><b>↑' + fmtBytes(nt.total_up) + ' ↓' + fmtBytes(nt.total_down) + '</b></div>' +
        '<div><span>本月度周期流量</span><b>↑' + fmtBytes(nt.month_up) + ' ↓' + fmtBytes(nt.month_down) + '</b></div>' +
      '</div>' +
      '<div class="confirm-warn">删除后正在使用该节点的长连接会被立即断开，数据不可恢复。</div>';
    showConfirm('删除节点', html, '确认删除', function() {
      api('DELETE', '/api/upstreams/' + id)
        .then(function(){ reloadNodes(); fetchStatus(); })
        .catch(function(e){ flash('node-msg', e.message, true); });
    });
  });
}

function confirmResetTraffic(id, name) {
  nodeTrafficInfo(id).then(function(nt) {
    var html = '<div class="confirm-desc">即将清除节点 <b>' + esc(name) + ' (#' + id + ')</b> 的全部流量统计：</div>' +
      '<div class="confirm-info">' +
        '<div><span>每日流量记录</span><b>' + (nt.history_days || 0) + ' 天</b></div>' +
        '<div><span>累计流量</span><b>↑' + fmtBytes(nt.total_up) + ' ↓' + fmtBytes(nt.total_down) + '</b></div>' +
        '<div><span>本月度周期流量</span><b>↑' + fmtBytes(nt.month_up) + ' ↓' + fmtBytes(nt.month_down) + '</b></div>' +
      '</div>' +
      '<div class="confirm-warn">今日已产生的未落库增量也会一并清零，操作不可恢复。节点本身与探测历史不受影响。</div>';
    showConfirm('清全部历史', html, '确认清除', function() {
      api('POST', '/api/upstreams/' + id + '/reset-traffic')
        .then(function(){ reloadNodes(); reloadTraffic(); fetchStatus(); })
        .catch(function(e){ flash('node-msg', e.message, true); });
    });
  });
}

function flash(elId, msg, isErr) {
  var el = document.getElementById(elId);
  if (!el) return;
  el.textContent = msg;
  el.className = 'msg ' + (isErr ? 'err' : 'ok');
  setTimeout(function(){ el.textContent = ''; }, 4000);
}

// ---------- 流量统计 ----------
function reloadTraffic() {
  var sel = document.getElementById('traffic-days').value;
  var isCycle = sel === 'cycle';
  var body = document.getElementById('traffic-body');
  body.innerHTML = '<span class="muted">加载中...</span>';
  var url = isCycle ? '/api/traffic?period=cycle' : '/api/traffic?days=' + sel;
  api('GET', url).then(function(data) {
    var list = data.traffic || [];
    if (!list.length) { body.innerHTML = '<div class="empty">暂无节点</div>'; return; }
    body.innerHTML = list.map(function(nt) {
      var rows = (nt.days || []).slice().reverse().map(function(d) {
        return '<tr><td class="mono">' + d.date + '</td><td>' + fmtBytes(d.up) + '</td><td>' + fmtBytes(d.down) + '</td><td>' + fmtBytes(d.up + d.down) + '</td></tr>';
      }).join('');
      if (!rows) rows = '<tr><td colspan="4"><div class="empty">' + (isCycle ? '本月度周期内暂无流量记录' : '该时间范围内无流量记录') + '</div></td></tr>';
      var rangeTag = isCycle && nt.cycle_start ? '<span class="mono">' + esc(nt.cycle_start) + ' ~ ' + esc(nt.cycle_end) + '</span>' : '';
      return '<div class="tf-block">' +
        '<div class="tf-title">' + esc(nt.name) + ' <span class="mono">#' + nt.id + '</span> ' + rangeTag +
          '<span class="muted" style="font-weight:400">月度周期 ↑' + fmtBytes(nt.month_up) + ' ↓' + fmtBytes(nt.month_down) + '</span>' +
          '<span class="muted" style="font-weight:400">累计 ↑' + fmtBytes(nt.total_up) + ' ↓' + fmtBytes(nt.total_down) + '</span></div>' +
        '<div class="table-scroll"><table><thead><tr><th>日期</th><th>上行</th><th>下行</th><th>合计</th></tr></thead><tbody>' + rows + '</tbody></table></div></div>';
    }).join('');
  }).catch(function(e){ body.innerHTML = '<span style="color:#dc2626">' + esc(e.message) + '</span>'; });
}

// ---------- 设置 ----------
function loadSettings() {
  api('GET','/api/settings').then(function(s) {
    document.getElementById('st-listen').value = s.listen || '';
    document.getElementById('st-status_listen').value = s.status_listen || '';
    document.getElementById('st-check_interval').value = s.check_interval || '';
    document.getElementById('st-check_timeout').value = s.check_timeout || '';
    document.getElementById('st-fail_threshold').value = s.fail_threshold || '';
    document.getElementById('st-history_size').value = s.history_size || '';
    document.getElementById('st-test_url').value = s.test_url || '';
  }).catch(function(e){ flash('settings-msg', e.message, true); });
}

function saveSettings() {
  var payload = {
    listen: document.getElementById('st-listen').value.trim(),
    status_listen: document.getElementById('st-status_listen').value.trim(),
    check_interval: parseInt(document.getElementById('st-check_interval').value, 10),
    check_timeout: parseInt(document.getElementById('st-check_timeout').value, 10),
    fail_threshold: parseInt(document.getElementById('st-fail_threshold').value, 10),
    history_size: parseInt(document.getElementById('st-history_size').value, 10),
    test_url: document.getElementById('st-test_url').value.trim()
  };
  api('PUT','/api/settings', payload).then(function(resp) {
    var note = '已保存';
    if (resp.restart_required && resp.restart_required.length) note += '，其中 ' + resp.restart_required.join(', ') + ' 需重启生效';
    flash('settings-msg', note);
    loadSettings();
  }).catch(function(e){ flash('settings-msg', e.message, true); });
}

// ---------- 启动 ----------
fetchStatus();
setInterval(fetchStatus, 10000);
</script>
</body>
</html>`
