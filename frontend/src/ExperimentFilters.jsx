const STATUSES = ["all", "running", "completed", "failed", "rolled_back", "emergency_stopped", "pending"];
const CHAOS_TYPES = ["all", "pod_delete", "network_latency", "network_loss", "cpu_stress", "memory_stress", "ec2_stop", "rds_failover", "route_blackhole"];
const SORT_OPTIONS = [
  { value: "newest", label: "Newest first" },
  { value: "oldest", label: "Oldest first" },
  { value: "name_asc", label: "Name A-Z" },
  { value: "status", label: "By status" },
];

export default function ExperimentFilters({ filters, onChange }) {
  const set = (key, value) => onChange({ ...filters, [key]: value });

  return (
    <div className="mb-4 flex flex-wrap items-end gap-3">
      {/* Search */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Search</label>
        <input
          value={filters.search}
          onChange={(e) => set("search", e.target.value)}
          placeholder="Name or ID..."
          className="w-44 rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
        />
      </div>

      {/* Status */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Status</label>
        <select
          value={filters.status}
          onChange={(e) => set("status", e.target.value)}
          className="rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
        >
          {STATUSES.map((s) => (
            <option key={s} value={s}>{s === "all" ? "All statuses" : s.replace(/_/g, " ")}</option>
          ))}
        </select>
      </div>

      {/* Chaos Type */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Type</label>
        <select
          value={filters.chaosType}
          onChange={(e) => set("chaosType", e.target.value)}
          className="rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
        >
          {CHAOS_TYPES.map((t) => (
            <option key={t} value={t}>{t === "all" ? "All types" : t.replace(/_/g, " ")}</option>
          ))}
        </select>
      </div>

      {/* Namespace */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Namespace</label>
        <input
          value={filters.namespace}
          onChange={(e) => set("namespace", e.target.value)}
          placeholder="Any"
          className="w-28 rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
        />
      </div>

      {/* Sort */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Sort</label>
        <select
          value={filters.sort}
          onChange={(e) => set("sort", e.target.value)}
          className="rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
        >
          {SORT_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      </div>

      {/* Reset */}
      <button
        onClick={() => onChange({ search: "", status: "all", chaosType: "all", namespace: "", sort: "newest" })}
        className="rounded border border-gray-300 px-3 py-1.5 text-xs text-gray-500 hover:bg-gray-50"
      >
        Reset
      </button>
    </div>
  );
}
