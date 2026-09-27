import React from 'react';
import { BarChart3, Zap, Database, Hash, Layers } from 'lucide-react';

export default function TelemetryView({ usageRecords }) {
  const records = usageRecords || [];

  const totalPromptTokens = records.reduce((acc, r) => acc + (r.prompt_tokens || 0), 0);
  const totalCompletionTokens = records.reduce((acc, r) => acc + (r.completion_tokens || 0), 0);
  const totalRequests = records.reduce((acc, r) => acc + (r.total_requests || 0), 0);

  return (
    <div>
      <div style={{ marginBottom: '24px' }}>
        <h2 style={{ fontSize: '20px', fontWeight: 700 }}>Telemetry & Usage Analytics</h2>
        <p style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          Aggregated, non-sensitive telemetry streamed from gateway nodes (Customer prompt/completion payloads are strictly excluded)
        </p>
      </div>

      <div className="stats-grid">
        <div className="glass-panel stat-card">
          <div className="stat-label">
            <span>Total Requests</span>
            <Hash size={16} color="#818cf8" />
          </div>
          <div className="stat-value">{totalRequests.toLocaleString()}</div>
          <div className="stat-sub">Across all routes & tenants</div>
        </div>

        <div className="glass-panel stat-card">
          <div className="stat-label">
            <span>Prompt Tokens</span>
            <Zap size={16} color="#38bdf8" />
          </div>
          <div className="stat-value">{totalPromptTokens.toLocaleString()}</div>
          <div className="stat-sub">Ingress prompt consumption</div>
        </div>

        <div className="glass-panel stat-card">
          <div className="stat-label">
            <span>Completion Tokens</span>
            <Zap size={16} color="#34d399" />
          </div>
          <div className="stat-value">{totalCompletionTokens.toLocaleString()}</div>
          <div className="stat-sub">Egress generation consumption</div>
        </div>

        <div className="glass-panel stat-card">
          <div className="stat-label">
            <span>Active Tenants</span>
            <Layers size={16} color="#c084fc" />
          </div>
          <div className="stat-value">{new Set(records.map((r) => r.tenant_id)).size}</div>
          <div className="stat-sub">Reporting traffic in window</div>
        </div>
      </div>

      <div className="glass-panel" style={{ padding: '20px' }}>
        <h3 style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>Tenant & Model Usage Breakdown</h3>

        {records.length === 0 ? (
          <div className="empty-state">
            <BarChart3 size={36} className="empty-state-icon" />
            <p style={{ fontSize: '14px', fontWeight: 500, color: 'var(--text-secondary)' }}>No Telemetry Usage Batches Received</p>
            <p style={{ fontSize: '12px', marginTop: '6px' }}>
              When gateway-data processes LLM requests, bounded usage reports are emitted over the control stream.
            </p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Tenant ID</th>
                  <th>Route ID</th>
                  <th>Target Model</th>
                  <th>Total Requests</th>
                  <th>Prompt Tokens</th>
                  <th>Completion Tokens</th>
                  <th>Avg Latency</th>
                  <th>Last Reported</th>
                </tr>
              </thead>
              <tbody>
                {records.map((r, idx) => {
                  const avgLat = r.total_requests > 0 ? (r.duration_ms_sum / r.total_requests).toFixed(1) : '0';
                  return (
                    <tr key={idx}>
                      <td><span className="mono" style={{ fontWeight: 600, color: '#f3f4f6' }}>{r.tenant_id}</span></td>
                      <td><span className="mono" style={{ color: '#818cf8' }}>{r.route_id}</span></td>
                      <td><span className="badge badge-neutral">{r.model}</span></td>
                      <td><span className="mono">{r.total_requests.toLocaleString()}</span></td>
                      <td><span className="mono" style={{ color: '#38bdf8' }}>{r.prompt_tokens.toLocaleString()}</span></td>
                      <td><span className="mono" style={{ color: '#34d399' }}>{r.completion_tokens.toLocaleString()}</span></td>
                      <td><span className="mono">{avgLat} ms</span></td>
                      <td><span style={{ fontSize: '12px' }}>{new Date(r.last_reported).toLocaleTimeString()}</span></td>
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
