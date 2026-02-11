import { useState, useMemo } from "react";
import { listExperiments, rollbackExperiment } from "./api";
import { usePolling } from "./hooks";
import StatusBadge from "./StatusBadge";
import Spinner from "./Spinner";
import ExperimentFilters from "./ExperimentFilters";

const PHASES = ["steady_state", "hypothesis", "inject", "observe", "rollback"];

export function PhaseTimeline({ current }) {
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

const DEFAULT_FILTERS = { search: "", status: "all", chaosType: "all", namespace: "", sort: "newest" };

function applyFilters(experiments, filters) {
  let list = [...experiments];

  if (filters.search) {
    const q = filters.search.toLowerCase();
    list = list.filter(
      (e) =>
        e.experiment_id?.toLowerCase().includes(q) ||
        e.config?.name?.toLowerCase().includes(q)
    );
  }
  if (filters.status !== "all") {
    list = list.filter((e) => e.status === filters.status);
  }
  if (filters.chaosType !== "all") {
    list = list.filter((e) => e.config?.chaos_type === filters.chaosType);
  }
  if (filters.namespace) {
    list = list.filter((e) =>
      e.config?.target_namespace?.toLowerCase().includes(filters.namespace.toLowerCase())
    );
  }

  switch (filters.sort) {
    case "oldest":
      list.sort((a, b) => new Date(a.started_at || 0) - new Date(b.started_at || 0));
      break;
    case "name_asc":
      list.sort((a, b) => (a.config?.name || "").localeCompare(b.config?.name || ""));
      break;
    case "status":
      list.sort((a, b) => (a.status || "").localeCompare(b.status || ""));
      break;
    default: // newest
      list.sort((a, b) => new Date(b.started_at || 0) - new Date(a.started_at || 0));
  }

  return list;
}

export default function ExperimentList({ onSelect }) {
  const [filters, setFilters] = useState(DEFAULT_FILTERS);
  const { data: experiments, error } = usePolling(listExperiments, 5000);

  const filtered = useMemo(
    () => (experiments ? applyFilters(experiments, filters) : []),
    [experiments, filters]
  );

  const handleRollback = async (id, e) => {
    e.stopPropagation();
    if (!confirm(`Rollback experiment ${id}?`)) return;
    await rollbackExperiment(id);
  };

  if (error)
    return <p className="text-sm text-red-500">Failed to load: {error}</p>;
  if (!experiments)
    return (
      <div className="flex items-center justify-center gap-2 py-8 text-gray-400">
        <Spinner size="md" />
        <span className="text-sm">Loading experiments...</span>
      </div>
    );
  if (experiments.length === 0)
    return (
      <p className="text-sm text-gray-500">
        No experiments yet. Create one above.
      </p>
    );

  return (
    <div>
      <ExperimentFilters filters={filters} onChange={setFilters} />
      <p className="mb-2 text-xs text-gray-400">
        Showing {filtered.length} of {experiments.length} experiments
      </p>
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
            {filtered.map((exp) => {
              const id = exp.experiment_id;
              return (
                <tr key={id} className="group">
                  <td colSpan={6} className="p-0">
                    <div
                      onClick={() => onSelect?.(id)}
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
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
