(() => {
  const byId = (id) => document.getElementById(id);
  const directBase = `http://${window.location.hostname}:24871`;
  const healthPath = '/releasestation/api/v1/system/health';

  byId('workspace-link').href = `${directBase}/releasestation/`;
  byId('open-app').addEventListener('click', () => window.open(`${directBase}/releasestation/`, '_blank'));

  async function loadHealth() {
    const dot = byId('status-dot');
    const title = byId('status-title');
    const detail = byId('status-detail');
    dot.className = 'status-dot checking';
    title.textContent = 'Checking package health';
    detail.textContent = 'Connecting to the local ReleaseStation service…';

    try {
      const response = await fetch(healthPath, { headers: { Accept: 'application/json' } });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const payload = await response.json();
      const data = payload.data || {};
      dot.className = 'status-dot healthy';
      title.textContent = 'ReleaseStation is healthy';
      detail.textContent = 'The local API and SQLite database are ready.';
      byId('version').textContent = data.version || '—';
      byId('platform').textContent = data.platform || '—';
      byId('architecture').textContent = data.architecture || '—';
      byId('database').textContent = data.database || '—';
    } catch (error) {
      dot.className = 'status-dot offline';
      title.textContent = 'ReleaseStation is unavailable';
      detail.textContent = 'Start the package or open the full workspace to inspect logs.';
      byId('database').textContent = 'Unavailable';
    }
  }

  byId('refresh').addEventListener('click', loadHealth);
  loadHealth();
})();
