import React, { useState } from 'react';
import { Ban, ShieldOff, Key, Lock, AlertOctagon, Check } from 'lucide-react';

export default function RevocationManager({ revocations, onAddRevocation, isSubmitting }) {
  const [keysInput, setKeysInput] = useState('');
  const [tokensInput, setTokensInput] = useState('');
  const [reason, setReason] = useState('');
  const [successMsg, setSuccessMsg] = useState('');

  const keys = revocations?.revoked_keys || [];
  const tokens = revocations?.revoked_tokens || [];

  const handleSubmit = async (e) => {
    e.preventDefault();
    const keyList = keysInput.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean);
    const tokenList = tokensInput.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean);

    if (keyList.length === 0 && tokenList.length === 0) {
      alert('Please specify at least one API key or token to revoke.');
      return;
    }

    await onAddRevocation(keyList, tokenList, reason || 'Operator triggered manual revocation');
    setKeysInput('');
    setTokensInput('');
    setReason('');
    setSuccessMsg(`Revoked ${keyList.length} keys and ${tokenList.length} tokens across all active gateways.`);
    setTimeout(() => setSuccessMsg(''), 5000);
  };

  return (
    <div>
      <div style={{ marginBottom: '24px' }}>
        <h2 style={{ fontSize: '20px', fontWeight: 700 }}>Security Revocation Center</h2>
        <p style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          Instantly invalidate compromised API keys or bearer tokens across the entire distributed data plane fleet
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px' }}>
        {/* Revoke Form */}
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
            <AlertOctagon size={20} color="#fb7185" />
            <h3 style={{ fontSize: '16px', fontWeight: 600 }}>Emergency Credential Revocation</h3>
          </div>

          <form onSubmit={handleSubmit}>
            <div style={{ marginBottom: '14px' }}>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                Revoke API Keys (comma or line separated):
              </label>
              <textarea
                rows={3}
                placeholder="key_live_9a8b..., key_test_1f2e..."
                value={keysInput}
                onChange={(e) => setKeysInput(e.target.value)}
                className="mono"
              />
            </div>

            <div style={{ marginBottom: '14px' }}>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                Revoke JWT / Bearer Token Signatures (hashes or IDs):
              </label>
              <textarea
                rows={3}
                placeholder="tok_sig_4c5d..., jti_e8a9..."
                value={tokensInput}
                onChange={(e) => setTokensInput(e.target.value)}
                className="mono"
              />
            </div>

            <div style={{ marginBottom: '18px' }}>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                Revocation Justification / Incident ID:
              </label>
              <input
                type="text"
                placeholder="INC-8921: Suspected developer credential leak"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </div>

            <button
              type="submit"
              disabled={isSubmitting}
              className="btn btn-danger"
              style={{ width: '100%', padding: '10px 16px', fontSize: '14px' }}
            >
              <ShieldOff size={16} />
              {isSubmitting ? 'Propagating Revocation...' : 'Broadcast Immediate Revocation to Fleet'}
            </button>
          </form>

          {successMsg && (
            <div style={{ marginTop: '16px', padding: '12px', background: 'rgba(16, 185, 129, 0.1)', border: '1px solid rgba(16, 185, 129, 0.25)', borderRadius: '8px', fontSize: '12px', color: '#34d399', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Check size={16} /> {successMsg}
            </div>
          )}
        </div>

        {/* Active Revocation Blocklist */}
        <div className="glass-panel" style={{ padding: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '16px', fontWeight: 600 }}>Active Blacklist Registry</h3>
            <span className="badge badge-danger">{keys.length + tokens.length} Blocked</span>
          </div>

          <div style={{ marginBottom: '18px' }}>
            <h4 style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
              Revoked API Keys ({keys.length})
            </h4>
            {keys.length === 0 ? (
              <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>No API keys currently revoked.</p>
            ) : (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', maxHeight: '140px', overflowY: 'auto' }}>
                {keys.map((k) => (
                  <span key={k} className="mono badge badge-danger" style={{ fontSize: '11px' }}>
                    <Key size={10} /> {k}
                  </span>
                ))}
              </div>
            )}
          </div>

          <div>
            <h4 style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
              Revoked Tokens / JTIs ({tokens.length})
            </h4>
            {tokens.length === 0 ? (
              <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>No tokens currently revoked.</p>
            ) : (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', maxHeight: '140px', overflowY: 'auto' }}>
                {tokens.map((t) => (
                  <span key={t} className="mono badge badge-danger" style={{ fontSize: '11px' }}>
                    <Lock size={10} /> {t}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
