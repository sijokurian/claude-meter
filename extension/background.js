chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'USAGE_SYNCED' || message.type === 'SYNC_ERROR') {
    chrome.storage.local.set({
      lastSync: message,
    });
  }
});

chrome.alarms.create('refresh-usage', { periodInMinutes: 1 });

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name !== 'refresh-usage') return;
  chrome.tabs.query({ url: 'https://claude.ai/*' }, (tabs) => {
    for (const tab of tabs) {
      chrome.tabs.sendMessage(tab.id, { type: 'REFRESH_USAGE' }).catch(() => {});
    }
  });
});
