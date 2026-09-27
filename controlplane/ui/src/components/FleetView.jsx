import React, { useState } from 'react';
import { Server, Activity, Cpu, HardDrive, Wifi, WifiOff, CheckCircle2, AlertTriangle, XCircle, Clock } from 'lucide-react';

export default function FleetView({ nodes, stats, targetVersion }) {
  const [filter, setFilter] = useState('ALL');

  const filteredNodes = nodes.filter((n) => {
    if (filter === 'ALL') return true;
    return n.health_status === filter;
  });

  const formatBytes = (bytes) => {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const formatTime = (ts) => {
    if (!ts) return 'Never';
    const date = new Date(ts);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  const getStatusBadge = (status) => {
    switch (status) {
      case 'HEALTHY':
        return <span className="badge badge-success"><CheckCircle2 size={12} /> Healthy</span>;
      case 'LAGGING':
        return <span className="badge badge-warning"><AlertTriangle size={12} /> Lagging</span>;
      case 'DEGRADED':
        return <span className="badge badge-danger"><XCircle size={12} /> Degraded</span>;
      case 'DISCONNECTED':
        return <span className="badge badge-neutral"><WifiOff size={12} /> Disconnected</span>;
      default:
        return <span className="badge badge-neutral">{status}</span>;
    }
  };

  const getAckBadge = (status, nackReason) => {
    switch (status) {
      case 'ACK':
        return <span className="badge badge-success">ACK</span>;
      case 'NACK':
        return <span className="badge badge-danger" title={nackReason}>NACK</span>;
      case 'PENDING':
        return <span className="badge badge-warning">PENDING</span>;
      default:
        return <span className="badge badge-neutral">-</span>;
    }
  };

  return (
    <div>
      {/* Fleet Top Metrics */}
      <div className="stats-grid">
        <div className="glass-panel stat-card">
          <div className="stat-label">
            <span>Fleet Gateways</span>
            <Server size={16} color="#818cf8" />
          </div>
          <div className="stat-value">{stats.total_nodes}</div>
          <div className="stat-sub">
            <span style={{ color: stats.connected_nodes > 0 ? '#34d399' : '#94a3b8' }}>
              ● {stats.connected_nodes} Active Streams
            </span>
          </div>
        </div>

        <div className="glass-panel stat-card">
          <div className="stat-label">
            <span>Health Status</span>
            <Activity size={16} color="#34d399" />
          </div>
          <div className="stat-value" style={{ color: stats.healthy_nodes === stats.total_nodes && stats.total_nodes > 0 ? '#34d399' : '#f9fafb' }}>
            {stats.healthy_nodes} / {stats.total_nodes}
          </div>
          <div className="stat-sub">
            {stats.lagging_nodes > 0 && <span style={{ color: '#fbbf24' }}>{stats.lagging_nodes} Lagging </span>}
            {stats.degraded_nodes > 0 && <span style={{ color: '#fb7185' }}>{stats.degraded_nodes} Degraded</span>}
            {stats.lagging_nodes === 0 && stats.degraded_nodes === 0 && <span style={{ color: '#94a3b8' }}>100% in sync</span>}
          </div>
        </div>

        <div className="glass-panel stat-card">
          <div className="stat-label">
            <span>Active Connections</span>
            <Wifi size={16} color="#38bdf8" />
          </div>
          <div className="stat-value">{stats.total_active_conns}</div>
          <div className="stat-sub">Across all data plane pods</div>
        </div>

        <div className="glass-panel stat-card">
          <div className="stat-label">
            <span>Total Fleet Memory</span>
            <HardDrive size={16} color="#c084fc" />
          </div>
          <div className="stat-value">{formatBytes(stats.total_memory_bytes)}</div>
          <div className="stat-sub">Allocated runtime heap</div>
        </div>
      </div>

      {/* Nodes Table Panel */}
      <div className="glass-panel" style={{ padding: '20px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <div>
            <h3 style={{ fontSize: '16px', fontWeight: 600 }}>Connected Gateway Nodes</h3>
            <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Real-time telemetry and configuration synchronization across active data plane pods
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            {['ALL', 'HEALTHY', 'LAGGING', 'DEGRADED', 'DISCONNECTED'].map((st) => (
              <button
                key={st}
                onClick={() => setFilter(st)}
                className={`btn ${filter === st ? 'btn-primary' : 'btn-secondary'}`}
                style={{ padding: '4px 10px', fontSize: '11px' }}
              >
                {st}
              </button>
            ))}
          </div>
        </div>

        {filteredNodes.length === 0 ? (
          <div className="empty-state">
            <Server size={36} className="empty-state-icon" />
            <p style={{ fontSize: '14px', fontWeight: 500, color: 'var(--text-secondary)' }}>No Gateway Nodes Detected</p>
            <p style={{ fontSize: '12px', marginTop: '6px' }}>
              Launch a <code>gateway-data</code> instance with <code>--platform-url localhost:9090</code> to connect.
            </p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Node ID & Namespace</th>
                  <th>Status</th>
                  <th>Version</th>
                  <th>Applied Config</th>
                  <th>Rollout State</th>
                  <th>Conns / Streams</th>
                  <th>Memory / CPU</th>
                  <th>Last Heartbeat</th>
                </tr>
              </thead>
              <tbody>
                {filteredNodes.map((node) => {
                  const isCurrent = node.current_config_version >= (targetVersion || 1);
                  return (
                    <tr key={node.node_id}>
                      <td>
                        <div style={{ fontWeight: 600, color: '#f9fafb' }}>{node.node_id}</div>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                          ns: <span className="mono" style={{ color: '#a5b4fc' }}>{node.namespace}</span> | {node.remote_addr}
                        </div>
                      </td>
                      <td>{getStatusBadge(node.health_status)}</td>
                      <td>
                        <span className="mono" style={{ fontSize: '12px' }}>{node.version || '0.1.0'}</span>
                      </td>
                      <td>
                        <span className={`mono badge ${isCurrent ? 'badge-success' : 'badge-warning'}`}>
                          v{node.current_config_version || 0}
                        </span>
                      </td>
                      <td>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                          {getAckBadge(node.last_ack_status, node.last_nack_reason)}
                          {node.last_nack_reason && (
                            <span style={{ fontSize: '11px', color: '#fb7185', maxWidth: '120px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={node.last_nack_reason}>
                              {node.last_nack_reason}
                            </span>
                          )}
                        </div>
                      </td>
                      <td>
                        <span className="mono">{node.active_connections}</span> conns / <span className="mono">{node.active_streams}</span> str
                      </td>
                      <td>
                        <div className="mono" style={{ fontSize: '12px' }}>{formatBytes(node.memory_allocated_bytes)}</div>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>CPU: {(node.cpu_usage_permille / 10).toFixed(1)}%</div>
                      </td>
                      <td>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '12px' }}>
                          <Clock size={12} color="#94a3b8" />
                          <span>{formatTime(node.last_heartbeat)}</span>
                        </div>
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
