import { useState } from "react";
import { listExperiments, rollbackExperiment } from "./api";
import { usePolling } from "./hooks";
import StatusBadge from "./StatusBadge";

const PHASES = ["steady_state", "hypothesis", "inject", "observe", "rollback"];

function PhaseTimeline({ current }) {
  const idx = PHASES.indexOf(current);
  return (
    <div className="flex items-center gap-1">
      {PHASES.map((p, i) => (
        <div key={p} className="flex items-center">
          <div
            className={`h-3 w-3 rounded-full ${
              i < idx
                ? "bg-green-500"
                : i === idx
                  ? "bg-blue-500 ring-2 ring-blue-200"
                  : "bg-gray-300"
            }`}
            title={p.replace(/_/g, " ")}
          />
          {i < PHASES.length - 1 && (
            <div
              className={`h-0.5 w-4 ${i < idx ? "bg-green-400" : "bg-gray-200"}`}
            />
          )}
        </div>
      ))}
    </div>
  );
}

function JsonToggle({ label, data }) {
  const [open, setOpen] = useState(false);
  if (!data) return null;
  return (
    <div className="mt-2">
      <button
        onClick={() => setOpen((v) => !v)}
        className="text-xs font-medium text-blue-600 hover:underline"
      >
        {open ? "Hide" : "Show"} {label}
      </button>
      {open && (
        <pre className="mt-1 max-h-48 overflow-auto rounded bg-gray-50 p-2 text-xs">
          {JSON.stringify(data, null, 2)}
        </pre>
      )}
    </div>
  );
}

function ExperimentDetail({ exp }) {
  return (
    <div className="border-t border-gray-100 bg-gray-50 px-4 py-3">
      <div className="mb-3 flex items-center gap-4">
        <span className="text-xs font-medium text-gray-500">Phase:</span>
        <PhaseTimeline current={exp.phase} />
        <span className="text-xs text-gray-400">
          {exp.phase?.replace(/_/g, " ")}
        </span>
      </div>

      {exp.config?.description && (
        <p className="mb-2 text-sm text-gray-600">{exp.config.description}</p>
      )}

      {exp.hypothesis && (
        <div className="mb-2 rounded border-l-4 border-purple-400 bg-purple-50 p-2 text-sm">
          <span className="font-medium">Hypothesis:</span> {exp.hypothesis}
        </div>
      )}

      {exp.error && (
        <div className="mb-2 rounded border-l-4 border-red-400 bg-red-50 p-2 text-sm text-red-700">
          {exp.error}
        </div>
      )}

      {exp.ai_insights && (
        <div className="mb-2 rounded border-l-4 border-indigo-400 bg-indigo-50 p-2">
          <span className="text-xs font-medium text-indigo-700">
            AI Insights
          </span>
          <JsonToggle label="details" data={exp.ai_insights} />
        </div>
      )}

      <div className="flex flex-wrap gap-4">
        <JsonToggle label="Steady State" data={exp.steady_state} />
        <JsonToggle label="Injection Result" data={exp.injection_result} />
        <JsonToggle label="Observations" data={exp.observations} />
        <JsonToggle label="Rollback Result" data={exp.rollback_result} />
      </div>

      <div className="mt-2 text-xs text-gray-400">
        Started: {exp.started_at || "-"} | Completed: {exp.completed_at || "-"}
      </div>
    </div>
  );
}

export default function ExperimentList() {
  const [expanded, setExpanded] = useState(null);
  const { data: experiments, error } = usePolling(listExperiments, 5000);

  const handleRollback = async (id, e) => {
    e.stopPropagation();
    if (!confirm(`Rollback experiment ${id}?`)) return;
    await rollbackExperiment(id);
  };

  if (error)
    return <p className="text-sm text-red-500">Failed to load: {error}</p>;
  if (!experiments)
    return <p className="text-sm text-gray-400">Loading experiments...</p>;
  if (experiments.length === 0)
    return (
      <p className="text-sm text-gray-500">
        No experiments yet. Create one above.
      </p>
    );

  return (
    <div className="overflow-hidden rounded-lg border border-gray-200">
      <table className="w-full text-left text-sm">
        <thead className="border-b border-gray-200 bg-gray-50">
          <tr>
            <th className="px-4 py-2.5 font-medium text-gray-600">ID</th>
            <th className="px-4 py-2.5 font-medium text-gray-600">Name</th>
            <th className="px-4 py-2.5 font-medium text-gray-600">Type</th>
            <th className="px-4 py-2.5 font-medium text-gray-600">Status</th>
            <th className="px-4 py-2.5 font-medium text-gray-600">Phase</th>
            <th className="px-4 py-2.5 font-medium text-gray-600">Actions</th>
          </tr>
        </thead>
        <tbody>
          {experiments.map((exp) => {
            const id = exp.experiment_id;
            const isExpanded = expanded === id;
            return (
              <tr key={id} className="group">
                <td colSpan={6} className="p-0">
                  <div
                    onClick={() => setExpanded(isExpanded ? null : id)}
                    className="flex cursor-pointer items-center border-b border-gray-100 hover:bg-gray-50"
                  >
                    <span className="w-24 px-4 py-2.5 font-mono text-xs">
                      {id}
                    </span>
                    <span className="flex-1 px-4 py-2.5">
                      {exp.config?.name}
                    </span>
                    <span className="w-32 px-4 py-2.5 text-gray-500">
                      {exp.config?.chaos_type?.replace(/_/g, " ")}
                    </span>
                    <span className="w-28 px-4 py-2.5">
                      <StatusBadge value={exp.status} />
                    </span>
                    <span className="w-28 px-4 py-2.5">
                      <StatusBadge value={exp.phase} type="phase" />
                    </span>
                    <span className="w-24 px-4 py-2.5">
                      {(exp.status === "running" ||
                        exp.status === "completed") && (
                        <button
                          onClick={(e) => handleRollback(id, e)}
                          className="rounded border border-orange-300 px-2 py-0.5 text-xs text-orange-600 hover:bg-orange-50"
                        >
                          Rollback
                        </button>
                      )}
                    </span>
                  </div>
                  {isExpanded && <ExperimentDetail exp={exp} />}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
