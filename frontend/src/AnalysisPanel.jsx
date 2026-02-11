import { useState } from "react";
import {
  nlExperiment,
  createExperiment,
  getCombinedTopology,
  generateExperiments,
  getResilienceTrend,
} from "./api";
import { useApi } from "./hooks";
import { useToast } from "./ToastContext";
import ResilienceChart from "./ResilienceChart";

/* ── Section 1: Natural Language Experiment Creator ── */
function NlExperimentSection() {
  const [text, setText] = useState("");
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [created, setCreated] = useState(false);

  const handleParse = async () => {
    if (!text.trim()) return;
    setLoading(true);
    setError(null);
    setConfig(null);
    setCreated(false);
    const res = await nlExperiment({ text });
    if (res?.error) {
      setError(res.error);
    } else {
      setConfig(res);
    }
    setLoading(false);
  };

  const handleRun = async () => {
    if (!config) return;
    setLoading(true);
    const res = await createExperiment(config);
    if (res?.error) {
      setError(res.error);
    } else {
      setCreated(true);
    }
    setLoading(false);
  };

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <h3 className="mb-3 text-sm font-semibold text-gray-700">
        Natural Language Experiment
      </h3>
      <div className="flex gap-2">
        <input
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && handleParse()}
          placeholder='e.g. "Delete nginx pods in staging and test recovery"'
          className="flex-1 rounded border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
        />
        <button
          onClick={handleParse}
          disabled={loading || !text.trim()}
          className="rounded bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          {loading ? "Parsing..." : "Parse"}
        </button>
      </div>

      {error && (
        <p className="mt-2 text-sm text-red-500">{error}</p>
      )}

      {config && !created && (
        <div className="mt-3">
          <pre className="mb-2 max-h-40 overflow-auto rounded bg-gray-50 p-3 text-xs">
            {JSON.stringify(config, null, 2)}
          </pre>
          <button
            onClick={handleRun}
            disabled={loading}
            className="rounded bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            Run This Experiment
          </button>
        </div>
      )}

      {created && (
        <p className="mt-2 text-sm text-green-600">
          Experiment created successfully!
        </p>
      )}
    </div>
  );
}

/* ── Section 2: AI Experiment Recommendations ── */
function AiRecommendations() {
  const [namespace, setNamespace] = useState("default");
  const [recommendations, setRecommendations] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const toast = useToast();

  const handleGenerate = async () => {
    setLoading(true);
    setError(null);
    setRecommendations(null);
    const topo = await getCombinedTopology(namespace);
    if (topo?.error) {
      setError(topo.error);
      setLoading(false);
      return;
    }
    const res = await generateExperiments({
      topology: topo,
      target_namespace: namespace,
      count: 3,
    });
    if (res?.error) {
      setError(res.error);
    } else {
      setRecommendations(res.experiments ?? []);
    }
    setLoading(false);
  };

  const handleRun = async (config) => {
    const res = await createExperiment(config);
    if (res?.error) {
      toast(res.error, "error");
    } else {
      toast(`Experiment ${res.experiment_id} created`, "success");
    }
  };

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <h3 className="mb-3 text-sm font-semibold text-gray-700">
        AI Experiment Recommendations
      </h3>
      <div className="flex items-end gap-3">
        <div>
          <label className="mb-1 block text-xs text-gray-500">Namespace</label>
          <input
            value={namespace}
            onChange={(e) => setNamespace(e.target.value)}
            className="w-40 rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
          />
        </div>
        <button
          onClick={handleGenerate}
          disabled={loading}
          className="rounded bg-purple-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-purple-700 disabled:opacity-50"
        >
          {loading ? "Generating..." : "Generate"}
        </button>
      </div>

      {error && <p className="mt-2 text-sm text-red-500">{error}</p>}

      {recommendations && (
        <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {recommendations.map((exp, i) => (
            <div
              key={i}
              className="rounded border border-gray-100 bg-gray-50 p-3"
            >
              <p className="text-sm font-medium">{exp.name}</p>
              <p className="mt-1 text-xs text-gray-500">
                {exp.chaos_type?.replace(/_/g, " ")} | {exp.target_namespace}
              </p>
              {exp.description && (
                <p className="mt-1 text-xs text-gray-400">{exp.description}</p>
              )}
              <button
                onClick={() => handleRun(exp)}
                className="mt-2 rounded border border-blue-300 px-2 py-0.5 text-xs text-blue-600 hover:bg-blue-50"
              >
                Run
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/* ── Section 3: Resilience Trend ── */
function ResilienceTrendSection() {
  const [namespace, setNamespace] = useState("");
  const [days, setDays] = useState(30);

  const { data: trend, loading, error } = useApi(
    () => getResilienceTrend(namespace || undefined, days),
    [namespace, days]
  );

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <h3 className="mb-3 text-sm font-semibold text-gray-700">
        Resilience Score Trend
      </h3>
      <div className="mb-4 flex items-end gap-3">
        <div>
          <label className="mb-1 block text-xs text-gray-500">Namespace</label>
          <input
            value={namespace}
            onChange={(e) => setNamespace(e.target.value)}
            placeholder="all"
            className="w-40 rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs text-gray-500">Period</label>
          <select
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
            className="rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
          >
            <option value={7}>7 days</option>
            <option value={14}>14 days</option>
            <option value={30}>30 days</option>
            <option value={90}>90 days</option>
          </select>
        </div>
      </div>

      {loading && <p className="text-sm text-gray-400">Loading trend...</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}
      {trend && <ResilienceChart data={trend.trend ?? []} />}
      {trend && (
        <p className="mt-2 text-xs text-gray-400">
          {trend.count ?? 0} data points over {trend.period_days} days
        </p>
      )}
    </div>
  );
}

/* ── Main Panel ── */
export default function AnalysisPanel() {
  return (
    <div className="space-y-6">
      <NlExperimentSection />
      <AiRecommendations />
      <ResilienceTrendSection />
    </div>
  );
}
