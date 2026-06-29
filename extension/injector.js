if (!window.__claudeMeterPatched) {
  window.__claudeMeterPatched = true;

  const origFetch = window.fetch;
  window.fetch = async function (...args) {
    const response = await origFetch.apply(this, args);
    try {
      const url = (typeof args[0] === 'string') ? args[0] : args[0]?.url || '';
      // Log all API calls to discover the right endpoint
      if (/\/api\//i.test(url)) {
        const clone = response.clone();
        clone.json().then(data => {
          console.log('[claude-meter] API:', url, JSON.stringify(data).slice(0, 300));
          window.postMessage({ type: 'CLAUDE_METER_FETCH', url, data }, '*');
        }).catch(() => {});
      }
    } catch (_) {}
    return response;
  };
}
