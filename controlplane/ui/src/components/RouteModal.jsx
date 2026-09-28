import React, { useState, useEffect } from 'react';
import { X, Route, Shield, Globe, Clock, Check, Plus, Trash2 } from 'lucide-react';

export default function RouteModal({
  isOpen,
  onClose,
  route,
  upstreams,
  policies,
  onSave,
}) {
  if (!isOpen) return null;

  const [id, setId] = useState('');
  const [path, setPath] = useState('');
  const [pathPrefix, setPathPrefix] = useState(false);
  const [method, setMethod] = useState('POST');
  const [upstreamId, setUpstreamId] = useState('');
  const [selectedPolicies, setSelectedPolicies] = useState([]);
  const [timeoutMs, setTimeoutMs] = useState(30000);
  const [headers, setHeaders] = useState([]);

  useEffect(() => {
    if (route) {
      setId(route.id || '');
      setPath(route.path || '');
      setPathPrefix(Boolean(route.path_prefix));
      setMethod(route.method || 'POST');
      setUpstreamId(route.upstream_id || (upstreams[0]?.id || ''));
      setSelectedPolicies(route.policy_ids || []);
      setTimeoutMs(route.timeout_ms || 30000);
      const hList = Object.entries(route.headers || {}).map(([key, value]) => ({ key, value }));
      setHeaders(hList);
    } else {
      setId(`route-${Date.now().toString().slice(-4)}`);
      setPath('/v1/chat/completions');
      setPathPrefix(false);
      setMethod('POST');
      setUpstreamId(upstreams[0]?.id || 'upstream-llm-primary');
      setSelectedPolicies(policies.length > 0 ? [policies[0].id] : []);
      setTimeoutMs(30000);
      setHeaders([]);
    }
  }, [route, upstreams, policies]);

  const handleTogglePolicy = (policyId) => {
    if (selectedPolicies.includes(policyId)) {
      setSelectedPolicies(selectedPolicies.filter((p) => p !== policyId));
    } else {
      setSelectedPolicies([...selectedPolicies, policyId]);
    }
  };

  const handleAddHeader = () => {
    setHeaders([...headers, { key: '', value: '' }]);
  };

  const handleHeaderChange = (index, field, value) => {
    const updated = [...headers];
    updated[index][field] = value;
    setHeaders(updated);
  };

  const handleRemoveHeader = (index) => {
    setHeaders(headers.filter((_, i) => i !== index));
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    const headerMap = {};
    headers.forEach((h) => {
      if (h.key.trim()) headerMap[h.key.trim()] = h.value.trim();
    });

    const updatedRoute = {
      id: id.trim(),
      path: path.trim(),
      path_prefix: pathPrefix,
      method: method,
      upstream_id: upstreamId,
      policy_ids: selectedPolicies,
      timeout_ms: parseInt(timeoutMs, 10) || 30000,
      headers: headerMap,
    };

    onSave(updatedRoute);
    onClose();
  };

  return (
    <div className="modal-backdrop">
      <div className="modal-content glass-panel" style={{ maxWidth: '640px' }}>
        <div className="modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div className="brand-logo" style={{ width: '28px', height: '28px' }}>
              <Route size={16} />
            </div>
            <h3 style={{ fontSize: '18px', fontWeight: 600 }}>
              {route ? 'Edit Gateway Ingress Route' : 'Create New Ingress Route'}
            </h3>
          </div>
          <button onClick={onClose} className="modal-close-btn">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '16px', marginTop: '16px' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <div>
              <label className="form-label">Route Identifier</label>
              <input
                type="text"
                value={id}
                onChange={(e) => setId(e.target.value)}
                placeholder="e.g. route-openai-llm"
                required
                className="mono"
              />
            </div>

            <div>
              <label className="form-label">HTTP Method</label>
              <select value={method} onChange={(e) => setMethod(e.target.value)}>
                <option value="POST">POST</option>
                <option value="GET">GET</option>
                <option value="PUT">PUT</option>
                <option value="DELETE">DELETE</option>
                <option value="PATCH">PATCH</option>
                <option value="*">ANY (*)</option>
              </select>
            </div>
          </div>

          <div>
            <label className="form-label">Ingress URI Path</label>
            <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
              <input
                type="text"
                value={path}
                onChange={(e) => setPath(e.target.value)}
                placeholder="/v1/chat/completions or /mcp"
                required
                className="mono"
                style={{ flex: 1 }}
              />
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', cursor: 'pointer', whiteSpace: 'nowrap' }}>
                <input
                  type="checkbox"
                  checked={pathPrefix}
                  onChange={(e) => setPathPrefix(e.target.checked)}
                  style={{ width: 'auto' }}
                />
                Match Prefix
              </label>
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <div>
              <label className="form-label">Target Upstream Cluster</label>
              <select value={upstreamId} onChange={(e) => setUpstreamId(e.target.value)} required>
                {upstreams.map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.id} ({u.protocol})
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="form-label">Request Timeout (ms)</label>
              <input
                type="number"
                value={timeoutMs}
                onChange={(e) => setTimeoutMs(e.target.value)}
                min="100"
                step="500"
                required
                className="mono"
              />
            </div>
          </div>

          <div>
            <label className="form-label">Attach Policies (Zero-Trust Guardrails & Authn)</label>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', maxHeight: '120px', overflowY: 'auto', padding: '8px', background: 'var(--bg-input)', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
              {policies.map((p) => {
                const isSelected = selectedPolicies.includes(p.id);
                return (
                  <button
                    type="button"
                    key={p.id}
                    onClick={() => handleTogglePolicy(p.id)}
                    className={`badge ${isSelected ? 'badge-success' : 'badge-neutral'}`}
                    style={{ cursor: 'pointer', padding: '6px 10px', fontSize: '12px', display: 'flex', alignItems: 'center', gap: '6px' }}
                  >
                    {isSelected && <Check size={12} />}
                    <span>{p.name || p.id}</span>
                    <span style={{ opacity: 0.6, fontSize: '10px' }}>({p.type})</span>
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <label className="form-label" style={{ marginBottom: 0 }}>Match Headers (Optional)</label>
              <button type="button" onClick={handleAddHeader} className="btn btn-secondary" style={{ padding: '2px 8px', fontSize: '11px' }}>
                <Plus size={12} /> Add Header
              </button>
            </div>
            {headers.map((h, idx) => (
              <div key={idx} style={{ display: 'flex', gap: '8px', marginBottom: '6px' }}>
                <input
                  type="text"
                  placeholder="Header Name (e.g. X-Agent-ID)"
                  value={h.key}
                  onChange={(e) => handleHeaderChange(idx, 'key', e.target.value)}
                  className="mono"
                />
                <input
                  type="text"
                  placeholder="Header Value regex/string"
                  value={h.value}
                  onChange={(e) => handleHeaderChange(idx, 'value', e.target.value)}
                  className="mono"
                />
                <button
                  type="button"
                  onClick={() => handleRemoveHeader(idx)}
                  className="btn btn-danger"
                  style={{ padding: '4px 8px' }}
                >
                  <Trash2 size={12} />
                </button>
              </div>
            ))}
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '12px' }}>
            <button type="button" onClick={onClose} className="btn btn-secondary">
              Cancel
            </button>
            <button type="submit" className="btn btn-primary">
              <Check size={16} /> Save Route
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
