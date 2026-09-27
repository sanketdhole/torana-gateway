const API_BASE = '/api';

export async function fetchStatus() {
  const res = await fetch(`${API_BASE}/status`);
  if (!res.ok) throw new Error('Failed to fetch status');
  return res.json();
}

export async function fetchNodes() {
  const res = await fetch(`${API_BASE}/nodes`);
  if (!res.ok) throw new Error('Failed to fetch nodes');
  return res.json();
}

export async function fetchConfig() {
  const res = await fetch(`${API_BASE}/config`);
  if (!res.ok) throw new Error('Failed to fetch config');
  return res.json();
}

export async function fetchConfigHistory() {
  const res = await fetch(`${API_BASE}/config/history`);
  if (!res.ok) throw new Error('Failed to fetch config history');
  return res.json();
}

export async function publishConfig(schema, comment) {
  const res = await fetch(`${API_BASE}/config/publish`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ schema, comment }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Publish failed' }));
    throw new Error(err.error || 'Publish failed');
  }
  return res.json();
}

export async function rollbackConfig(targetVersion) {
  const res = await fetch(`${API_BASE}/config/rollback`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target_version: targetVersion }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Rollback failed' }));
    throw new Error(err.error || 'Rollback failed');
  }
  return res.json();
}

export async function fetchRevocations() {
  const res = await fetch(`${API_BASE}/revocations`);
  if (!res.ok) throw new Error('Failed to fetch revocations');
  return res.json();
}

export async function addRevocation(keys, tokens, reason) {
  const res = await fetch(`${API_BASE}/revocations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keys, tokens, reason }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Revocation failed' }));
    throw new Error(err.error || 'Revocation failed');
  }
  return res.json();
}

export async function fetchUsage() {
  const res = await fetch(`${API_BASE}/usage`);
  if (!res.ok) throw new Error('Failed to fetch usage');
  return res.json();
}

export async function fetchAuditLogs(limit = 100) {
  const res = await fetch(`${API_BASE}/audit?limit=${limit}`);
  if (!res.ok) throw new Error('Failed to fetch audit logs');
  return res.json();
}

export function subscribeToEvents(onMessage, onError) {
  const es = new EventSource(`${API_BASE}/events`);
  es.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      onMessage(data);
    } catch (err) {
      console.warn('Invalid SSE event data:', e.data);
    }
  };
  es.onerror = (err) => {
    if (onError) onError(err);
  };
  return () => es.close();
}
