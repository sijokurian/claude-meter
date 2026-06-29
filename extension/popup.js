const dot = document.getElementById('dot');
const statusText = document.getElementById('statusText');
const usageSection = document.getElementById('usageSection');
const pctValue = document.getElementById('pctValue');
const sourceValue = document.getElementById('sourceValue');
const timeValue = document.getElementById('timeValue');
const portInput = document.getElementById('portInput');
const savePortBtn = document.getElementById('savePort');

async function checkTray(port) {
  try {
    const r = await fetch(`http://127.0.0.1:${port}/api/status`, {
      signal: AbortSignal.timeout(2000),
    });
    if (r.ok) {
      const data = await r.json();
      dot.className = 'dot ok';
      statusText.textContent = 'Connected to tray app';
      if (data.web_pct !== undefined && data.web_pct !== null) {
        usageSection.style.display = '';
        pctValue.textContent = `${data.web_pct.toFixed(1)}%`;
        sourceValue.textContent = data.web_source || '—';
        timeValue.textContent = data.web_last_update
          ? new Date(data.web_last_update).toLocaleTimeString()
          : '—';
      }
    } else {
      throw new Error('bad status');
    }
  } catch {
    dot.className = 'dot err';
    statusText.textContent = 'Tray app not reachable';
  }
}

chrome.storage.local.get(['port', 'lastSync'], (items) => {
  const port = items.port || 52413;
  portInput.value = port;
  checkTray(port);

  if (items.lastSync?.type === 'USAGE_SYNCED') {
    usageSection.style.display = '';
    pctValue.textContent = `${items.lastSync.percentage.toFixed(1)}%`;
    sourceValue.textContent = items.lastSync.source || '—';
    timeValue.textContent = items.lastSync.timestamp
      ? new Date(items.lastSync.timestamp).toLocaleTimeString()
      : '—';
  }
});

savePortBtn.addEventListener('click', () => {
  const port = parseInt(portInput.value, 10);
  if (port >= 1024 && port <= 65535) {
    chrome.storage.local.set({ port });
    checkTray(port);
  }
});
