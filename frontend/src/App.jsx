import { useState, useEffect } from "react";

const TABS = ["Topology", "Experiments", "Analysis"];

function App() {
  const [activeTab, setActiveTab] = useState("Topology");
  const [health, setHealth] = useState(null);
  const [emergencyStopped, setEmergencyStopped] = useState(false);

  useEffect(() => {
    fetch("/health")
      .then((r) => r.json())
      .then((data) => {
        setHealth(data);
        setEmergencyStopped(data.emergency_stop);
      })
      .catch(() => setHealth({ status: "unreachable" }));
  }, []);

  const handleEmergencyStop = async () => {
    if (!confirm("Trigger Emergency Stop? This will rollback ALL active experiments.")) {
      return;
    }
    const res = await fetch("/emergency-stop", { method: "POST" });
    if (res.ok) {
      setEmergencyStopped(true);
    }
  };

  return (
    <div style={{ fontFamily: "system-ui, sans-serif", maxWidth: 960, margin: "0 auto", padding: 20 }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
        <h1 style={{ margin: 0 }}>ChaosDuck</h1>
        <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
          <span style={{
            display: "inline-block",
            width: 10, height: 10, borderRadius: "50%",
            background: health?.status === "healthy" ? "#22c55e" : "#ef4444",
          }} />
          <span>{health?.status || "loading..."}</span>
          <button
            onClick={handleEmergencyStop}
            disabled={emergencyStopped}
            style={{
              background: emergencyStopped ? "#999" : "#dc2626",
              color: "white", border: "none", padding: "8px 16px",
              borderRadius: 4, cursor: emergencyStopped ? "not-allowed" : "pointer",
              fontWeight: "bold",
            }}
          >
            {emergencyStopped ? "STOPPED" : "EMERGENCY STOP"}
          </button>
        </div>
      </header>

      <nav style={{ display: "flex", gap: 0, borderBottom: "2px solid #e5e7eb", marginBottom: 24 }}>
        {TABS.map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            style={{
              padding: "8px 20px", border: "none", cursor: "pointer",
              background: activeTab === tab ? "#f3f4f6" : "transparent",
              borderBottom: activeTab === tab ? "2px solid #2563eb" : "2px solid transparent",
              fontWeight: activeTab === tab ? "bold" : "normal",
              marginBottom: -2,
            }}
          >
            {tab}
          </button>
        ))}
      </nav>

      <main>
        {activeTab === "Topology" && <TopologyPanel />}
        {activeTab === "Experiments" && <ExperimentsPanel />}
        {activeTab === "Analysis" && <AnalysisPanel />}
      </main>
    </div>
  );
}

function TopologyPanel() {
  return (
    <div>
      <h2>Infrastructure Topology</h2>
      <p>Topology visualization will be added in a future update (D3.js / React Flow).</p>
      <p>API: GET /api/topology/k8s, /api/topology/aws, /api/topology/combined</p>
    </div>
  );
}

function ExperimentsPanel() {
  const [experiments, setExperiments] = useState([]);

  useEffect(() => {
    fetch("/api/chaos/experiments")
      .then((r) => r.json())
      .then(setExperiments)
      .catch(() => {});
  }, []);

  return (
    <div>
      <h2>Chaos Experiments</h2>
      {experiments.length === 0 ? (
        <p>No experiments yet. Use the API or CLI to create one.</p>
      ) : (
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ borderBottom: "2px solid #e5e7eb", textAlign: "left" }}>
              <th style={{ padding: 8 }}>ID</th>
              <th style={{ padding: 8 }}>Name</th>
              <th style={{ padding: 8 }}>Type</th>
              <th style={{ padding: 8 }}>Status</th>
              <th style={{ padding: 8 }}>Phase</th>
            </tr>
          </thead>
          <tbody>
            {experiments.map((exp) => (
              <tr key={exp.experiment_id} style={{ borderBottom: "1px solid #e5e7eb" }}>
                <td style={{ padding: 8 }}>{exp.experiment_id}</td>
                <td style={{ padding: 8 }}>{exp.config.name}</td>
                <td style={{ padding: 8 }}>{exp.config.chaos_type}</td>
                <td style={{ padding: 8 }}>{exp.status}</td>
                <td style={{ padding: 8 }}>{exp.phase}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function AnalysisPanel() {
  return (
    <div>
      <h2>AI Analysis</h2>
      <p>AI-powered experiment analysis, hypothesis generation, and resilience scoring.</p>
      <p>API: POST /api/analysis/experiment/&#123;id&#125;, /api/analysis/hypotheses, /api/analysis/resilience-score</p>
    </div>
  );
}

export default App;
