import { useState } from "react";
import { getExperiment, analyzeExperiment, generateReport, rollbackExperiment } from "./api";
import { useApi } from "./hooks";
import { useToast } from "./ToastContext";
import useSSE from "./useSSE";
import StatusBadge from "./StatusBadge";
import { PhaseTimeline } from "./ExperimentList";
import Spinner from "./Spinner";

const SEVERITY_COLORS = {
  critical: "bg-red-100 text-red-800",
  high: "bg-orange-100 text-orange-800",
  medium: "bg-yellow-100 text-yellow-800",
  low: "bg-green-100 text-green-800",
};

function JsonSection({ label, data, defaultOpen = false }) {
  const [open, setOpen] = useState(defaultOpen);
  if (!data || (typeof data === "object" && Object.keys(data).length === 0)) return null;
  return (
    <div className="rounded border border-gray-200 bg-white">
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center justify-between px-4 py-2 text-left text-sm font-medium text-gray-700 hover:bg-gray-50"
      >
        {label}
        <span className="text-xs text-gray-400">{open ? "Collapse" : "Expand"}</span>
      </button>
      {open && (
        <pre className="max-h-64 overflow-auto border-t border-gray-100 px-4 py-3 text-xs text-gray-600">
          {JSON.stringify(data, null, 2)}
        </pre>
      )}
    </div>
  );
}

