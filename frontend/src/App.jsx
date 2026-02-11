import { useState } from "react";
import { fetchHealth, triggerEmergencyStop } from "./api";
import { usePolling } from "./hooks";
import { useToast } from "./ToastContext";
import StatusBadge from "./StatusBadge";
import Dashboard from "./Dashboard";
import ExperimentList from "./ExperimentList";
import ExperimentForm from "./ExperimentForm";
import TopologyView from "./TopologyView";
import AnalysisPanel from "./AnalysisPanel";

const TABS = ["Dashboard", "Experiments", "Topology", "Analysis"];

export default function App() {
  const [activeTab, setActiveTab] = useState("Dashboard");
  const [showForm, setShowForm] = useState(false);
  const { data: health } = usePolling(fetchHealth, 10000);
  const emergencyStopped = health?.emergency_stop ?? false;

  const toast = useToast();

  const handleEmergencyStop = async () => {
    if (!confirm("Trigger Emergency Stop? This will rollback ALL active experiments."))
      return;
    const res = await triggerEmergencyStop();
    if (res?.error) {
      toast(res.error, "error");
    } else {
      toast("Emergency stop triggered — all experiments rolling back", "warning");
    }
  };

  return (
    <div className="mx-auto max-w-6xl px-4 py-6">
      {/* Header */}
      <header className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">
          ChaosDuck
        </h1>
        <div className="flex items-center gap-3">
          <StatusBadge value={health?.status || "unknown"} />
          <button
            onClick={handleEmergencyStop}
            disabled={emergencyStopped}
            className={`rounded px-4 py-2 text-sm font-bold text-white ${
              emergencyStopped
                ? "cursor-not-allowed bg-gray-400"
                : "bg-red-600 hover:bg-red-700"
            }`}
          >
            {emergencyStopped ? "STOPPED" : "EMERGENCY STOP"}
          </button>
        </div>
      </header>

      {/* Tab Navigation */}
      <nav className="mb-6 flex border-b border-gray-200">
        {TABS.map((tab) => (
          <button
            key={tab}
            onClick={() => { setActiveTab(tab); setShowForm(false); }}
            className={`-mb-px border-b-2 px-5 py-2.5 text-sm font-medium transition-colors ${
              activeTab === tab
                ? "border-blue-600 text-blue-600"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            {tab}
          </button>
        ))}
      </nav>

      {/* Main Content */}
      <main>
        {activeTab === "Dashboard" && <Dashboard onNavigate={setActiveTab} />}
        {activeTab === "Experiments" && (
          <>
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold">Chaos Experiments</h2>
              <button
                onClick={() => setShowForm((v) => !v)}
                className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
              >
                {showForm ? "Close" : "+ New Experiment"}
              </button>
            </div>
            {showForm && (
              <ExperimentForm onCreated={() => setShowForm(false)} />
            )}
            <ExperimentList />
          </>
        )}
        {activeTab === "Topology" && <TopologyView />}
        {activeTab === "Analysis" && <AnalysisPanel />}
      </main>
    </div>
  );
}
