import React, { useState, useEffect } from 'react';
import { X, ShieldAlert, Code, Check, Plus, Trash2, BookOpen } from 'lucide-react';

export default function PolicyModal({ isOpen, onClose, policy, onSave }) {
  if (!isOpen) return null;

  const [id, setId] = useState('');
  const [name, setName] = useState('');
  const [type, setType] = useState('cel');
  const [action, setAction] = useState('ALLOW');
  const [celExpression, setCelExpression] = useState('');
  const [params, setParams] = useState([]);

  const templates = [
    { label: 'Require Tenant Identity', expr: 'request.auth.tenant_id != ""' },
    { label: 'Admin Role Verification', expr: 'request.auth.claims["role"] == "admin"' },
    { label: 'API Key Header Guard', expr: 'request.headers["x-api-key"] != ""' },
    { label: 'Prompt Length Limit (< 8k)', expr: 'size(request.body) < 8192' },
  ];

  useEffect(() => {
    if (policy) {
      setId(policy.id || '');
      setName(policy.name || '');
      setType(policy.type || 'cel');
      setAction(policy.action || 'ALLOW');
      setCelExpression(policy.cel_expression || '');
      const pList = Object.entries(policy.parameters || {}).map(([key, value]) => ({ key, value }));
      setParams(pList);
    } else {
      setId(`policy-cel-${Date.now().toString().slice(-4)}`);
      setName('Tenant Isolation Guard');
      setType('cel');
      setAction('ALLOW');
      setCelExpression('request.auth.tenant_id != ""');
      setParams([]);
    }
  }, [policy]);

  const handleAddParam = () => {
    setParams([...params, { key: '', value: '' }]);
  };

  const handleParamChange = (index, field, value) => {
    const updated = [...params];
    updated[index][field] = value;
    setParams(updated);
  };

  const handleRemoveParam = (index) => {
    setParams(params.filter((_, i) => i !== index));
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    const paramMap = {};
    params.forEach((p) => {
      if (p.key.trim()) paramMap[p.key.trim()] = p.value.trim();
    });

    const updatedPolicy = {
      id: id.trim(),
      name: name.trim(),
      type: type,
      action: action,
      cel_expression: celExpression.trim(),
      parameters: paramMap,
    };

    onSave(updatedPolicy);
    onClose();
  };

  return (
    <div className="modal-backdrop">
      <div className="modal-content glass-panel" style={{ maxWidth: '640px' }}>
        <div className="modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div className="brand-logo" style={{ width: '28px', height: '28px', background: 'linear-gradient(135deg, #f43f5e, #a855f7)' }}>
              <ShieldAlert size={16} />
            </div>
            <h3 style={{ fontSize: '18px', fontWeight: 600 }}>
              {policy ? 'Edit Zero-Trust Policy' : 'Create Zero-Trust CEL Policy'}
            </h3>
          </div>
          <button onClick={onClose} className="modal-close-btn">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '16px', marginTop: '16px' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <div>
              <label className="form-label">Policy Identifier</label>
              <input
                type="text"
                value={id}
                onChange={(e) => setId(e.target.value)}
                placeholder="e.g. policy-cel-guard"
                required
                className="mono"
              />
            </div>

            <div>
              <label className="form-label">Policy Name</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Prompt Security Guardrail"
                required
              />
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <div>
              <label className="form-label">Policy Type</label>
              <select value={type} onChange={(e) => setType(e.target.value)}>
                <option value="cel">cel (Common Expression Language)</option>
                <option value="authn">authn (Authentication Provider)</option>
                <option value="authz">authz (Authorization Rule)</option>
                <option value="ratelimit">ratelimit (Rate Limiter)</option>
                <option value="schema">schema (JSON Schema Guard)</option>
              </select>
            </div>

            <div>
              <label className="form-label">Enforcement Action</label>
              <select value={action} onChange={(e) => setAction(e.target.value)}>
                <option value="ALLOW">ALLOW (Proceed if expression is true)</option>
                <option value="DENY">DENY (Halt request if triggered)</option>
                <option value="AUDIT">AUDIT (Log verdict but allow traffic)</option>
                <option value="MUTATE">MUTATE (Apply header/metadata transformation)</option>
              </select>
            </div>
          </div>

          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <label className="form-label" style={{ marginBottom: 0 }}>CEL Expression</label>
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '11px', color: 'var(--text-muted)' }}>
                <BookOpen size={12} /> Templates:
              </div>
            </div>

            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', marginBottom: '8px' }}>
              {templates.map((tpl, i) => (
                <button
                  type="button"
                  key={i}
                  onClick={() => setCelExpression(tpl.expr)}
                  className="badge badge-neutral"
                  style={{ cursor: 'pointer', padding: '4px 8px' }}
                >
                  {tpl.label}
                </button>
              ))}
            </div>

            <textarea
              rows={4}
              value={celExpression}
              onChange={(e) => setCelExpression(e.target.value)}
              placeholder="e.g. request.auth.tenant_id != ''"
              className="mono"
              style={{ fontSize: '12px', lineHeight: 1.4 }}
            />
          </div>

          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <label className="form-label" style={{ marginBottom: 0 }}>Policy Parameters</label>
              <button type="button" onClick={handleAddParam} className="btn btn-secondary" style={{ padding: '2px 8px', fontSize: '11px' }}>
                <Plus size={12} /> Add Parameter
              </button>
            </div>
            {params.map((p, idx) => (
              <div key={idx} style={{ display: 'flex', gap: '8px', marginBottom: '6px' }}>
                <input
                  type="text"
                  placeholder="Key (e.g. provider)"
                  value={p.key}
                  onChange={(e) => handleParamChange(idx, 'key', e.target.value)}
                  className="mono"
                />
                <input
                  type="text"
                  placeholder="Value (e.g. default-jwt)"
                  value={p.value}
                  onChange={(e) => handleParamChange(idx, 'value', e.target.value)}
                  className="mono"
                />
                <button
                  type="button"
                  onClick={() => handleRemoveParam(idx)}
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
              <Check size={16} /> Save Policy
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