function AiAnalysisSection({ experimentId }) {
  const [analysis, setAnalysis] = useState(null);
  const [report, setReport] = useState(null);
  const [analyzing, setAnalyzing] = useState(false);
  const [generating, setGenerating] = useState(false);
  const toast = useToast();

  const handleAnalyze = async () => {
    setAnalyzing(true);
    const res = await analyzeExperiment(experimentId);
    if (res?.error) {
      toast(res.error, "error");
    } else {
      setAnalysis(res);
    }
    setAnalyzing(false);
  };

  const handleReport = async () => {
    setGenerating(true);
    const res = await generateReport({ experiment_id: experimentId });
    if (res?.error) {
      toast(res.error, "error");
    } else {
      setReport(res.report || res);
    }
    setGenerating(false);
  };

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <h3 className="mb-3 text-sm font-semibold text-gray-700">AI Analysis</h3>

      <div className="flex gap-2">
        <button
          onClick={handleAnalyze}
          disabled={analyzing}
          className="inline-flex items-center gap-2 rounded bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          {analyzing && <Spinner size="sm" />}
          {analyzing ? "Analyzing..." : "Analyze with AI"}
        </button>
        {analysis && (
          <button
            onClick={handleReport}
            disabled={generating}
            className="inline-flex items-center gap-2 rounded border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
          >
            {generating && <Spinner size="sm" />}
            Generate Report
          </button>
        )}
      </div>

      {analysis && (
        <div className="mt-4 space-y-3">
          {/* Severity */}
          {analysis.severity && (
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-gray-500">Severity:</span>
              <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${SEVERITY_COLORS[analysis.severity] || "bg-gray-100 text-gray-600"}`}>
                {analysis.severity}
              </span>
            </div>
          )}

          {/* Root Cause */}
          {analysis.root_cause && (
            <div>
              <span className="text-xs font-medium text-gray-500">Root Cause:</span>
              <p className="mt-1 text-sm text-gray-700">{analysis.root_cause}</p>
            </div>
          )}

          {/* Confidence */}
          {analysis.confidence != null && (
            <div>
              <span className="text-xs font-medium text-gray-500">Confidence:</span>
              <div className="mt-1 flex items-center gap-2">
                <div className="h-2 flex-1 rounded-full bg-gray-200">
                  <div
                    className="h-2 rounded-full bg-indigo-500"
                    style={{ width: `${Math.min(analysis.confidence * 100, 100)}%` }}
                  />
                </div>
                <span className="text-xs text-gray-600">
                  {(analysis.confidence * 100).toFixed(0)}%
                </span>
              </div>
            </div>
          )}

          {/* Recommendations */}
          {analysis.recommendations?.length > 0 && (
            <div>
              <span className="text-xs font-medium text-gray-500">Recommendations:</span>
              <ul className="mt-1 list-inside list-disc space-y-1">
                {analysis.recommendations.map((r, i) => (
                  <li key={i} className="text-sm text-gray-700">{r}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      {/* Report */}
      {report && (
        <div className="mt-4 rounded border border-gray-200 bg-gray-50 p-3">
          <h4 className="mb-2 text-xs font-semibold text-gray-600">Report</h4>
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap text-xs text-gray-700">
            {typeof report === "string" ? report : JSON.stringify(report, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

function ProbeResults({ probes }) {
  if (!probes || probes.length === 0) return null;
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-semibold text-gray-700">Probe Results</h3>
      <div className="grid gap-2 sm:grid-cols-2">
        {probes.map((p, i) => (
          <div key={i} className="flex items-center gap-2 rounded border border-gray-200 bg-white p-3">
            <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${p.passed ? "bg-green-100 text-green-800" : "bg-red-100 text-red-800"}`}>
              {p.passed ? "PASS" : "FAIL"}
            </span>
            <div>
              <p className="text-sm font-medium">{p.name}</p>
              <p className="text-xs text-gray-400">{p.type} / {p.mode}</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function ExperimentDetail({ experimentId, onBack }) {
  const { data: initialExp, loading, error, reload } = useApi(() => getExperiment(experimentId), [experimentId]);
  const isRunning = initialExp?.status === "running" || initialExp?.status === "pending";
  const { data: sseExp, connected: sseConnected } = useSSE(isRunning ? experimentId : null);
  const exp = sseExp || initialExp;
  const toast = useToast();

  const handleRollback = async () => {
    if (!confirm(`Rollback experiment ${experimentId}?`)) return;
    const res = await rollbackExperiment(experimentId);
    if (res?.error) {
      toast(res.error, "error");
    } else {
      toast("Rollback initiated", "success");
      reload();
    }
  };

  if (loading)
    return (
      <div className="flex items-center justify-center gap-2 py-12 text-gray-400">
        <Spinner size="lg" />
        <span>Loading experiment...</span>
      </div>
    );
  if (error)
    return <p className="text-sm text-red-500">Error: {error}</p>;
  if (!exp)
    return null;

  const duration =
    exp.started_at && exp.completed_at
      ? ((new Date(exp.completed_at) - new Date(exp.started_at)) / 1000).toFixed(1) + "s"
      : exp.started_at
        ? "In progress..."
        : "-";

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            className="rounded border border-gray-300 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50"
          >
            Back
          </button>
          <h2 className="text-lg font-semibold">{exp.config?.name}</h2>
          <StatusBadge value={exp.status} />
          {sseConnected && (
            <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-green-500" />
              Live
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {(exp.status === "running" || exp.status === "completed") && (
            <button
              onClick={handleRollback}
              className="rounded border border-orange-300 px-3 py-1.5 text-sm text-orange-600 hover:bg-orange-50"
            >
              Rollback
            </button>
          )}
        </div>
      </div>

      {/* Phase Timeline */}
      <div className="flex items-center gap-4 rounded-lg border border-gray-200 bg-white px-4 py-3">
        <span className="text-xs font-medium text-gray-500">Phase:</span>
        <PhaseTimeline current={exp.phase} />
        <span className="text-xs text-gray-400">{exp.phase?.replace(/_/g, " ")}</span>
      </div>

      {/* Config Summary */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div className="rounded border border-gray-200 bg-white p-3">
          <p className="text-xs text-gray-400">Chaos Type</p>
          <p className="text-sm font-medium">{exp.config?.chaos_type?.replace(/_/g, " ")}</p>
        </div>
        <div className="rounded border border-gray-200 bg-white p-3">
          <p className="text-xs text-gray-400">Namespace</p>
          <p className="text-sm font-medium">{exp.config?.target_namespace || "-"}</p>
        </div>
        <div className="rounded border border-gray-200 bg-white p-3">
          <p className="text-xs text-gray-400">Duration</p>
          <p className="text-sm font-medium">{duration}</p>
        </div>
        <div className="rounded border border-gray-200 bg-white p-3">
          <p className="text-xs text-gray-400">ID</p>
          <p className="font-mono text-sm font-medium">{exp.experiment_id}</p>
        </div>
      </div>

      {/* Labels */}
      {exp.config?.target_labels && Object.keys(exp.config.target_labels).length > 0 && (
        <div className="flex flex-wrap gap-1">
          {Object.entries(exp.config.target_labels).map(([k, v]) => (
            <span key={k} className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
              {k}={v}
            </span>
          ))}
        </div>
      )}

      {/* Parameters */}
      {exp.config?.parameters && Object.keys(exp.config.parameters).length > 0 && (
        <div className="rounded border border-gray-200 bg-white p-3">
          <p className="mb-2 text-xs font-medium text-gray-500">Parameters</p>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
            {Object.entries(exp.config.parameters).map(([k, v]) => (
              <div key={k}>
                <span className="text-xs text-gray-400">{k}: </span>
                <span className="text-sm">{JSON.stringify(v)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Error */}
      {exp.error && (
        <div className="rounded border-l-4 border-red-400 bg-red-50 p-3 text-sm text-red-700">
          {exp.error}
        </div>
      )}

      {/* Hypothesis */}
      {exp.hypothesis && (
        <div className="rounded border-l-4 border-purple-400 bg-purple-50 p-3 text-sm">
          <span className="font-medium">Hypothesis:</span> {exp.hypothesis}
        </div>
      )}

      {/* Phase Results */}
      <div className="space-y-2">
        <JsonSection label="Steady State" data={exp.steady_state} />
        <JsonSection label="Injection Result" data={exp.injection_result} />
        <JsonSection label="Observations" data={exp.observations} />
        <JsonSection label="Rollback Result" data={exp.rollback_result} />
        <JsonSection label="AI Insights" data={exp.ai_insights} />
      </div>

      {/* Probe Results */}
      <ProbeResults probes={exp.config?.probes} />

      {/* AI Analysis */}
      <AiAnalysisSection experimentId={exp.experiment_id} />

      {/* Timestamps */}
      <div className="flex gap-4 text-xs text-gray-400">
        <span>Started: {exp.started_at ? new Date(exp.started_at).toLocaleString() : "-"}</span>
        <span>Completed: {exp.completed_at ? new Date(exp.completed_at).toLocaleString() : "-"}</span>
      </div>
    </div>
  );
}
