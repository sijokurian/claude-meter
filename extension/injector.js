if (!window.__claudeMeterPatched) {
  window.__claudeMeterPatched = true;

  const origFetch = window.fetch;
  window.fetch = async function (...args) {
    const response = await origFetch.apply(this, args);
    try {
      const url = (typeof args[0] === 'string') ? args[0] : args[0]?.url || '';
      if (/\/api\/organizations\/[^/]+\/usage/i.test(url)) {
        const clone = response.clone();
        clone.json().then(data => {
          window.postMessage({ type: 'CLAUDE_METER_FETCH', url, data }, window.location.origin);
        }).catch(() => {});
      }
    } catch (_) {}
    return response;
  };
}
