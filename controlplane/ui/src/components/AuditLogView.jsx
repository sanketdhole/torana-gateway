import React from 'react';
import { ShieldCheck, FileText, CheckCircle2, AlertCircle, XCircle } from 'lucide-react';

export default function AuditLogView({ logs }) {
  const auditList = logs || [];

  const getStatusBadge = (st) => {
    switch (st) {
      case 'SUCCESS':
        return <span className="badge badge-success"><CheckCircle2 size={12} /> SUCCESS</span>;
      case 'BLOCKED':
        return <span className="badge badge-danger"><XCircle size={12} /> BLOCKED</span>;
      case 'FAILURE':
        return <span className="badge badge-warning"><AlertCircle size={12} /> FAILURE</span>;
      default:
        return <span className="badge badge-neutral">{st}</span>;
    }
  };

  return (
    <div>
      <div style={{ marginBottom: '24px' }}>
        <h2 style={{ fontSize: '20px', fontWeight: 700 }}>Security & Administrative Audit Trail</h2>
        <p style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          Immutable append-only record of all configuration releases, security revocations, and node enrollments
        </p>
      </div>

      <div className="glass-panel" style={{ padding: '20px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <h3 style={{ fontSize: '15px', fontWeight: 600 }}>Recorded Events ({auditList.length})</h3>
        </div>

        {auditList.length === 0 ? (
          <div className="empty-state">
            <FileText size={36} className="empty-state-icon" />
            <p style={{ fontSize: '14px', fontWeight: 500, color: 'var(--text-secondary)' }}>No Audit Events Yet</p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Timestamp</th>
                  <th>Actor / Role</th>
                  <th>Action</th>
                  <th>Resource</th>
                  <th>Status</th>
                  <th>Details</th>
                  <th>Client IP</th>
                </tr>
              </thead>
              <tbody>
                {auditList.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <span className="mono" style={{ fontSize: '12px' }}>
                        {new Date(item.timestamp).toLocaleString()}
                      </span>
                    </td>
                    <td>
                      <div style={{ fontWeight: 600, color: '#f3f4f6' }}>{item.actor}</div>
                      <span className="badge badge-neutral" style={{ fontSize: '10px' }}>{item.role}</span>
                    </td>
                    <td>
                      <span className="mono badge badge-info" style={{ fontWeight: 600 }}>{item.action}</span>
                    </td>
                    <td><span className="mono" style={{ color: '#818cf8' }}>{item.resource}</span></td>
                    <td>{getStatusBadge(item.status)}</td>
                    <td style={{ maxWidth: '320px' }}>
                      <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>{item.details}</span>
                    </td>
                    <td>
                      <span className="mono" style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                        {item.client_ip || 'internal'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
