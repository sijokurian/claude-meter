chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'USAGE_SYNCED' || message.type === 'SYNC_ERROR') {
    chrome.storage.local.set({
      lastSync: message,
    });
  }
});
