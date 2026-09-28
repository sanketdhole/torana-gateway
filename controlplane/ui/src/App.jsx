import React, { useState, useEffect, useCallback } from 'react';
import Header from './components/Header';
import LoginModal from './components/LoginModal';
import FleetView from './components/FleetView';
import ConfigStudio from './components/ConfigStudio';
import RolloutPublisher from './components/RolloutPublisher';
import RevocationManager from './components/RevocationManager';
import TelemetryView from './components/TelemetryView';
import AuditLogView from './components/AuditLogView';
import {
  fetchMe,
  logout,
  fetchStatus,
  fetchNodes,
  fetchConfig,
  fetchConfigHistory,
  publishConfig,
  rollbackConfig,
  upsertRoute,
  deleteRoute,
  upsertPolicy,
  deletePolicy,
  upsertUpstream,
  deleteUpstream,
  fetchRevocations,
  addRevocation,
  fetchUsage,
  fetchAuditLogs,
  subscribeToEvents,
} from './api';
import { Server, Sliders, Send, ShieldAlert, BarChart3, FileText, CheckCircle2, AlertCircle } from 'lucide-react';

export default function App() {
  const [user, setUser] = useState(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [activeTab, setActiveTab] = useState('fleet');

  const [status, setStatus] = useState(null);
  const [nodes, setNodes] = useState([]);
  const [config, setConfig] = useState(null);
  const [history, setHistory] = useState([]);
  const [revocations, setRevocations] = useState({ revoked_keys: [], revoked_tokens: [] });
  const [usage, setUsage] = useState([]);
  const [auditLogs, setAuditLogs] = useState([]);

  const [isRefreshing, setIsRefreshing] = useState(false);
  const [sseConnected, setSseConnected] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);
  const [isRevoking, setIsRevoking] = useState(false);
  const [lastPublishResult, setLastPublishResult] = useState(null);
  const [toast, setToast] = useState(null);

  const showToast = (message, type = 'success') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 4000);
  };

  const loadAllData = useCallback(async () => {
    if (!isAuthenticated) return;
    setIsRefreshing(true);
    try {
      const [st, nds, cfg, hist, revs, usg, aud] = await Promise.all([
        fetchStatus().catch(() => null),
        fetchNodes().then((r) => r.nodes).catch(() => []),
        fetchConfig().catch(() => null),
        fetchConfigHistory().then((r) => r.history).catch(() => []),
        fetchRevocations().catch(() => ({ revoked_keys: [], revoked_tokens: [] })),
        fetchUsage().then((r) => r.usage_records).catch(() => []),
        fetchAuditLogs().then((r) => r.audit_logs).catch(() => []),
      ]);

      if (st) setStatus(st);
      if (nds) setNodes(nds);
      if (cfg) setConfig(cfg);
      if (hist) setHistory(hist);
      if (revs) setRevocations(revs);
      if (usg) setUsage(usg);
      if (aud) setAuditLogs(aud);
    } catch (err) {
      console.error('Error fetching dashboard data:', err);
    } finally {
      setIsRefreshing(false);
    }
  }, [isAuthenticated]);

  // Check login state on initial load
  useEffect(() => {
    fetchMe()
      .then((u) => {
        setUser(u);
        setIsAuthenticated(true);
      })
      .catch(() => {
        setIsAuthenticated(false);
        setUser(null);
      });

    const handleUnauthorized = () => {
      setIsAuthenticated(false);
      setUser(null);
    };

    window.addEventListener('torana_unauthorized', handleUnauthorized);
    return () => window.removeEventListener('torana_unauthorized', handleUnauthorized);
  }, []);

  // SSE subscription & periodic polling when authenticated
  useEffect(() => {
    if (!isAuthenticated) return;

    loadAllData();

    const unsubscribe = subscribeToEvents(
      (data) => {
        setSseConnected(true);
        if (
          data.type === 'node_enrolled' ||
          data.type === 'node_ack' ||
          data.type === 'node_nack' ||
          data.type === 'node_hello'
        ) {
          setNodes((prev) => {
            const idx = prev.findIndex((n) => n.node_id === data.node.node_id);
            if (idx >= 0) {
              const copy = [...prev];
              copy[idx] = data.node;
              return copy;
            }
            return [...prev, data.node];
          });
        } else if (data.type === 'config_published' || data.type === 'config_rollback') {
          loadAllData();
        } else if (data.type === 'revocation_issued') {
          fetchRevocations().then(setRevocations);
        }
      },
      () => setSseConnected(false)
    );

    const interval = setInterval(loadAllData, 5000);

    return () => {
      unsubscribe();
      clearInterval(interval);
    };
  }, [isAuthenticated, loadAllData]);

  const handleLoginSuccess = (userData) => {
    setUser(userData);
    setIsAuthenticated(true);
    showToast(`Welcome back, ${userData.username || 'admin'}!`, 'success');
  };

  const handleLogout = async () => {
    await logout();
    setUser(null);
    setIsAuthenticated(false);
    showToast('Logged out of control plane session', 'neutral');
  };

  // Fine-grained route handlers
  const handleSaveRoute = async (route) => {
    try {
      const res = await upsertRoute(route);
      showToast(`Route '${route.id}' saved and broadcast as v${res.config_version}!`, 'success');
      await loadAllData();
    } catch (err) {
      showToast(`Failed to update route: ${err.message}`, 'error');
    }
  };

  const handleDeleteRoute = async (routeId) => {
    try {
      const res = await deleteRoute(routeId);
      showToast(`Route '${routeId}' deleted and broadcast as v${res.config_version}!`, 'success');
      await loadAllData();
    } catch (err) {
      showToast(`Failed to delete route: ${err.message}`, 'error');
    }
  };

  // Fine-grained policy handlers
  const handleSavePolicy = async (policy) => {
    try {
      const res = await upsertPolicy(policy);
      showToast(`Policy '${policy.name || policy.id}' saved and broadcast as v${res.config_version}!`, 'success');
      await loadAllData();
    } catch (err) {
      showToast(`Failed to update policy: ${err.message}`, 'error');
    }
  };

  const handleDeletePolicy = async (policyId) => {
    try {
      const res = await deletePolicy(policyId);
      showToast(`Policy '${policyId}' deleted and broadcast as v${res.config_version}!`, 'success');
      await loadAllData();
    } catch (err) {
      showToast(`Failed to delete policy: ${err.message}`, 'error');
    }
  };

  // Fine-grained upstream handlers
  const handleSaveUpstream = async (upstream) => {
    try {
      const res = await upsertUpstream(upstream);
      showToast(`Upstream cluster '${upstream.id}' saved and broadcast as v${res.config_version}!`, 'success');
      await loadAllData();
    } catch (err) {
      showToast(`Failed to update upstream: ${err.message}`, 'error');
    }
  };

  const handleDeleteUpstream = async (upstreamId) => {
    try {
      const res = await deleteUpstream(upstreamId);
      showToast(`Upstream cluster '${upstreamId}' deleted and broadcast as v${res.config_version}!`, 'success');
      await loadAllData();
    } catch (err) {
      showToast(`Failed to delete upstream: ${err.message}`, 'error');
    }
  };

  // Full snapshot publish & rollback
  const handlePublish = async (comment) => {
    if (!config) return;
    setIsPublishing(true);
    try {
      const res = await publishConfig(config, comment);
      setLastPublishResult(res);
      showToast(`Published v${res.config_version} signed with Ed25519 to ${res.nodes_notified} node streams!`, 'success');
      await loadAllData();
    } catch (err) {
      showToast(`Publish failed: ${err.message}`, 'error');
    } finally {
      setIsPublishing(false);
    }
  };

  const handleRollback = async (version) => {
    try {
      const res = await rollbackConfig(version);
      showToast(`Rolled back to snapshot v${version}, new active version is v${res.config_version}!`, 'success');
      await loadAllData();
    } catch (err) {
      showToast(`Rollback failed: ${err.message}`, 'error');
    }
  };

  const handleAddRevocation = async (keys, tokens, reason) => {
    setIsRevoking(true);
    try {
      const res = await addRevocation(keys, tokens, reason);
      showToast(`Broadcasted revocation to ${res.nodes_notified} nodes!`, 'success');
      const updated = await fetchRevocations();
      setRevocations(updated);
      const aud = await fetchAuditLogs();
      setAuditLogs(aud.audit_logs || []);
    } catch (err) {
      showToast(`Revocation failed: ${err.message}`, 'error');
    } finally {
      setIsRevoking(false);
    }
  };

  const fleetStats = status?.fleet_stats || {
    total_nodes: nodes.length,
    connected_nodes: nodes.filter((n) => n.health_status !== 'DISCONNECTED').length,
    healthy_nodes: nodes.filter((n) => n.health_status === 'HEALTHY').length,
    lagging_nodes: nodes.filter((n) => n.health_status === 'LAGGING').length,
    degraded_nodes: nodes.filter((n) => n.health_status === 'DEGRADED').length,
    total_active_conns: nodes.reduce((a, b) => a + (b.active_connections || 0), 0),
    total_memory_bytes: nodes.reduce((a, b) => a + (b.memory_allocated_bytes || 0), 0),
  };

  if (!isAuthenticated) {
    return <LoginModal onLoginSuccess={handleLoginSuccess} />;
  }

  return (
    <div className="app-container">
      {/* Toast Notification */}
      {toast && (
        <div className={`toast-notification toast-${toast.type}`}>
          {toast.type === 'success' ? <CheckCircle2 size={16} /> : <AlertCircle size={16} />}
          <span>{toast.message}</span>
        </div>
      )}

      <Header
        status={status}
        user={user}
        onRefresh={loadAllData}
        isRefreshing={isRefreshing}
        sseConnected={sseConnected}
        onLogout={handleLogout}
      />

      <nav className="nav-tabs">
        <button
          onClick={() => setActiveTab('fleet')}
          className={`nav-tab ${activeTab === 'fleet' ? 'active' : ''}`}
        >
          <Server size={16} /> Fleet Gateways ({nodes.length})
        </button>
        <button
          onClick={() => setActiveTab('studio')}
          className={`nav-tab ${activeTab === 'studio' ? 'active' : ''}`}
        >
          <Sliders size={16} /> Config Studio
        </button>
        <button
          onClick={() => setActiveTab('rollout')}
          className={`nav-tab ${activeTab === 'rollout' ? 'active' : ''}`}
        >
          <Send size={16} /> Rollout Publisher
        </button>
        <button
          onClick={() => setActiveTab('revocations')}
          className={`nav-tab ${activeTab === 'revocations' ? 'active' : ''}`}
        >
          <ShieldAlert size={16} /> Revocation Center
        </button>
        <button
          onClick={() => setActiveTab('telemetry')}
          className={`nav-tab ${activeTab === 'telemetry' ? 'active' : ''}`}
        >
          <BarChart3 size={16} /> Telemetry & Usage
        </button>
        <button
          onClick={() => setActiveTab('audit')}
          className={`nav-tab ${activeTab === 'audit' ? 'active' : ''}`}
        >
          <FileText size={16} /> Audit Trail
        </button>
      </nav>

      <main className="main-content">
        {activeTab === 'fleet' && (
          <FleetView
            nodes={nodes}
            stats={fleetStats}
            targetVersion={status?.config_version}
          />
        )}

        {activeTab === 'studio' && (
          <ConfigStudio
            config={config}
            onSaveRoute={handleSaveRoute}
            onDeleteRoute={handleDeleteRoute}
            onSavePolicy={handleSavePolicy}
            onDeletePolicy={handleDeletePolicy}
            onSaveUpstream={handleSaveUpstream}
            onDeleteUpstream={handleDeleteUpstream}
            onDirectDeploy={handlePublish}
            isDeploying={isPublishing}
          />
        )}

        {activeTab === 'rollout' && (
          <RolloutPublisher
            config={config}
            nodes={nodes}
            history={history}
            onPublish={handlePublish}
            onRollback={handleRollback}
            isPublishing={isPublishing}
            lastPublishResult={lastPublishResult}
          />
        )}

        {activeTab === 'revocations' && (
          <RevocationManager
            revocations={revocations}
            onAddRevocation={handleAddRevocation}
            isSubmitting={isRevoking}
          />
        )}

        {activeTab === 'telemetry' && <TelemetryView usageRecords={usage} />}

        {activeTab === 'audit' && <AuditLogView logs={auditLogs} />}
      </main>
    </div>
  );
}
