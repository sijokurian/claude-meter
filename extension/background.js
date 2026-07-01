chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'USAGE_SYNCED' || message.type === 'SYNC_ERROR') {
    chrome.storage.local.set({
      lastSync: message,
    });
  }
});

chrome.alarms.create('refresh-usage', { periodInMinutes: 1 });

let lastAlarmTime = Date.now();

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name !== 'refresh-usage') return;
  const now = Date.now();
  const elapsed = now - lastAlarmTime;
  lastAlarmTime = now;
  // If elapsed >> 1 minute, the system likely slept — log it
  if (elapsed > 3 * 60 * 1000) {
    console.log('[claude-meter] Detected wake from sleep, refreshing immediately');
  }
  refreshAllTabs();
});

// Refresh immediately on service worker startup (covers SW restart after sleep)
refreshAllTabs();

function refreshAllTabs() {
  chrome.tabs.query({ url: 'https://claude.ai/*' }, (tabs) => {
    for (const tab of tabs) {
      chrome.tabs.sendMessage(tab.id, { type: 'REFRESH_USAGE' }).catch(() => {});
    }
  });
}
