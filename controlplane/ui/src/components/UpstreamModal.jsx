import React, { useState, useEffect } from 'react';
import { X, Globe, Check, Plus, Trash2 } from 'lucide-react';

export default function UpstreamModal({ isOpen, onClose, upstream, onSave }) {
  if (!isOpen) return null;

  const [id, setId] = useState('');
  const [protocol, setProtocol] = useState('llm');
  const [endpoints, setEndpoints] = useState(['https://api.openai.com']);
  const [timeoutMs, setTimeoutMs] = useState(60000);
  const [maxConns, setMaxConns] = useState(100);

  useEffect(() => {
    if (upstream) {
      setId(upstream.id || '');
      setProtocol(upstream.protocol || 'llm');
      setEndpoints(upstream.endpoints?.length ? upstream.endpoints : ['https://api.openai.com']);
      setTimeoutMs(upstream.timeout_ms || 60000);
      setMaxConns(upstream.max_conns || 100);
    } else {
      setId(`upstream-${Date.now().toString().slice(-4)}`);
      setProtocol('llm');
      setEndpoints(['https://api.anthropic.com']);
      setTimeoutMs(60000);
      setMaxConns(100);
    }
  }, [upstream]);

  const handleAddEndpoint = () => {
    setEndpoints([...endpoints, '']);
  };

  const handleEndpointChange = (index, value) => {
    const updated = [...endpoints];
    updated[index] = value;
    setEndpoints(updated);
  };

  const handleRemoveEndpoint = (index) => {
    setEndpoints(endpoints.filter((_, i) => i !== index));
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    const cleanEndpoints = endpoints.map((ep) => ep.trim()).filter(Boolean);

    const updatedUpstream = {
      id: id.trim(),
      protocol: protocol,
      endpoints: cleanEndpoints,
      timeout_ms: parseInt(timeoutMs, 10) || 60000,
      max_conns: parseInt(maxConns, 10) || 100,
    };

    onSave(updatedUpstream);
    onClose();
  };

  return (
    <div className="modal-backdrop">
      <div className="modal-content glass-panel" style={{ maxWidth: '600px' }}>
        <div className="modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div className="brand-logo" style={{ width: '28px', height: '28px' }}>
              <Globe size={16} />
            </div>
            <h3 style={{ fontSize: '18px', fontWeight: 600 }}>
              {upstream ? 'Edit Upstream Destination' : 'Create Upstream Cluster'}
            </h3>
          </div>
          <button onClick={onClose} className="modal-close-btn">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '16px', marginTop: '16px' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <div>
              <label className="form-label">Cluster Identifier</label>
              <input
                type="text"
                value={id}
                onChange={(e) => setId(e.target.value)}
                placeholder="e.g. upstream-anthropic"
                required
                className="mono"
              />
            </div>

            <div>
              <label className="form-label">Protocol</label>
              <select value={protocol} onChange={(e) => setProtocol(e.target.value)}>
                <option value="llm">llm (Native LLM Provider)</option>
                <option value="http">http / REST (HTTP 1.1 / 2)</option>
                <option value="mcp">mcp (Model Context Protocol)</option>
                <option value="grpc">grpc (gRPC Upstream)</option>
                <option value="postgres">postgres (Vector DB / SQL)</option>
              </select>
            </div>
          </div>

          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <label className="form-label" style={{ marginBottom: 0 }}>Target Endpoints / Base URLs</label>
              <button type="button" onClick={handleAddEndpoint} className="btn btn-secondary" style={{ padding: '2px 8px', fontSize: '11px' }}>
                <Plus size={12} /> Add Endpoint
              </button>
            </div>

            {endpoints.map((ep, idx) => (
              <div key={idx} style={{ display: 'flex', gap: '8px', marginBottom: '6px' }}>
                <input
                  type="text"
                  placeholder="https://api.openai.com or http://10.0.0.15:8080"
                  value={ep}
                  onChange={(e) => handleEndpointChange(idx, e.target.value)}
                  required
                  className="mono"
                />
                {endpoints.length > 1 && (
                  <button
                    type="button"
                    onClick={() => handleRemoveEndpoint(idx)}
                    className="btn btn-danger"
                    style={{ padding: '4px 8px' }}
                  >
                    <Trash2 size={12} />
                  </button>
                )}
              </div>
            ))}
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <div>
              <label className="form-label">Max Connections Pool</label>
              <input
                type="number"
                value={maxConns}
                onChange={(e) => setMaxConns(e.target.value)}
                min="1"
                required
                className="mono"
              />
            </div>

            <div>
              <label className="form-label">Timeout (ms)</label>
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

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '12px' }}>
            <button type="button" onClick={onClose} className="btn btn-secondary">
              Cancel
            </button>
            <button type="submit" className="btn btn-primary">
              <Check size={16} /> Save Upstream
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
