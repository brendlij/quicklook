(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  let snapshot = null;
  const charts = new Map();

  const fmt = {
    bytes(value, rate = false) {
      if (!Number.isFinite(value)) return '—';
      const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
      let unit = 0;
      while (Math.abs(value) >= 1000 && unit < units.length - 1) { value /= 1000; unit++; }
      const precision = value >= 100 || unit === 0 ? 0 : value >= 10 ? 1 : 2;
      return `${value.toFixed(precision)} ${units[unit]}${rate ? '/s' : ''}`;
    },
    percent(value) { return Number.isFinite(value) ? `${value.toFixed(value >= 10 ? 0 : 1)}%` : '—'; },
    duration(seconds) {
      if (!Number.isFinite(seconds)) return '—';
      const days = Math.floor(seconds / 86400); seconds %= 86400;
      const hours = Math.floor(seconds / 3600); const minutes = Math.floor(seconds % 3600 / 60);
      if (days) return `${days}d ${hours}h`;
      if (hours) return `${hours}h ${minutes}m`;
      return `${minutes}m`;
    },
    date(value) {
      const date = new Date(value); return Number.isNaN(date.getTime()) ? '—' : new Intl.DateTimeFormat(undefined, {dateStyle:'medium', timeStyle:'short'}).format(date);
    }
  };

  const esc = (value) => String(value ?? '').replace(/[&<>'"]/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
  const setText = (id, value) => { const element = $(id); if (element && element.textContent !== String(value)) element.textContent = value; };

  function pathFor(values, max, width = 100, height = 40) {
    if (!values.length) return '';
    const floor = Math.max(max, 1);
    return values.map((value, index) => `${index ? 'L' : 'M'} ${(index / Math.max(values.length - 1, 1) * width).toFixed(2)} ${(height - Math.min(value / floor, 1) * (height - 2)).toFixed(2)}`).join(' ');
  }

  function renderSpark(id, values, color) {
    const element = $(id); if (!element || !values.length) return;
    element.innerHTML = `<svg viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true"><path d="${pathFor(values, 100, 100, 28)}" fill="none" stroke="${color}" stroke-width="1.5" vector-effect="non-scaling-stroke"/></svg>`;
  }

  function renderChart(id, series, options = {}) {
    const element = $(id); if (!element) return;
    const values = series.flatMap(item => item.values);
    const max = options.max || Math.max(...values, 1) * 1.12;
    const paths = series.map(item => {
      const path = pathFor(item.values, max);
      const area = `${path} L 100 40 L 0 40 Z`;
      return `<path class="area" d="${area}" fill="${item.color}"/><path class="line" d="${path}" stroke="${item.color}"/>`;
    }).join('');
    element.innerHTML = `<svg viewBox="0 0 100 40" preserveAspectRatio="none" aria-hidden="true"><line class="guide" x1="0" y1="39.5" x2="100" y2="39.5"/>${paths}<rect class="hit" x="0" y="0" width="100" height="40"/></svg>`;
    charts.set(id, {element, series, max, suffix:options.suffix || '', bytes:options.bytes});
  }

  function showTooltip(event, chart) {
    if (!chart.series[0].values.length) return;
    const rect = chart.element.getBoundingClientRect();
    const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
    const index = Math.round(ratio * (chart.series[0].values.length - 1));
    const lines = chart.series.map(item => `${item.name}: ${chart.bytes ? fmt.bytes(item.values[index], true) : item.values[index].toFixed(1) + chart.suffix}`);
    const tooltip = $('tooltip'); tooltip.textContent = lines.join(' · '); tooltip.hidden = false;
    tooltip.style.left = `${Math.min(event.clientX + 12, innerWidth - tooltip.offsetWidth - 8)}px`; tooltip.style.top = `${event.clientY - 34}px`;
  }

  document.addEventListener('pointermove', event => {
    const chartElement = event.target.closest?.('.chart'); const chart = chartElement && charts.get(chartElement.id);
    if (chart) showTooltip(event, chart); else $('tooltip').hidden = true;
  });

  function storageRows(filesystems, limit) {
    if (!filesystems.length) return '<div class="empty">No relevant filesystems reported</div>';
    return filesystems.slice(0, limit).map(fs => `<div class="storage-row"><div class="storage-name"><strong>${esc(fs.mount_point)}</strong><small>${esc(fs.device)} · ${esc(fs.type)}</small></div><div class="bar"><i style="width:${Math.min(fs.usage,100)}%"></i></div><div class="storage-value"><strong>${fmt.percent(fs.usage)}</strong>${fmt.bytes(fs.used)} / ${fmt.bytes(fs.total)}</div></div>`).join('');
  }

  function renderStorage(data) {
    $('storage-preview').innerHTML = storageRows(data.storage || [], 3);
    $('storage-full').innerHTML = (data.storage || []).map(fs => `<article class="disk-card"><header><div><h3>${esc(fs.mount_point)}</h3><span>${esc(fs.device)}</span></div><span>${esc(fs.type)}</span></header><div class="bar"><i style="width:${Math.min(fs.usage,100)}%"></i></div><div class="disk-meta"><span><strong>${fmt.bytes(fs.used)}</strong> used of ${fmt.bytes(fs.total)}</span><span>${fmt.percent(fs.usage)}</span></div></article>`).join('') || '<div class="panel empty">No relevant host filesystems are available.</div>';
    setText('disk-read', fmt.bytes(data.disk_io.read_bps, true)); setText('disk-write', fmt.bytes(data.disk_io.write_bps, true));
    setText('disk-throughput', `Read ${fmt.bytes(data.disk_io.read_bps, true)} · Write ${fmt.bytes(data.disk_io.write_bps, true)}`);
  }

  function renderDocker(data) {
    const docker = data.docker;
    setText('docker-running', docker.available ? docker.running : '—'); setText('docker-stopped', docker.available ? docker.stopped : '—');
    setText('docker-message', docker.available ? `${docker.running + docker.stopped} containers detected` : docker.error || 'Docker unavailable');
    const badge = $('docker-badge'); badge.textContent = docker.available ? 'Connected' : 'Unavailable'; badge.classList.toggle('offline', !docker.available);
    $('container-preview').innerHTML = docker.available ? docker.containers.filter(c => c.state === 'running').slice(0, 5).map(c => `<span class="mini-container"><i></i>${esc(c.name)}</span>`).join('') || '<span class="muted">No running containers</span>' : '<span class="muted">Mount the Docker socket to enable container metrics.</span>';
    $('container-table').innerHTML = docker.available ? docker.containers.map(c => `<tr data-id="${esc(c.id)}" tabindex="0"><td>${esc(c.name)}</td><td><i class="state-dot ${c.state === 'running' ? 'running' : ''}"></i>${esc(c.status || c.state)}</td><td>${fmt.percent(c.cpu)}</td><td>${fmt.bytes(c.memory)}</td><td>${fmt.bytes(c.network_rx)}</td><td>${fmt.bytes(c.network_tx)}</td><td>${c.started_at ? fmt.date(c.started_at) : '—'}</td><td>${c.restart_count}</td></tr>`).join('') || '<tr><td colspan="8" class="empty">No containers found</td></tr>' : `<tr><td colspan="8" class="empty">${esc(docker.error || 'Docker is unavailable')}</td></tr>`;
  }

  function renderNetwork(data) {
    const network = data.network; setText('net-down', fmt.bytes(network.rx_bps, true)); setText('net-up', fmt.bytes(network.tx_bps, true)); setText('network-current', `↓ ${fmt.bytes(network.rx_bps, true)} · ↑ ${fmt.bytes(network.tx_bps, true)}`);
    $('interfaces').innerHTML = (network.interfaces || []).map(item => `<article class="interface"><header><h3>${esc(item.name)}</h3><span class="link-state ${item.state === 'up' ? 'up' : ''}">● ${esc(item.state)}</span></header><div class="addresses">${[...(item.ipv4 || []), ...(item.ipv6 || [])].map(esc).join('<br>') || 'No address reported'}</div><div class="interface-traffic"><span>RX <strong>${fmt.bytes(item.rx)}</strong></span><span>TX <strong>${fmt.bytes(item.tx)}</strong></span></div></article>`).join('') || '<div class="panel empty">No relevant interfaces reported</div>';
  }

  function render(data) {
    snapshot = data; const history = data.history || [];
    setText('side-host', data.host.hostname || 'unknown host'); setText('side-uptime', fmt.duration(data.host.uptime)); $('side-status').classList.add('online');
    setText('host-detail', `${data.host.hostname || 'unknown'} · ${data.host.distribution || 'Linux'} · ${data.host.kernel || 'unknown kernel'} · ${data.host.architecture || ''}`);
    setText('cpu-value', fmt.percent(data.cpu.usage)); setText('cpu-temp', data.cpu.temperature == null ? 'No sensor' : `${data.cpu.temperature.toFixed(0)}°C`); setText('cpu-model', `${data.cpu.model} · ${data.cpu.cores} cores / ${data.cpu.threads} threads`);
    setText('memory-value', `${fmt.bytes(data.memory.used)} / ${fmt.bytes(data.memory.total)}`); setText('memory-percent', fmt.percent(data.memory.usage)); setText('memory-swap', data.memory.swap_total ? `Swap ${fmt.bytes(data.memory.swap_used)} / ${fmt.bytes(data.memory.swap_total)}` : 'No swap configured');
    setText('load-one', data.load.one.toFixed(2)); setText('load-five', data.load.five.toFixed(2)); setText('load-fifteen', data.load.fifteen.toFixed(2));
    setText('uptime-value', fmt.duration(data.host.uptime)); setText('server-time', `Sampled ${fmt.date(data.timestamp)}`);
    const cpu = history.map(p => p.cpu), memory = history.map(p => p.memory), rx = history.map(p => p.network_rx), tx = history.map(p => p.network_tx);
    renderSpark('cpu-spark', cpu, '#7195bd'); renderSpark('memory-spark', memory, '#8c83ad');
    renderChart('compute-chart', [{name:'CPU',values:cpu,color:'#7195bd'},{name:'Memory',values:memory,color:'#8c83ad'}], {max:100,suffix:'%'});
    const networkSeries = [{name:'Download',values:rx,color:'#7195bd'},{name:'Upload',values:tx,color:'#8c83ad'}]; renderChart('network-chart', networkSeries, {bytes:true}); renderChart('network-full-chart', networkSeries, {bytes:true});
    renderStorage(data); renderDocker(data); renderNetwork(data); document.body.classList.remove('loading');
  }

  function setConnection(online) {
    document.querySelector('.connection').classList.toggle('online', online); setText('connection-text', online ? 'Live' : 'Reconnecting');
  }

  function connect() {
    const events = new EventSource('/api/v1/events');
    events.addEventListener('snapshot', event => { try { render(JSON.parse(event.data)); setConnection(true); } catch (error) { console.error(error); } });
    events.onerror = () => setConnection(false);
  }

  function showView(id) {
    document.querySelectorAll('.view').forEach(view => view.classList.toggle('active', view.id === id));
    document.querySelectorAll('.nav-item').forEach(item => { const active = item.dataset.view === id; item.classList.toggle('active', active); item.toggleAttribute('aria-current', active); });
    setText('view-title', id.charAt(0).toUpperCase() + id.slice(1)); document.querySelector('.sidebar').classList.remove('open'); history.replaceState(null, '', id === 'overview' ? location.pathname : `#${id}`);
  }

  document.querySelectorAll('[data-view]').forEach(button => button.addEventListener('click', () => showView(button.dataset.view)));
  document.querySelectorAll('[data-jump]').forEach(button => button.addEventListener('click', () => showView(button.dataset.jump)));
  $('menu').addEventListener('click', () => document.querySelector('.sidebar').classList.toggle('open'));

  function openDrawer(id) {
    const item = snapshot?.docker.containers.find(container => container.id === id); if (!item) return;
    setText('drawer-name', item.name); setText('drawer-image', item.image);
    $('drawer-state').innerHTML = `<span>${esc(item.state)}</span>${item.health ? `<span>health: ${esc(item.health)}</span>` : ''}`;
    const rows = [['Container ID', item.id.slice(0,12)], ['CPU', fmt.percent(item.cpu)], ['Memory', `${fmt.bytes(item.memory)} / ${fmt.bytes(item.memory_limit)}`], ['Network', `↓ ${fmt.bytes(item.network_rx)} · ↑ ${fmt.bytes(item.network_tx)}`], ['Started', item.started_at ? fmt.date(item.started_at) : '—'], ['Restarts', item.restart_count]];
    $('drawer-details').innerHTML = rows.map(([key,value]) => `<div><dt>${esc(key)}</dt><dd>${esc(value)}</dd></div>`).join('');
    $('drawer-ports').innerHTML = (item.ports || []).map(port => `<span>${port.public_port ? `${esc(port.ip || '0.0.0.0')}:${port.public_port} → ` : ''}${port.private_port}/${esc(port.type)}</span>`).join('') || '<span>None exposed</span>';
    $('backdrop').hidden = false; $('drawer').classList.add('open'); $('drawer').setAttribute('aria-hidden','false'); $('drawer-close').focus();
  }

  function closeDrawer() { $('drawer').classList.remove('open'); $('drawer').setAttribute('aria-hidden','true'); $('backdrop').hidden = true; }
  $('container-table').addEventListener('click', event => { const row = event.target.closest('tr[data-id]'); if (row) openDrawer(row.dataset.id); });
  $('container-table').addEventListener('keydown', event => { const row = event.target.closest('tr[data-id]'); if (row && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); openDrawer(row.dataset.id); } });
  $('drawer-close').addEventListener('click', closeDrawer); $('backdrop').addEventListener('click', closeDrawer); document.addEventListener('keydown', event => { if (event.key === 'Escape') closeDrawer(); });

  const initialView = location.hash.slice(1); if (['containers','storage','network'].includes(initialView)) showView(initialView);
  connect();
})();

