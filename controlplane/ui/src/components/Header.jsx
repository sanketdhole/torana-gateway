import React from 'react';
import { Shield, RefreshCw, Radio, Key, Server, Cpu } from 'lucide-react';

export default function Header({ status, onRefresh, isRefreshing, sseConnected }) {
  const pubKeyShort = status?.public_key_hex
    ? `${status.public_key_hex.substring(0, 10)}...${status.public_key_hex.substring(status.public_key_hex.length - 8)}`
    : 'Generating...';

  return (
    <header className="app-header">
      <div className="brand">
        <div className="brand-logo">
          <Shield size={20} />
        </div>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span className="brand-title">Torana Enterprise</span>
            <span className="brand-tag">Control Plane</span>
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
            Zero-Trust Gateway Fleet Manager & Dynamic Config Distributor
          </div>
        </div>
      </div>

      <div className="header-meta">
        <div className="glass-panel" style={{ padding: '6px 12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Key size={14} color="#818cf8" />
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Ed25519 PubKey:</span>
          <span className="mono" style={{ fontSize: '11px', color: '#c7d2fe' }} title={status?.public_key_hex}>
            {pubKeyShort}
          </span>
        </div>

        <div className="glass-panel" style={{ padding: '6px 12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Server size={14} color="#34d399" />
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Active Version:</span>
          <span className="mono badge badge-info" style={{ fontSize: '11px' }}>
            v{status?.config_version || 1}
          </span>
        </div>

        <div className="live-pulse">
          <span className="pulse-dot" style={{ background: sseConnected ? '#10b981' : '#f59e0b' }}></span>
          <span style={{ color: sseConnected ? '#34d399' : '#fbbf24', fontSize: '12px' }}>
            {sseConnected ? 'STREAM ACTIVE' : 'RECONNECTING'}
          </span>
        </div>

        <button
          onClick={onRefresh}
          className="btn btn-secondary"
          style={{ padding: '6px 12px', fontSize: '12px' }}
          title="Refresh metrics and state"
          disabled={isRefreshing}
        >
          <RefreshCw size={14} className={isRefreshing ? 'spin' : ''} />
          Refresh
        </button>
      </div>
    </header>
  );
}
