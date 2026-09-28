import React, { useState } from 'react';
import { Shield, Lock, Key, ArrowRight, AlertCircle, Eye, EyeOff } from 'lucide-react';
import { login } from '../api';

export default function LoginModal({ onLoginSuccess }) {
  const [username, setUsername] = useState('admin');
  const [secret, setSecret] = useState('torana-admin-secret-key');
  const [showSecret, setShowSecret] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      const data = await login(username, secret);
      onLoginSuccess(data);
    } catch (err) {
      setError(err.message || 'Authentication failed. Please check credentials.');
    } finally {
      setIsLoading(false);
    }
  };

  const handleFillDevKey = () => {
    setUsername('admin');
    setSecret('torana-admin-secret-key');
  };

  return (
    <div className="login-overlay">
      <div className="login-card glass-panel">
        <div className="login-header">
          <div className="login-logo">
            <Shield size={28} />
          </div>
          <h2 className="login-title">Torana Enterprise</h2>
          <p className="login-subtitle">Control Plane & Fleet Management Security Portal</p>
        </div>

        {error && (
          <div className="login-error">
            <AlertCircle size={16} />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="login-form">
          <div className="form-group">
            <label>Operator Username</label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="e.g. admin"
              required
              autoFocus
            />
          </div>

          <div className="form-group">
            <label>Admin Secret Key / Password</label>
            <div className="password-input-wrapper">
              <input
                type={showSecret ? 'text' : 'password'}
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder="Enter admin token or password"
                required
                className="mono"
              />
              <button
                type="button"
                onClick={() => setShowSecret(!showSecret)}
                className="toggle-password-btn"
                tabIndex={-1}
              >
                {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          <div className="dev-hint-box">
            <span>Dev Secret:</span>
            <code onClick={handleFillDevKey} className="dev-key-code" title="Click to fill">
              torana-admin-secret-key
            </code>
          </div>

          <button type="submit" disabled={isLoading} className="btn btn-primary login-submit-btn">
            {isLoading ? (
              'Authenticating...'
            ) : (
              <>
                <span>Authenticate & Access Console</span>
                <ArrowRight size={16} />
              </>
            )}
          </button>
        </form>

        <div className="login-footer">
          <Lock size={12} />
          <span>Zero-Trust Perimeter • End-to-End Cryptographically Signed Snapshots</span>
        </div>
      </div>
    </div>
  );
}
