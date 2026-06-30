/**
 * Content script injected into claude.ai pages.
 *
 * Listens for intercepted API responses from injector.js (which runs in the
 * page's main world and patches window.fetch). Extracts usage data from the
 * /api/organizations/{org}/usage endpoint and forwards it to the desktop tray app.
 */

const TRAY_PORT = 52413;
const TRAY_URL = `http://127.0.0.1:${TRAY_PORT}/api/web-usage`;
const DEBUG = false;

let lastUsageUrl = '';

function dbg(...args) {
  if (DEBUG) console.log('[claude-meter]', ...args);
}

window.addEventListener('message', (event) => {
  if (event.source !== window) return;
  if (event.data?.type !== 'CLAUDE_METER_FETCH') return;
  dbg('API intercepted:', event.data.url);
  if (event.data.data?.limits) {
    lastUsageUrl = event.data.url;
  }
  processUsageResponse(event.data.data);
});

chrome.runtime.onMessage.addListener((msg) => {
  if (msg.type === 'REFRESH_USAGE') {
    fetchUsageDirect();
  }
});

async function fetchUsageDirect() {
  if (!lastUsageUrl) return;
  dbg('Background fetch:', lastUsageUrl);
  try {
    const resp = await fetch(lastUsageUrl, { credentials: 'include' });
    if (resp.ok) {
      const data = await resp.json();
      processUsageResponse(data);
    }
  } catch (e) {
    dbg('Background fetch failed:', e.message);
  }
}

function processUsageResponse(data) {
  if (!data || !Array.isArray(data.limits)) return;

  const sections = [];
  for (const limit of data.limits) {
    if (typeof limit.percent !== 'number') continue;
    const resetsAt = formatResetTime(limit.resets_at);
    let label = limit.kind || 'unknown';
    if (limit.scope?.model?.display_name) {
      label = limit.scope.model.display_name;
    }
    sections.push({
      label: label,
      percentage: limit.percent,
      resets_at: resetsAt,
      type: limit.kind || 'unknown',
    });
  }

  if (sections.length === 0) return;

  const session = sections.find(s => s.type === 'session');
  const weekly = sections.find(s => s.type === 'weekly_all');
  const best = session || weekly || sections[0];

  dbg('Usage sections:', JSON.stringify(sections));
  dbg('Best:', best.percentage, '%');

  sendToTray({
    percentage: best.percentage,
    sections: sections,
  });
}

function formatResetTime(isoString) {
  if (!isoString) return '';
  try {
    const reset = new Date(isoString);
    const now = new Date();
    const diffMs = reset - now;
    if (diffMs <= 0) return '';

    const totalMin = Math.floor(diffMs / 60000);
    const hours = Math.floor(totalMin / 60);
    const mins = totalMin % 60;

    if (hours > 24) {
      const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
      const h = reset.getHours();
      const m = reset.getMinutes();
      const ampm = h >= 12 ? 'PM' : 'AM';
      const h12 = h % 12 || 12;
      const mStr = m > 0 ? `:${m.toString().padStart(2, '0')}` : '';
      return `Resets ${days[reset.getDay()]} ${h12}${mStr} ${ampm}`;
    }

    if (hours > 0) {
      return `Resets in ${hours} hr ${mins} min`;
    }
    return `Resets in ${mins} min`;
  } catch (_) {
    return '';
  }
}

let lastSent = 0;
let lastPayloadStr = '';
const MIN_SEND_INTERVAL_MS = 5_000;

async function sendToTray(usage) {
  const now = Date.now();

  const payload = {
    percentage: usage.percentage,
    source: 'api',
    timestamp: new Date().toISOString(),
    sections: usage.sections || [],
  };

  const payloadStr = JSON.stringify({ p: payload.percentage, s: payload.sections });
  if (now - lastSent < MIN_SEND_INTERVAL_MS && payloadStr === lastPayloadStr) return;
  lastSent = now;
  lastPayloadStr = payloadStr;

  dbg('Sending to tray:', JSON.stringify(payload));

  try {
    await fetch(TRAY_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    chrome.runtime.sendMessage({ type: 'USAGE_SYNCED', ...payload }).catch(() => {});
  } catch (err) {
    dbg('Send failed:', err.message);
    chrome.runtime.sendMessage({ type: 'SYNC_ERROR', error: err.message }).catch(() => {});
  }
}
