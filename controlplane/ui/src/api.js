const API_BASE = '/api';

function getAuthHeader() {
  const token = localStorage.getItem('torana_auth_token');
  if (token) {
    return { Authorization: `Bearer ${token}` };
  }
  return {};
}

async function request(url, options = {}) {
  const headers = {
    'Content-Type': 'application/json',
    ...getAuthHeader(),
    ...(options.headers || {}),
  };

  const res = await fetch(url, { ...options, headers });
  if (res.status === 401) {
    // If unauthorized, clear token and notify
    localStorage.removeItem('torana_auth_token');
    window.dispatchEvent(new CustomEvent('torana_unauthorized'));
    const err = await res.json().catch(() => ({ error: 'Unauthorized' }));
    throw new Error(err.error || 'Authentication required');
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `Request failed with status ${res.status}` }));
    throw new Error(err.error || 'Request failed');
  }

  return res.json();
}

// Authentication APIs
export async function login(username, passwordOrToken) {
  const data = await request(`${API_BASE}/auth/login`, {
    method: 'POST',
    body: JSON.stringify({
      username: username || 'admin',
      password: passwordOrToken,
      token: passwordOrToken,
    }),
  });

  if (data.token) {
    localStorage.setItem('torana_auth_token', data.token);
  }
  return data;
}

export async function logout() {
  try {
    await request(`${API_BASE}/auth/logout`, { method: 'POST' });
  } finally {
    localStorage.removeItem('torana_auth_token');
  }
}

export async function fetchMe() {
  return request(`${API_BASE}/auth/me`);
}

// Core Config and State APIs
export async function fetchStatus() {
  return request(`${API_BASE}/status`);
}

export async function fetchNodes() {
  return request(`${API_BASE}/nodes`);
}

export async function fetchConfig() {
  return request(`${API_BASE}/config`);
}

export async function fetchConfigHistory() {
  return request(`${API_BASE}/config/history`);
}

export async function publishConfig(schema, comment) {
  return request(`${API_BASE}/config/publish`, {
    method: 'POST',
    body: JSON.stringify({ schema, comment }),
  });
}

export async function rollbackConfig(targetVersion) {
  return request(`${API_BASE}/config/rollback`, {
    method: 'POST',
    body: JSON.stringify({ target_version: targetVersion }),
  });
}

// Fine-grained Route CRUD
export async function upsertRoute(route) {
  return request(`${API_BASE}/config/routes`, {
    method: 'POST',
    body: JSON.stringify(route),
  });
}

export async function deleteRoute(routeId) {
  return request(`${API_BASE}/config/routes?id=${encodeURIComponent(routeId)}`, {
    method: 'DELETE',
  });
}

// Fine-grained Policy CRUD
export async function upsertPolicy(policy) {
  return request(`${API_BASE}/config/policies`, {
    method: 'POST',
    body: JSON.stringify(policy),
  });
}

export async function deletePolicy(policyId) {
  return request(`${API_BASE}/config/policies?id=${encodeURIComponent(policyId)}`, {
    method: 'DELETE',
  });
}

// Fine-grained Upstream CRUD
export async function upsertUpstream(upstream) {
  return request(`${API_BASE}/config/upstreams`, {
    method: 'POST',
    body: JSON.stringify(upstream),
  });
}

export async function deleteUpstream(upstreamId) {
  return request(`${API_BASE}/config/upstreams?id=${encodeURIComponent(upstreamId)}`, {
    method: 'DELETE',
  });
}

// Revocations & Telemetry
export async function fetchRevocations() {
  return request(`${API_BASE}/revocations`);
}

export async function addRevocation(keys, tokens, reason) {
  return request(`${API_BASE}/revocations`, {
    method: 'POST',
    body: JSON.stringify({ keys, tokens, reason }),
  });
}

export async function fetchUsage() {
  return request(`${API_BASE}/usage`);
}

export async function fetchAuditLogs(limit = 100) {
  return request(`${API_BASE}/audit?limit=${limit}`);
}

export function subscribeToEvents(onMessage, onError) {
  const token = localStorage.getItem('torana_auth_token');
  const url = token ? `${API_BASE}/events?token=${encodeURIComponent(token)}` : `${API_BASE}/events`;
  const es = new EventSource(url);
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
