import React, { useState } from 'react';
import {
  Route as RouteIcon,
  Globe,
  ShieldAlert,
  Coins,
  Code,
  Plus,
  Trash2,
  Edit3,
  Send,
  CheckCircle2,
  Search,
  Zap,
} from 'lucide-react';
import RouteModal from './RouteModal';
import PolicyModal from './PolicyModal';
import UpstreamModal from './UpstreamModal';

export default function ConfigStudio({
  config,
  onSaveRoute,
  onDeleteRoute,
  onSavePolicy,
  onDeletePolicy,
  onSaveUpstream,
  onDeleteUpstream,
  onDirectDeploy,
  isDeploying,
}) {
  const [activeSection, setActiveSection] = useState('routes');
  const [searchTerm, setSearchTerm] = useState('');

  // Modals state
  const [editingRoute, setEditingRoute] = useState(null);
  const [isRouteModalOpen, setIsRouteModalOpen] = useState(false);

  const [editingPolicy, setEditingPolicy] = useState(null);
  const [isPolicyModalOpen, setIsPolicyModalOpen] = useState(false);

  const [editingUpstream, setEditingUpstream] = useState(null);
  const [isUpstreamModalOpen, setIsUpstreamModalOpen] = useState(false);

  const [jsonText, setJsonText] = useState(JSON.stringify(config, null, 2));
  const [jsonError, setJsonError] = useState('');

  const routes = config?.routes || [];
  const upstreams = config?.upstreams || [];
  const policies = config?.policies || [];
  const tokenBudgets = config?.token_budgets || [];

  // Filter lists based on search
  const filteredRoutes = routes.filter(
    (r) =>
      r.id.toLowerCase().includes(searchTerm.toLowerCase()) ||
      r.path.toLowerCase().includes(searchTerm.toLowerCase()) ||
      r.upstream_id.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const filteredPolicies = policies.filter(
    (p) =>
      p.id.toLowerCase().includes(searchTerm.toLowerCase()) ||
      p.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      p.type.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const filteredUpstreams = upstreams.filter(
    (u) =>
      u.id.toLowerCase().includes(searchTerm.toLowerCase()) ||
      u.protocol.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (u.endpoints || []).some((ep) => ep.toLowerCase().includes(searchTerm.toLowerCase()))
  );

  const handleJsonChange = (e) => {
    setJsonText(e.target.value);
    try {
      const parsed = JSON.parse(e.target.value);
      setJsonError('');
      // Update locally
    } catch (err) {
      setJsonError(err.message);
    }
  };

  const handleJsonDeploy = () => {
    try {
      const parsed = JSON.parse(jsonText);
      onDirectDeploy(parsed, 'Direct JSON deploy from Studio');
    } catch (err) {
      alert(`Invalid JSON: ${err.message}`);
    }
  };

  return (
    <div>
      {/* Studio Top Control Header */}
      <div className="studio-header">
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <h2 style={{ fontSize: '22px', fontWeight: 700 }}>Configuration Studio</h2>
            <span className="badge badge-info">Active Version: v{config?.version || 1}</span>
          </div>
          <p style={{ fontSize: '13px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Create and edit ingress routes, upstream targets, and zero-trust CEL policies with instant Ed25519-signed fleet deployment
          </p>
        </div>

        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <button
            onClick={() => onDirectDeploy(config, 'Saved changes from Configuration Studio')}
            disabled={isDeploying}
            className="btn btn-primary"
            style={{ padding: '10px 18px', fontSize: '13px' }}
          >
            <Zap size={16} />
            <span>{isDeploying ? 'Signing & Broadcasting...' : '⚡ Save & Deploy to Fleet'}</span>
          </button>
        </div>
      </div>

      {/* Tabs & Search Filter Bar */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '18px', flexWrap: 'wrap', gap: '12px' }}>
        <div style={{ display: 'flex', gap: '8px' }}>
          <button
            onClick={() => setActiveSection('routes')}
            className={`btn ${activeSection === 'routes' ? 'btn-primary' : 'btn-secondary'}`}
          >
            <RouteIcon size={14} /> Routes ({routes.length})
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
            <ShieldAlert size={14} /> CEL Policies ({policies.length})
          </button>
          <button
            onClick={() => setActiveSection('budgets')}
            className={`btn ${activeSection === 'budgets' ? 'btn-primary' : 'btn-secondary'}`}
          >
            <Coins size={14} /> Quotas ({tokenBudgets.length})
          </button>
          <button
            onClick={() => {
              setJsonText(JSON.stringify(config, null, 2));
              setActiveSection('json');
            }}
            className={`btn ${activeSection === 'json' ? 'btn-primary' : 'btn-secondary'}`}
          >
            <Code size={14} /> Raw JSON
          </button>
        </div>

        {activeSection !== 'json' && (
          <div style={{ position: 'relative', width: '260px' }}>
            <Search size={14} style={{ position: 'absolute', left: '10px', top: '11px', color: 'var(--text-muted)' }} />
            <input
              type="text"
              placeholder="Search by ID, path, protocol..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              style={{ paddingLeft: '32px', fontSize: '12px', height: '36px' }}
            />
          </div>
        )}
      </div>

      {/* ================= ROUTES SECTION ================= */}
      {activeSection === 'routes' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <div>
              <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Active Ingress Routes</h3>
              <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                Incoming traffic is matched by path, method, and headers and dispatched to destination clusters
              </p>
            </div>
            <button
              onClick={() => {
                setEditingRoute(null);
                setIsRouteModalOpen(true);
              }}
              className="btn btn-primary"
              style={{ padding: '6px 14px', fontSize: '12px' }}
            >
              <Plus size={14} /> Create Route
            </button>
          </div>

          {filteredRoutes.length === 0 ? (
            <div className="empty-state">
              <RouteIcon size={36} className="empty-state-icon" />
              <p style={{ fontSize: '14px', fontWeight: 500, color: 'var(--text-secondary)' }}>No Matching Routes Found</p>
              <button
                onClick={() => {
                  setEditingRoute(null);
                  setIsRouteModalOpen(true);
                }}
                className="btn btn-secondary"
                style={{ marginTop: '12px' }}
              >
                Create First Route
              </button>
            </div>
          ) : (
            <div className="table-container">
              <table>
                <thead>
                  <tr>
                    <th>Route ID</th>
                    <th>Path & Matcher</th>
                    <th>Method</th>
                    <th>Target Upstream</th>
                    <th>Attached Guardrails</th>
                    <th>Timeout</th>
                    <th style={{ textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredRoutes.map((r) => (
                    <tr key={r.id}>
                      <td>
                        <span className="mono" style={{ fontWeight: 600, color: '#f3f4f6' }}>
                          {r.id}
                        </span>
                      </td>
                      <td>
                        <span className="mono" style={{ color: '#818cf8', fontWeight: 500 }}>
                          {r.path}
                        </span>
                        {r.path_prefix && (
                          <span className="badge badge-neutral" style={{ marginLeft: '6px', fontSize: '10px' }}>
                            Prefix
                          </span>
                        )}
                      </td>
                      <td>
                        <span className="badge badge-info">{r.method || 'ANY'}</span>
                      </td>
                      <td>
                        <span className="mono" style={{ color: '#34d399', fontWeight: 500 }}>
                          {r.upstream_id}
                        </span>
                      </td>
                      <td>
                        <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                          {(r.policy_ids || []).map((p) => (
                            <span key={p} className="badge badge-neutral" style={{ fontSize: '11px' }}>
                              {p}
                            </span>
                          ))}
                        </div>
                      </td>
                      <td>
                        <span className="mono">{r.timeout_ms || 30000} ms</span>
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        <div style={{ display: 'inline-flex', gap: '6px' }}>
                          <button
                            onClick={() => {
                              setEditingRoute(r);
                              setIsRouteModalOpen(true);
                            }}
                            className="btn btn-secondary"
                            style={{ padding: '5px 10px', fontSize: '11px' }}
                            title="Edit Route"
                          >
                            <Edit3 size={12} /> Edit
                          </button>
                          <button
                            onClick={() => {
                              if (window.confirm(`Are you sure you want to delete route '${r.id}'?`)) {
                                onDeleteRoute(r.id);
                              }
                            }}
                            className="btn btn-danger"
                            style={{ padding: '5px 8px', fontSize: '11px' }}
                            title="Delete Route"
                          >
                            <Trash2 size={12} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* ================= POLICIES SECTION ================= */}
      {activeSection === 'policies' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <div>
              <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Zero-Trust CEL & Guardrail Policies</h3>
              <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                Compiled Common Expression Language (CEL) expressions and authentication providers evaluated in-flight
              </p>
            </div>
            <button
              onClick={() => {
                setEditingPolicy(null);
                setIsPolicyModalOpen(true);
              }}
              className="btn btn-primary"
              style={{ padding: '6px 14px', fontSize: '12px' }}
            >
              <Plus size={14} /> Create Policy
            </button>
          </div>

          {filteredPolicies.length === 0 ? (
            <div className="empty-state">
              <ShieldAlert size={36} className="empty-state-icon" />
              <p style={{ fontSize: '14px', fontWeight: 500, color: 'var(--text-secondary)' }}>No Policies Found</p>
              <button
                onClick={() => {
                  setEditingPolicy(null);
                  setIsPolicyModalOpen(true);
                }}
                className="btn btn-secondary"
                style={{ marginTop: '12px' }}
              >
                Create First Policy
              </button>
            </div>
          ) : (
            <div className="table-container">
              <table>
                <thead>
                  <tr>
                    <th>Policy ID</th>
                    <th>Policy Name</th>
                    <th>Type</th>
                    <th>Action</th>
                    <th>CEL Expression / Rules</th>
                    <th style={{ textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredPolicies.map((p) => (
                    <tr key={p.id}>
                      <td>
                        <span className="mono" style={{ fontWeight: 600, color: '#f3f4f6' }}>
                          {p.id}
                        </span>
                      </td>
                      <td style={{ fontWeight: 500, color: '#f9fafb' }}>{p.name}</td>
                      <td>
                        <span className="badge badge-info">{p.type}</span>
                      </td>
                      <td>
                        <span className={`badge ${p.action === 'ALLOW' ? 'badge-success' : 'badge-danger'}`}>
                          {p.action}
                        </span>
                      </td>
                      <td style={{ maxWidth: '380px' }}>
                        {p.cel_expression ? (
                          <code className="mono" style={{ color: '#a5b4fc', background: 'rgba(0,0,0,0.3)', padding: '3px 8px', borderRadius: '4px', display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={p.cel_expression}>
                            {p.cel_expression}
                          </code>
                        ) : (
                          <span style={{ color: 'var(--text-muted)', fontSize: '12px' }}>Configured via parameters</span>
                        )}
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        <div style={{ display: 'inline-flex', gap: '6px' }}>
                          <button
                            onClick={() => {
                              setEditingPolicy(p);
                              setIsPolicyModalOpen(true);
                            }}
                            className="btn btn-secondary"
                            style={{ padding: '5px 10px', fontSize: '11px' }}
                            title="Edit Policy"
                          >
                            <Edit3 size={12} /> Edit
                          </button>
                          <button
                            onClick={() => {
                              if (window.confirm(`Are you sure you want to delete policy '${p.id}'?`)) {
                                onDeletePolicy(p.id);
                              }
                            }}
                            className="btn btn-danger"
                            style={{ padding: '5px 8px', fontSize: '11px' }}
                            title="Delete Policy"
                          >
                            <Trash2 size={12} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* ================= UPSTREAMS SECTION ================= */}
      {activeSection === 'upstreams' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <div>
              <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Destination Upstream Clusters</h3>
              <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                Target model provider APIs, remote MCP tool servers, vector databases, and internal microservices
              </p>
            </div>
            <button
              onClick={() => {
                setEditingUpstream(null);
                setIsUpstreamModalOpen(true);
              }}
              className="btn btn-primary"
              style={{ padding: '6px 14px', fontSize: '12px' }}
            >
              <Plus size={14} /> Create Upstream
            </button>
          </div>

          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Cluster ID</th>
                  <th>Protocol</th>
                  <th>Endpoints</th>
                  <th>Connection Pool</th>
                  <th>Timeout</th>
                  <th style={{ textAlign: 'right' }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filteredUpstreams.map((u) => (
                  <tr key={u.id}>
                    <td>
                      <span className="mono" style={{ fontWeight: 600, color: '#f3f4f6' }}>
                        {u.id}
                      </span>
                    </td>
                    <td>
                      <span className="badge badge-info">{u.protocol}</span>
                    </td>
                    <td>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
                        {(u.endpoints || []).map((ep) => (
                          <span key={ep} className="mono" style={{ color: '#c7d2fe', fontSize: '12px' }}>
                            {ep}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td>
                      <span className="mono">{u.max_conns || 100} conns</span>
                    </td>
                    <td>
                      <span className="mono">{u.timeout_ms || 30000} ms</span>
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <div style={{ display: 'inline-flex', gap: '6px' }}>
                        <button
                          onClick={() => {
                            setEditingUpstream(u);
                            setIsUpstreamModalOpen(true);
                          }}
                          className="btn btn-secondary"
                          style={{ padding: '5px 10px', fontSize: '11px' }}
                        >
                          <Edit3 size={12} /> Edit
                        </button>
                        <button
                          onClick={() => {
                            if (window.confirm(`Are you sure you want to delete upstream '${u.id}'?`)) {
                              onDeleteUpstream(u.id);
                            }
                          }}
                          className="btn btn-danger"
                          style={{ padding: '5px 8px', fontSize: '11px' }}
                        >
                          <Trash2 size={12} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* ================= TOKEN BUDGETS SECTION ================= */}
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
                    <td>
                      <span className="mono" style={{ fontWeight: 600, color: '#818cf8' }}>
                        {b.tenant_id}
                      </span>
                    </td>
                    <td>
                      <span className="badge badge-neutral">{b.model_id}</span>
                    </td>
                    <td>
                      <span className="mono">{(b.max_prompt_tokens_per_min || 0).toLocaleString()}</span>
                    </td>
                    <td>
                      <span className="mono">{(b.max_completion_tokens_per_min || 0).toLocaleString()}</span>
                    </td>
                    <td>
                      <span className="mono" style={{ color: '#34d399', fontWeight: 600 }}>
                        ${(b.max_cost_per_day || 0).toFixed(2)}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* ================= RAW JSON TAB ================= */}
      {activeSection === 'json' && (
        <div className="glass-panel" style={{ padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <div>
              <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Declarative Schema JSON</h3>
              <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                Directly edit the complete state definition and broadcast to all connected gateways
              </p>
            </div>
            <button onClick={handleJsonDeploy} className="btn btn-primary" style={{ padding: '6px 14px', fontSize: '12px' }}>
              <Send size={14} /> Deploy JSON Definition
            </button>
          </div>
          {jsonError && (
            <div style={{ color: '#fb7185', background: 'rgba(244,63,94,0.1)', padding: '8px 12px', borderRadius: '6px', fontSize: '12px', marginBottom: '10px' }}>
              JSON Syntax Error: {jsonError}
            </div>
          )}
          <textarea
            value={jsonText}
            onChange={handleJsonChange}
            rows={22}
            className="mono"
            style={{ width: '100%', resize: 'vertical', fontSize: '12px', lineHeight: 1.4 }}
          />
        </div>
      )}

      {/* Route Modal */}
      <RouteModal
        isOpen={isRouteModalOpen}
        onClose={() => setIsRouteModalOpen(false)}
        route={editingRoute}
        upstreams={upstreams}
        policies={policies}
        onSave={(updated) => onSaveRoute(updated)}
      />

      {/* Policy Modal */}
      <PolicyModal
        isOpen={isPolicyModalOpen}
        onClose={() => setIsPolicyModalOpen(false)}
        policy={editingPolicy}
        onSave={(updated) => onSavePolicy(updated)}
      />

      {/* Upstream Modal */}
      <UpstreamModal
        isOpen={isUpstreamModalOpen}
        onClose={() => setIsUpstreamModalOpen(false)}
        upstream={editingUpstream}
        onSave={(updated) => onSaveUpstream(updated)}
      />
    </div>
  );
}
