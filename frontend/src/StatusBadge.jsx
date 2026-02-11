const STATUS_COLORS = {
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  pending: "bg-yellow-100 text-yellow-800",
  failed: "bg-red-100 text-red-800",
  rolled_back: "bg-orange-100 text-orange-800",
  emergency_stopped: "bg-red-200 text-red-900",
  healthy: "bg-green-100 text-green-800",
  degraded: "bg-yellow-100 text-yellow-800",
  unhealthy: "bg-red-100 text-red-800",
  unknown: "bg-gray-100 text-gray-600",
};

const PHASE_COLORS = {
  steady_state: "bg-cyan-100 text-cyan-800",
  hypothesis: "bg-purple-100 text-purple-800",
  inject: "bg-red-100 text-red-800",
  observe: "bg-amber-100 text-amber-800",
  rollback: "bg-indigo-100 text-indigo-800",
};

export default function StatusBadge({ value, type = "status" }) {
  if (!value) return null;
  const colors =
    type === "phase" ? PHASE_COLORS : STATUS_COLORS;
  const cls = colors[value] || "bg-gray-100 text-gray-600";
  return (
    <span
      className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${cls}`}
    >
      {value.replace(/_/g, " ")}
    </span>
  );
}
