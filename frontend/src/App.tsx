import { useState, useEffect } from 'react';
import './App.css';

interface HealthData {
  status: string;
  service: string;
  version: string;
  timestamp: string;
}

interface ReadyData {
  status: string;
  service: string;
  database: string;
  timestamp: string;
}

interface Money {
  amount: number;
  currency: string;
}

interface ExceptionItem {
  id: string;
  job_id: string;
  transaction_reference: string;
  type: string;
  status: string;
  ledger_amount: Money;
  gateway_amount: Money;
  difference_amount: Money;
  description: string;
  created_at: string;
}

const BACKEND_BASE = 'http://localhost:8086';

export default function App() {
  const [health, setHealth] = useState<HealthData | null>(null);
  const [healthLoading, setHealthLoading] = useState<boolean>(false);
  const [healthLatency, setHealthLatency] = useState<number | null>(null);
  const [healthError, setHealthError] = useState<string | null>(null);

  const [ready, setReady] = useState<ReadyData | null>(null);
  const [readyLoading, setReadyLoading] = useState<boolean>(false);
  const [readyError, setReadyError] = useState<string | null>(null);

  const [exceptions, setExceptions] = useState<ExceptionItem[]>([]);
  const [loadingExceptions, setLoadingExceptions] = useState<boolean>(false);
  const [reconciling, setReconciling] = useState<boolean>(false);

  // Fetch /health (GEN-10)
  const fetchHealth = async () => {
    setHealthLoading(true);
    setHealthError(null);
    const start = performance.now();
    try {
      const res = await fetch(`${BACKEND_BASE}/health`, {
        headers: { 'Accept': 'application/json' }
      });
      const latency = Math.round(performance.now() - start);
      setHealthLatency(latency);
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: ${res.statusText}`);
      }
      const data: HealthData = await res.json();
      setHealth(data);
    } catch (err: any) {
      setHealthError(err.message || 'Failed to fetch /health');
    } finally {
      setHealthLoading(false);
    }
  };

  // Fetch /ready (GEN-10)
  const fetchReady = async () => {
    setReadyLoading(true);
    setReadyError(null);
    try {
      const res = await fetch(`${BACKEND_BASE}/ready`, {
        headers: { 'Accept': 'application/json' }
      });
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: ${res.statusText}`);
      }
      const data: ReadyData = await res.json();
      setReady(data);
    } catch (err: any) {
      setReadyError(err.message || 'Failed to fetch /ready');
    } finally {
      setReadyLoading(false);
    }
  };

  // Fetch exceptions
  const fetchExceptions = async () => {
    setLoadingExceptions(true);
    try {
      const res = await fetch(`${BACKEND_BASE}/api/v1/exceptions`);
      if (res.ok) {
        const data = await res.json();
        setExceptions(data.items || []);
      }
    } catch (err) {
      console.error('Failed to load exceptions', err);
    } finally {
      setLoadingExceptions(false);
    }
  };

  // Trigger reconciliation job
  const handleRunReconciliation = async () => {
    setReconciling(true);
    try {
      const res = await fetch(`${BACKEND_BASE}/api/v1/reconciliations/jobs`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Idempotency-Key': crypto.randomUUID(),
        },
        body: JSON.stringify({
          from_time: new Date(Date.now() - 86400000).toISOString(),
          to_time: new Date().toISOString(),
          scope: 'ALL',
        }),
      });
      if (res.ok) {
        await fetchExceptions();
      }
    } catch (err) {
      console.error('Failed to run reconciliation', err);
    } finally {
      setReconciling(false);
    }
  };

  useEffect(() => {
    fetchHealth();
    fetchReady();
    fetchExceptions();
  }, []);

  const formatRials = (num: number) => {
    return new Intl.NumberFormat('fa-IR').format(num) + ' ریال';
  };

  return (
    <div className="dashboard-container" id="app-root">
      {/* Header */}
      <header className="dashboard-header" id="dashboard-header">
        <div className="brand-section">
          <div className="logo-badge" id="service-badge">RCN</div>
          <div>
            <h1 className="brand-title" id="page-title">LabBank Core Banking</h1>
            <div className="brand-subtitle" id="service-subtitle">
              S6-RCN • Reconciliation & Exceptions Service
            </div>
          </div>
        </div>

        <div className="header-actions">
          {health?.status === 'ok' ? (
            <span className="badge badge-success pulse" id="system-status-badge">
              <span className="badge-dot"></span>
              Backend Online (Port 8086)
            </span>
          ) : (
            <span className="badge badge-error" id="system-status-badge">
              <span className="badge-dot"></span>
              Backend Offline
            </span>
          )}

          <a
            href="http://localhost:8086/swagger/"
            target="_blank"
            rel="noopener noreferrer"
            className="btn-secondary"
            id="link-swagger-ui"
          >
            <span>📜</span> Swagger UI
          </a>
        </div>
      </header>

      {/* Stats Summary */}
      <section className="stats-grid" id="stats-section">
        <div className="stat-card">
          <div className="stat-label">Service Code</div>
          <div className="stat-value" style={{ color: 'var(--accent-cyan)' }}>S6-RCN</div>
          <div className="stat-sub">Reconciliation & Exceptions</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">PostgreSQL Database</div>
          <div className="stat-value" style={{ color: ready?.database === 'connected' ? 'var(--accent-emerald)' : 'var(--accent-amber)' }}>
            {ready?.database === 'connected' ? 'Connected' : 'Connecting...'}
          </div>
          <div className="stat-sub">Container: labbank-postgres:5432</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Open Exceptions</div>
          <div className="stat-value" style={{ color: 'var(--accent-rose)' }}>
            {exceptions.length}
          </div>
          <div className="stat-sub">Active discrepancies</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Response Latency</div>
          <div className="stat-value" style={{ color: 'var(--accent-indigo)' }}>
            {healthLatency !== null ? `${healthLatency} ms` : '--'}
          </div>
          <div className="stat-sub">Local HTTP roundtrip</div>
        </div>
      </section>

      {/* Main Grid: Health Check & Database Readiness */}
      <section className="dashboard-grid">
        {/* GET /health Card (Requirement 3.4 & Deliverable 5) */}
        <div className="card glass-panel" id="health-card">
          <div className="card-header">
            <h2 className="card-title">
              <span className="card-icon">🩺</span>
              Process Health Check
            </h2>
            <button
              id="btn-check-health"
              className="btn-primary"
              onClick={fetchHealth}
              disabled={healthLoading}
            >
              {healthLoading ? <div className="spinner"></div> : '🔄 Test GET /health'}
            </button>
          </div>

          <div className="kv-list">
            <div className="kv-item">
              <span className="kv-key">Endpoint</span>
              <span className="kv-value">GET /health</span>
            </div>
            <div className="kv-item">
              <span className="kv-key">Status</span>
              <span className="kv-value">
                {health?.status === 'ok' ? (
                  <span style={{ color: 'var(--accent-emerald)' }}>● 200 OK (Healthy)</span>
                ) : (
                  <span style={{ color: 'var(--accent-rose)' }}>● Error / Down</span>
                )}
              </span>
            </div>
            <div className="kv-item">
              <span className="kv-key">Service Identifier</span>
              <span className="kv-value">{health?.service || 'N/A'}</span>
            </div>
            <div className="kv-item">
              <span className="kv-key">Timestamp (UTC)</span>
              <span className="kv-value">{health?.timestamp || 'N/A'}</span>
            </div>
          </div>

          <div style={{ marginTop: 'auto' }}>
            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
              Raw Response Payload:
            </div>
            <pre className="code-container" id="health-json-response">
              {healthLoading
                ? 'Requesting GET /health...'
                : healthError
                ? `Error: ${healthError}`
                : JSON.stringify(health, null, 2)}
            </pre>
          </div>
        </div>

        {/* GET /ready Card */}
        <div className="card glass-panel" id="readiness-card">
          <div className="card-header">
            <h2 className="card-title">
              <span className="card-icon">🗄️</span>
              PostgreSQL Readiness
            </h2>
            <button
              id="btn-check-ready"
              className="btn-secondary"
              onClick={fetchReady}
              disabled={readyLoading}
            >
              {readyLoading ? <div className="spinner"></div> : '🔄 Test GET /ready'}
            </button>
          </div>

          <div className="kv-list">
            <div className="kv-item">
              <span className="kv-key">Endpoint</span>
              <span className="kv-value">GET /ready</span>
            </div>
            <div className="kv-item">
              <span className="kv-key">Database State</span>
              <span className="kv-value">
                {ready?.database === 'connected' ? (
                  <span style={{ color: 'var(--accent-emerald)' }}>● Connected (Active)</span>
                ) : (
                  <span style={{ color: 'var(--accent-amber)' }}>● Offline</span>
                )}
              </span>
            </div>
            <div className="kv-item">
              <span className="kv-key">Data Volume</span>
              <span className="kv-value">labbank_pgdata (Persistent)</span>
            </div>
            <div className="kv-item">
              <span className="kv-key">Standard</span>
              <span className="kv-value">GEN-10 Health & Readiness</span>
            </div>
          </div>

          <div style={{ marginTop: 'auto' }}>
            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
              Readiness Response:
            </div>
            <pre className="code-container" id="ready-json-response">
              {readyLoading
                ? 'Requesting GET /ready...'
                : readyError
                ? `Error: ${readyError}`
                : JSON.stringify(ready, null, 2)}
            </pre>
          </div>
        </div>
      </section>

      {/* Reconciliation Exceptions Management */}
      <section className="card glass-panel" id="exceptions-section">
        <div className="card-header">
          <div>
            <h2 className="card-title">
              <span className="card-icon">⚖️</span>
              Financial Discrepancy & Exceptions Management
            </h2>
            <p style={{ fontSize: '0.8125rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>
              Automated reconciliation matches records between Ledger (S2) and Settlement Gateway (S5).
            </p>
          </div>

          <button
            id="btn-run-reconciliation"
            className="btn-primary"
            onClick={handleRunReconciliation}
            disabled={reconciling}
          >
            {reconciling ? <div className="spinner"></div> : '⚡ Run Reconciliation'}
          </button>
        </div>

        <div className="table-responsive">
          <table className="custom-table" id="exceptions-table">
            <thead>
              <tr>
                <th>Tx Reference</th>
                <th>Discrepancy Type</th>
                <th>Status</th>
                <th>Ledger (S2)</th>
                <th>Gateway (S5)</th>
                <th>Difference</th>
                <th>Description</th>
              </tr>
            </thead>
            <tbody>
              {loadingExceptions ? (
                <tr>
                  <td colSpan={7} style={{ textAlign: 'center', padding: '2rem' }}>
                    Loading exceptions...
                  </td>
                </tr>
              ) : exceptions.length === 0 ? (
                <tr>
                  <td colSpan={7} style={{ textAlign: 'center', padding: '2rem', color: 'var(--text-muted)' }}>
                    No open discrepancies. All accounts in balance!
                  </td>
                </tr>
              ) : (
                exceptions.map((item) => (
                  <tr key={item.id} id={`exception-row-${item.transaction_reference}`}>
                    <td className="text-mono" style={{ fontWeight: 600 }}>{item.transaction_reference}</td>
                    <td>
                      <span className="badge badge-warning">{item.type}</span>
                    </td>
                    <td>
                      <span className="badge badge-error">{item.status}</span>
                    </td>
                    <td className="text-mono">{formatRials(item.ledger_amount.amount)}</td>
                    <td className="text-mono">{formatRials(item.gateway_amount.amount)}</td>
                    <td className="text-mono text-danger">{formatRials(item.difference_amount.amount)}</td>
                    <td style={{ maxWidth: '320px', color: 'var(--text-secondary)' }}>{item.description}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
