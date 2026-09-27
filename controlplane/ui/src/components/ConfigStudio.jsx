import React, { useState } from 'react';
import { Route, Globe, ShieldAlert, Coins, Code, Plus, Trash2, Edit3, Save } from 'lucide-react';

export default function ConfigStudio({ config, onUpdateConfig, onNavigateToPublish }) {
  const [activeSection, setActiveSection] = useState('routes');
  const [jsonText, setJsonText] = useState(JSON.stringify(config, null, 2));
  const [jsonError, setJsonError] = useState('');

  // Handle local state copies
  const routes = config?.routes || [];
  const upstreams = config?.upstreams || [];
  const policies = config?.policies || [];
  const tokenBudgets = config?.token_budgets || [];

  const handleJsonChange = (e) => {
    setJsonText(e.target.value);
    try {
      const parsed = JSON.parse(e.target.value);
      setJsonError('');
      onUpdateConfig(parsed);
    } catch (err) {
      setJsonError(err.message);
    }
  };

  const handleAddRoute = () => {
    const newRoute = {
      id: `route-${Date.now().toString().slice(-4)}`,
      path: '/v1/agent/task',
      path_prefix: false,
      method: 'POST',
      upstream_id: upstreams[0]?.id || 'upstream-llm-primary',
      policy_ids: ['policy-cel-guard'],
      timeout_ms: 45000,
    };
    const updated = {
      ...config,
      routes: [...routes, newRoute],
    };
    onUpdateConfig(updated);
    setJsonText(JSON.stringify(updated, null, 2));
  };

  const handleDeleteRoute = (id) => {
    const updated = {
      ...config,
      routes: routes.filter((r) => r.id !== id),
    };
    onUpdateConfig(updated);
    setJsonText(JSON.stringify(updated, null, 2));
  };

  const handleAddUpstream = () => {
    const newUpstream = {
      id: `upstream-${Date.now().toString().slice(-4)}`,
      protocol: 'http',
      endpoints: ['https://api.groq.com/openai'],
      timeout_ms: 30000,
      max_conns: 100,
    };
    const updated = {
      ...config,
      upstreams: [...upstreams, newUpstream],
    };
    onUpdateConfig(updated);
    setJsonText(JSON.stringify(updated, null, 2));
  };

  const handleDeleteUpstream = (id) => {
    const updated = {
      ...config,
      upstreams: upstreams.filter((u) => u.id !== id),
    };
    onUpdateConfig(updated);
    setJsonText(JSON.stringify(updated, null, 2));
  };

  return (
    <div>
      {/* Studio Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
        <div>
          <h2 style={{ fontSize: '20px', fontWeight: 700 }}>Configuration Studio</h2>
          <p style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
            Design and validate routes, upstream clusters, and zero-trust CEL policies before deployment
          </p>
        </div>

        <button onClick={onNavigateToPublish} className="btn btn-primary">
          Proceed to Rollout Publisher →
        </button>
      </div>

      {/* Sub tabs */}
      <div style={{ display: 'flex', gap: '8px', marginBottom: '18px' }}>
        <button
          onClick={() => setActiveSection('routes')}
          className={`btn ${activeSection === 'routes' ? 'btn-primary' : 'btn-secondary'}`}
        >
          <Route size={14} /> Routes ({routes.length})
        </button>
        <button
          onClick={() => setActiveSection('upstreams')}
          className={`btn ${activeSection === 'upstreams' ? 'btn-primary' : 'btn-secondary'}`}
        >
          <Globe size={14} /> Upstreams ({upstreams.length})
        </button>
        <button
          onClick={() => setActiveSection('policies')}
          className={`btn ${activeSection === 'policies' ? 'btn-primary' : 'btn-secondary'}`}
        >
          <ShieldAlert size={14} /> Policies ({policies.length})
        </button>
        <button
          onClick={() => setActiveSection('budgets')}
          className={`btn ${activeSection === 'budgets' ? 'btn-primary' : 'btn-secondary'}`}
        >
          <Coins size={14} /> Token Budgets ({tokenBudgets.length})
        </button>
        <button
          onClick={() => setActiveSection('json')}
          className={`btn ${activeSection === 'json' ? 'btn-primary' : 'btn-secondary'}`}
        >
          <Code size={14} /> Raw JSON
        </button>
      </div>

      {/* ROUTES TAB */}
      {activeSection === 'routes' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Active Ingress Routes</h3>
            <button onClick={handleAddRoute} className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: '12px' }}>
              <Plus size={14} /> Add Route
            </button>
          </div>

          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Route ID</th>
                  <th>Path & Matcher</th>
                  <th>Method</th>
                  <th>Target Upstream</th>
                  <th>Attached Policies</th>
                  <th>Timeout</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {routes.map((r) => (
                  <tr key={r.id}>
                    <td><span className="mono" style={{ fontWeight: 600, color: '#f3f4f6' }}>{r.id}</span></td>
                    <td>
                      <span className="mono" style={{ color: '#818cf8' }}>{r.path}</span>
                      {r.path_prefix && <span className="badge badge-neutral" style={{ marginLeft: '6px' }}>Prefix</span>}
                    </td>
                    <td><span className="badge badge-info">{r.method || 'ANY'}</span></td>
                    <td><span className="mono" style={{ color: '#34d399' }}>{r.upstream_id}</span></td>
                    <td>
                      <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                        {(r.policy_ids || []).map((p) => (
                          <span key={p} className="badge badge-neutral">{p}</span>
                        ))}
                      </div>
                    </td>
                    <td><span className="mono">{r.timeout_ms || 30000} ms</span></td>
                    <td>
                      <button
                        onClick={() => handleDeleteRoute(r.id)}
                        className="btn btn-danger"
                        style={{ padding: '4px 8px', fontSize: '11px' }}
                        title="Delete Route"
                      >
                        <Trash2 size={12} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* UPSTREAMS TAB */}
      {activeSection === 'upstreams' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Destination Upstream Clusters</h3>
            <button onClick={handleAddUpstream} className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: '12px' }}>
              <Plus size={14} /> Add Upstream
            </button>
          </div>

          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Cluster ID</th>
                  <th>Protocol</th>
                  <th>Endpoints</th>
                  <th>Max Connections</th>
                  <th>Timeout</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {upstreams.map((u) => (
                  <tr key={u.id}>
                    <td><span className="mono" style={{ fontWeight: 600, color: '#f3f4f6' }}>{u.id}</span></td>
                    <td><span className="badge badge-info">{u.protocol}</span></td>
                    <td>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                        {(u.endpoints || []).map((ep) => (
                          <span key={ep} className="mono" style={{ color: '#c7d2fe' }}>{ep}</span>
                        ))}
                      </div>
                    </td>
                    <td><span className="mono">{u.max_conns || 100}</span></td>
                    <td><span className="mono">{u.timeout_ms || 30000} ms</span></td>
                    <td>
                      <button
                        onClick={() => handleDeleteUpstream(u.id)}
                        className="btn btn-danger"
                        style={{ padding: '4px 8px', fontSize: '11px' }}
                      >
                        <Trash2 size={12} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* POLICIES TAB */}
      {activeSection === 'policies' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <h3 style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>Zero-Trust CEL & Guardrail Policies</h3>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Policy ID</th>
                  <th>Name</th>
                  <th>Type</th>
                  <th>Action</th>
                  <th>CEL Expression / Rules</th>
                </tr>
              </thead>
              <tbody>
                {policies.map((p) => (
                  <tr key={p.id}>
                    <td><span className="mono" style={{ fontWeight: 600, color: '#f3f4f6' }}>{p.id}</span></td>
                    <td>{p.name}</td>
                    <td><span className="badge badge-info">{p.type}</span></td>
                    <td>
                      <span className={`badge ${p.action === 'ALLOW' ? 'badge-success' : 'badge-danger'}`}>
                        {p.action}
                      </span>
                    </td>
                    <td>
                      {p.cel_expression ? (
                        <code className="mono" style={{ color: '#a5b4fc', background: 'rgba(0,0,0,0.3)', padding: '2px 6px', borderRadius: '4px' }}>
                          {p.cel_expression}
                        </code>
                      ) : (
                        <span style={{ color: 'var(--text-muted)' }}>Configured via parameters</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* TOKEN BUDGETS TAB */}
      {activeSection === 'budgets' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <h3 style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>Tenant Quotas & Token Rate Limits</h3>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Tenant ID</th>
                  <th>Model ID</th>
                  <th>Max Prompt Tokens / Min</th>
                  <th>Max Completion Tokens / Min</th>
                  <th>Daily Spend Limit</th>
                </tr>
              </thead>
              <tbody>
                {tokenBudgets.map((b, idx) => (
                  <tr key={idx}>
                    <td><span className="mono" style={{ fontWeight: 600, color: '#818cf8' }}>{b.tenant_id}</span></td>
                    <td><span className="badge badge-neutral">{b.model_id}</span></td>
                    <td><span className="mono">{b.max_prompt_tokens_per_min.toLocaleString()}</span></td>
                    <td><span className="mono">{b.max_completion_tokens_per_min.toLocaleString()}</span></td>
                    <td><span className="mono" style={{ color: '#34d399' }}>${b.max_cost_per_day.toFixed(2)}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* RAW JSON TAB */}
      {activeSection === 'json' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Declarative Schema JSON</h3>
            {jsonError && <span style={{ color: '#fb7185', fontSize: '12px' }}>Syntax Error: {jsonError}</span>}
          </div>
          <textarea
            value={jsonText}
            onChange={handleJsonChange}
            rows={22}
            className="mono"
            style={{ width: '100%', resize: 'vertical', fontSize: '12px', lineHeight: 1.4 }}
          />
        </div>
      )}
    </div>
  );
}
