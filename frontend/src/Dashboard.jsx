import { listExperiments, fetchHealth } from "./api";
import { useApi } from "./hooks";
import StatusBadge from "./StatusBadge";

function StatCard({ label, value, color = "text-gray-900" }) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <p className="text-xs font-medium text-gray-500">{label}</p>
      <p className={`mt-1 text-2xl font-bold ${color}`}>{value}</p>
    </div>
  );
}

export default function Dashboard({ onNavigate }) {
  const { data: experiments } = useApi(listExperiments);
  const { data: health } = useApi(fetchHealth);

  const total = experiments?.length ?? 0;
  const running =
    experiments?.filter((e) => e.status === "running").length ?? 0;
  const completed =
    experiments?.filter((e) => e.status === "completed").length ?? 0;
  const failed =
    experiments?.filter(
      (e) => e.status === "failed" || e.status === "rolled_back"
    ).length ?? 0;
  const successRate =
    total > 0 ? Math.round((completed / total) * 100) : 0;
  const recent = experiments?.slice(0, 5) ?? [];

  return (
    <div>
      {/* Summary Cards */}
      <div className="mb-6 grid grid-cols-2 gap-4 md:grid-cols-4">
        <StatCard label="Total Experiments" value={total} />
        <StatCard
          label="Running"
          value={running}
          color={running > 0 ? "text-blue-600" : "text-gray-900"}
        />
        <StatCard
          label="Success Rate"
          value={`${successRate}%`}
          color={successRate >= 80 ? "text-green-600" : "text-amber-600"}
        />
        <StatCard
          label="System"
          value={health?.status ?? "unknown"}
          color={
            health?.status === "healthy" ? "text-green-600" : "text-red-600"
          }
        />
      </div>

      {/* Quick Actions */}
      <div className="mb-6 flex gap-3">
        <button
          onClick={() => onNavigate("Experiments")}
          className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          + New Experiment
        </button>
        <button
          onClick={() => onNavigate("Topology")}
          className="rounded border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
        >
          View Topology
        </button>
        <button
          onClick={() => onNavigate("Analysis")}
          className="rounded border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
        >
          AI Analysis
        </button>
      </div>

      {/* Recent Experiments */}
      <div className="rounded-lg border border-gray-200 bg-white">
        <h3 className="border-b border-gray-200 px-4 py-3 text-sm font-semibold text-gray-700">
          Recent Experiments
        </h3>
        {recent.length === 0 ? (
          <p className="px-4 py-6 text-center text-sm text-gray-400">
            No experiments yet
          </p>
        ) : (
          <table className="w-full text-left text-sm">
            <thead className="border-b border-gray-100 bg-gray-50 text-xs text-gray-500">
              <tr>
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">Type</th>
                <th className="px-4 py-2">Status</th>
                <th className="px-4 py-2">Phase</th>
                <th className="px-4 py-2">Time</th>
              </tr>
            </thead>
            <tbody>
              {recent.map((exp) => (
                <tr
                  key={exp.experiment_id}
                  className="border-b border-gray-50 hover:bg-gray-50"
                >
                  <td className="px-4 py-2 font-medium">
                    {exp.config?.name}
                  </td>
                  <td className="px-4 py-2 text-gray-500">
                    {exp.config?.chaos_type?.replace(/_/g, " ")}
                  </td>
                  <td className="px-4 py-2">
                    <StatusBadge value={exp.status} />
                  </td>
                  <td className="px-4 py-2">
                    <StatusBadge value={exp.phase} type="phase" />
                  </td>
                  <td className="px-4 py-2 text-xs text-gray-400">
                    {exp.started_at
                      ? new Date(exp.started_at).toLocaleString()
                      : "-"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Failed Count */}
      {failed > 0 && (
        <p className="mt-4 text-sm text-red-500">
          {failed} experiment(s) failed or rolled back
        </p>
      )}
    </div>
  );
}
