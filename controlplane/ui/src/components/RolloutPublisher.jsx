import React, { useState } from 'react';
import { Send, CheckCircle2, RotateCcw, ShieldCheck, History, AlertTriangle, Cpu } from 'lucide-react';

export default function RolloutPublisher({
  config,
  nodes,
  history,
  onPublish,
  onRollback,
  isPublishing,
  lastPublishResult,
}) {
  const [comment, setComment] = useState('');
  const [rollbackVer, setRollbackVer] = useState(null);

  const currentVer = config?.version || 1;
  const targetVer = lastPublishResult?.config_version || currentVer;

  // Calculate rollout percentage
  const totalNodes = nodes.length;
  const ackedNodes = nodes.filter((n) => n.current_config_version >= targetVer && n.last_ack_status === 'ACK').length;
  const rolloutPercentage = totalNodes > 0 ? Math.round((ackedNodes / totalNodes) * 100) : 100;

  const handlePublishSubmit = (e) => {
    e.preventDefault();
    onPublish(comment || 'Standard production rollout');
    setComment('');
  };

  const handleRollbackSubmit = (v) => {
    if (window.confirm(`Are you sure you want to rollback to snapshot version v${v}? This will immediately push to all connected gateways.`)) {
      onRollback(v);
    }
  };

  return (
    <div>
      <div style={{ marginBottom: '24px' }}>
        <h2 style={{ fontSize: '20px', fontWeight: 700 }}>Rollout Publisher & Signature Authority</h2>
        <p style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          Cryptographically sign configuration snapshots using Ed25519 and atomically broadcast to the data plane fleet
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px', marginBottom: '24px' }}>
        {/* Publish Card */}
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
            <ShieldCheck size={20} color="#818cf8" />
            <h3 style={{ fontSize: '16px', fontWeight: 600 }}>Publish Next Snapshot</h3>
          </div>

          <div style={{ marginBottom: '16px', fontSize: '13px', color: 'var(--text-secondary)' }}>
            <div>Target Version: <span className="mono badge badge-info">v{currentVer + 1}</span></div>
            <div style={{ marginTop: '4px' }}>
              Contains: <strong style={{ color: '#fff' }}>{config?.routes?.length || 0}</strong> routes,{' '}
              <strong style={{ color: '#fff' }}>{config?.upstreams?.length || 0}</strong> upstreams,{' '}
              <strong style={{ color: '#fff' }}>{config?.policies?.length || 0}</strong> policies
            </div>
          </div>

          <form onSubmit={handlePublishSubmit}>
            <div style={{ marginBottom: '16px' }}>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                Release Description / Change Note:
              </label>
              <input
                type="text"
                placeholder="e.g. Added vector DB egress and tightened token budget"
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                required
              />
            </div>

            <button
              type="submit"
              disabled={isPublishing}
              className="btn btn-primary"
              style={{ width: '100%', padding: '10px 16px', fontSize: '14px' }}
            >
              <Send size={16} />
              {isPublishing ? 'Signing & Broadcasting...' : 'Sign with Ed25519 & Push to Fleet'}
            </button>
          </form>

          {lastPublishResult && (
            <div style={{ marginTop: '16px', padding: '12px', background: 'rgba(16, 185, 129, 0.1)', border: '1px solid rgba(16, 185, 129, 0.25)', borderRadius: '8px', fontSize: '12px', color: '#34d399' }}>
              ✓ Published v{lastPublishResult.config_version} ({lastPublishResult.signature_len} bytes signature) to {lastPublishResult.nodes_notified} node streams.
            </div>
          )}
        </div>

        {/* Live Rollout Progress */}
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '16px', fontWeight: 600 }}>Fleet Convergence</h3>
            <span className="mono badge badge-info">Target: v{targetVer}</span>
          </div>

          <div style={{ marginBottom: '12px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', color: 'var(--text-muted)', marginBottom: '6px' }}>
              <span>Atomic Application (ACKed)</span>
              <span className="mono" style={{ fontWeight: 600, color: '#f3f4f6' }}>{rolloutPercentage}%</span>
            </div>
            <div style={{ width: '100%', height: '8px', background: 'rgba(255,255,255,0.06)', borderRadius: '4px', overflow: 'hidden' }}>
              <div
                style={{
                  width: `${rolloutPercentage}%`,
                  height: '100%',
                  background: 'linear-gradient(90deg, #6366f1, #10b981)',
                  transition: 'width 0.4s ease',
                }}
              />
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginTop: '20px' }}>
            <div className="glass-panel" style={{ padding: '12px', textAlign: 'center' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Nodes Synced</div>
              <div style={{ fontSize: '20px', fontWeight: 700, color: '#34d399' }}>{ackedNodes} / {totalNodes}</div>
            </div>
            <div className="glass-panel" style={{ padding: '12px', textAlign: 'center' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Rollout Status</div>
              <div style={{ fontSize: '14px', fontWeight: 700, marginTop: '4px', color: rolloutPercentage === 100 ? '#34d399' : '#fbbf24' }}>
                {rolloutPercentage === 100 ? 'CONVERGED' : 'IN PROGRESS'}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Snapshot Version History & Rollback */}
      <div className="glass-panel" style={{ padding: '20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '16px' }}>
          <History size={18} color="#818cf8" />
          <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Snapshot Version History & Instant Rollback</h3>
        </div>

        {(!history || history.length === 0) ? (
          <p style={{ fontSize: '13px', color: 'var(--text-muted)' }}>No historical snapshots recorded yet.</p>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Created At</th>
                  <th>Author</th>
                  <th>Change Comment</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {history.slice().reverse().map((h) => {
                  const isCurrent = h.version === currentVer;
                  return (
                    <tr key={h.version}>
                      <td>
                        <span className={`mono badge ${isCurrent ? 'badge-success' : 'badge-neutral'}`}>
                          v{h.version}
                        </span>
                      </td>
                      <td><span className="mono" style={{ fontSize: '12px' }}>{new Date(h.timestamp).toLocaleString()}</span></td>
                      <td><span style={{ fontSize: '12px' }}>{h.author || 'admin'}</span></td>
                      <td><span style={{ color: '#f3f4f6' }}>{h.comment}</span></td>
                      <td>
                        {!isCurrent && (
                          <button
                            onClick={() => handleRollbackSubmit(h.version)}
                            className="btn btn-secondary"
                            style={{ padding: '4px 10px', fontSize: '11px' }}
                            title="Restore this version"
                          >
                            <RotateCcw size={12} /> Rollback
                          </button>
                        )}
                        {isCurrent && (
                          <span style={{ fontSize: '11px', color: '#34d399', fontWeight: 600 }}>Active</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
