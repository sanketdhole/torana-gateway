import React from 'react';
import { Shield, RefreshCw, Key, Server, LogOut, UserCheck } from 'lucide-react';

export default function Header({
  status,
  user,
  onRefresh,
  isRefreshing,
  sseConnected,
  onLogout,
}) {
  const pubKeyShort = status?.public_key_hex
    ? `${status.public_key_hex.substring(0, 8)}...${status.public_key_hex.substring(status.public_key_hex.length - 6)}`
    : 'Signing Key Active';

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
        <div className="glass-panel header-chip" title={`Full Public Key: ${status?.public_key_hex}`}>
          <Key size={13} color="#818cf8" />
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Ed25519:</span>
          <span className="mono" style={{ fontSize: '11px', color: '#c7d2fe' }}>
            {pubKeyShort}
          </span>
        </div>

        <div className="glass-panel header-chip">
          <Server size={13} color="#34d399" />
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Active:</span>
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
          <RefreshCw size={13} className={isRefreshing ? 'spin' : ''} />
          <span>Refresh</span>
        </button>

        {/* User Session & Logout */}
        {user && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderLeft: '1px solid var(--border-color)', paddingLeft: '12px' }}>
            <div style={{ textAlign: 'right' }}>
              <div style={{ fontSize: '12px', fontWeight: 600, color: '#f3f4f6' }}>
                {user.username || 'admin'}
              </div>
              <div style={{ fontSize: '10px', color: '#34d399', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                {user.role || 'ADMIN'}
              </div>
            </div>

            <button
              onClick={onLogout}
              className="btn btn-secondary"
              style={{ padding: '6px 10px', fontSize: '11px', color: '#fb7185' }}
              title="Logout from control plane"
            >
              <LogOut size={13} />
              <span>Logout</span>
            </button>
          </div>
        )}
      </div>
    </header>
  );
}
