import React, { useState, useEffect, useCallback } from 'react';
import Header from './components/Header';
import FleetView from './components/FleetView';
import ConfigStudio from './components/ConfigStudio';
import RolloutPublisher from './components/RolloutPublisher';
import RevocationManager from './components/RevocationManager';
import TelemetryView from './components/TelemetryView';
import AuditLogView from './components/AuditLogView';
import {
  fetchStatus,
  fetchNodes,
  fetchConfig,
  fetchConfigHistory,
  publishConfig,
  rollbackConfig,
  fetchRevocations,
  addRevocation,
  fetchUsage,
  fetchAuditLogs,
  subscribeToEvents,
} from './api';
import { Server, Sliders, Send, ShieldAlert, BarChart3, FileText } from 'lucide-react';

export default function App() {
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

  const loadAllData = useCallback(async () => {
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
  }, []);

  useEffect(() => {
    loadAllData();

    // Subscribe to SSE
    const unsubscribe = subscribeToEvents(
      (data) => {
        setSseConnected(true);
        if (data.type === 'node_enrolled' || data.type === 'node_ack' || data.type === 'node_nack' || data.type === 'node_hello') {
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

    // Periodic poll fallback every 5s
    const interval = setInterval(loadAllData, 5000);

    return () => {
      unsubscribe();
      clearInterval(interval);
    };
  }, [loadAllData]);

  const handlePublish = async (comment) => {
    if (!config) return;
    setIsPublishing(true);
    try {
      const res = await publishConfig(config, comment);
      setLastPublishResult(res);
      await loadAllData();
    } catch (err) {
      alert(`Publish failed: ${err.message}`);
    } finally {
      setIsPublishing(false);
    }
  };

  const handleRollback = async (version) => {
    try {
      await rollbackConfig(version);
      await loadAllData();
    } catch (err) {
      alert(`Rollback failed: ${err.message}`);
    }
  };

  const handleAddRevocation = async (keys, tokens, reason) => {
    setIsRevoking(true);
    try {
      await addRevocation(keys, tokens, reason);
      const updated = await fetchRevocations();
      setRevocations(updated);
      const aud = await fetchAuditLogs();
      setAuditLogs(aud.audit_logs || []);
    } catch (err) {
      alert(`Revocation failed: ${err.message}`);
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

  return (
    <div className="app-container">
      <Header
        status={status}
        onRefresh={loadAllData}
        isRefreshing={isRefreshing}
        sseConnected={sseConnected}
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
            onUpdateConfig={setConfig}
            onNavigateToPublish={() => setActiveTab('rollout')}
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

        {activeTab === 'telemetry' && (
          <TelemetryView usageRecords={usage} />
        )}

        {activeTab === 'audit' && (
          <AuditLogView logs={auditLogs} />
        )}
      </main>
    </div>
  );
}
