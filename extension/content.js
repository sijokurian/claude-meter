/**
 * Content script injected into claude.ai pages.
 *
 * Two strategies run in parallel:
 *   1. Fetch interception — a script injected into the page's main world
 *      patches window.fetch and posts matching API responses back to this
 *      content script via window.postMessage.
 *   2. DOM scanning — looks for the rate-limit usage bar specifically.
 *
 * Extracted data is forwarded to the tray app via localhost HTTP.
 */

const TRAY_PORT = 52413;
const TRAY_URL = `http://127.0.0.1:${TRAY_PORT}/api/web-usage`;
const DEBUG = true;

function dbg(...args) {
  if (DEBUG) console.log('[claude-meter]', ...args);
}

// ── Listen for intercepted API data (injector.js runs in MAIN world) ──

window.addEventListener('message', (event) => {
  if (event.source !== window) return;
  if (event.data?.type !== 'CLAUDE_METER_FETCH') return;
  dbg('API intercepted:', event.data.url, event.data.data);
  processApiResponse(event.data.url, event.data.data);
});

function processApiResponse(url, data) {
  const usage = extractUsageFromApi(data);
  if (usage !== null) {
    dbg('Extracted from API:', usage.percentage, '%');
    sendToTray({ source: 'api', ...usage });
  }
}

function extractUsageFromApi(data) {
  if (!data || typeof data !== 'object') return null;

  // Direct percentage fields (rate-limit specific)
  for (const key of ['usage_percentage', 'percent_used', 'rate_limit_percentage',
                      'usage_pct', 'pct_used', 'percentage_used']) {
    if (typeof data[key] === 'number') {
      return { percentage: data[key], raw: data };
    }
  }

  // Look for a "message_limit" style object: { remaining: N, limit: N }
  if (typeof data.remaining === 'number' && typeof data.limit === 'number' && data.limit > 0) {
    return { percentage: ((data.limit - data.remaining) / data.limit) * 100, raw: data };
  }

  // Nested under rate-limit-specific keys only (NOT "usage" — that's per-message tokens)
  for (const key of ['rate_limit', 'billing', 'subscription', 'message_limit',
                      'quota', 'entitlement']) {
    const nested = data[key];
    if (nested && typeof nested === 'object' && !Array.isArray(nested)) {
      const result = extractUsageFromApi(nested);
      if (result) return result;
    }
  }

  return null;
}

// ── DOM scanning ────────────────────────────────────────────────────

const SCAN_INTERVAL_MS = 10_000;

function scanDom() {
  if (!document.body) return;

  // Strategy 1 (preferred): Text percentage — more precise than progress bar.
  // Match "X%" near usage-related keywords.
  const walker = document.createTreeWalker(
    document.body,
    NodeFilter.SHOW_TEXT,
    { acceptNode: (n) => n.textContent.includes('%') ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT }
  );

  let node;
  while ((node = walker.nextNode())) {
    const text = node.textContent.trim();
    const match = text.match(/([\d.]+)\s*%/);
    if (!match) continue;

    let context = text;
    let el = node.parentElement;
    for (let i = 0; i < 3 && el; i++) {
      context += ' ' + (el.textContent || '').slice(0, 200);
      el = el.parentElement;
    }

    if (!/usage|limit|rate|quota|allowance|your.*(message|conversation)/i.test(context)) continue;
    if (/```|token|price|\$|cache|input_token|output_token/i.test(context)) continue;

    let pct = parseFloat(match[1]);
    if (/remaining/i.test(context)) pct = 100 - pct;

    if (!isNaN(pct) && pct >= 0 && pct <= 100) {
      dbg('DOM text match:', pct, '% from:', text.slice(0, 60));
      sendToTray({ source: 'dom-text', percentage: pct });
      return;
    }
  }

  // Strategy 2 (fallback): Progress bar aria-valuenow (often rounded).
  const progressBars = document.querySelectorAll('[role="progressbar"]');
  for (const bar of progressBars) {
    const label = (bar.getAttribute('aria-label') || '').toLowerCase();
    const labelledBy = bar.getAttribute('aria-labelledby');
    let contextText = label;
    if (labelledBy) {
      const labelEl = document.getElementById(labelledBy);
      if (labelEl) contextText += ' ' + labelEl.textContent.toLowerCase();
    }
    const parent = bar.closest('[class*="usage"], [class*="limit"], [class*="rate"]');
    if (parent) contextText += ' ' + parent.textContent.toLowerCase();

    if (!/usage|limit|rate|quota|allowance|message/i.test(contextText)) continue;

    const ariaVal = bar.getAttribute('aria-valuenow');
    if (ariaVal) {
      const pct = parseFloat(ariaVal);
      if (!isNaN(pct) && pct >= 0 && pct <= 100) {
        dbg('DOM progressbar:', pct, '% context:', contextText.slice(0, 80));
        sendToTray({ source: 'dom-progressbar', percentage: pct });
        return;
      }
    }

    const inner = bar.querySelector('[style*="width"]');
    if (inner) {
      const widthMatch = inner.style.width.match(/([\d.]+)%/);
      if (widthMatch) {
        const pct = parseFloat(widthMatch[1]);
        if (!isNaN(pct) && pct >= 0 && pct <= 100) {
          dbg('DOM inner-bar width:', pct, '%');
          sendToTray({ source: 'dom-bar', percentage: pct });
          return;
        }
      }
    }
  }
}

setInterval(scanDom, SCAN_INTERVAL_MS);
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => setTimeout(scanDom, 3000));
} else {
  setTimeout(scanDom, 3000);
}

// ── Send data to tray app ───────────────────────────────────────────

let lastSent = 0;
let lastPct = null;
const MIN_SEND_INTERVAL_MS = 5_000;

async function sendToTray(usage) {
  const now = Date.now();
  // Debounce, but always send if value changed
  if (now - lastSent < MIN_SEND_INTERVAL_MS && usage.percentage === lastPct) return;
  lastSent = now;
  lastPct = usage.percentage;

  const payload = {
    percentage: usage.percentage,
    source: usage.source || 'unknown',
    timestamp: new Date().toISOString(),
    raw: usage.raw || null,
  };

  dbg('Sending to tray:', payload.percentage, '% via', payload.source);

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
