import { useState } from "react";
import { createExperiment, dryRunExperiment } from "./api";

const CHAOS_TYPES = {
  "K8s": ["pod_delete", "network_latency", "network_loss", "cpu_stress", "memory_stress"],
  "AWS": ["ec2_stop", "rds_failover", "route_blackhole"],
};

const PARAM_FIELDS = {
  network_latency: [{ key: "latency_ms", label: "Latency (ms)", type: "number", placeholder: "100" }],
  network_loss: [{ key: "loss_percent", label: "Loss (%)", type: "number", placeholder: "50" }],
  cpu_stress: [
    { key: "cores", label: "Cores", type: "number", placeholder: "1" },
    { key: "duration_seconds", label: "Duration (s)", type: "number", placeholder: "30" },
  ],
  memory_stress: [
    { key: "memory_bytes", label: "Memory (bytes)", type: "number", placeholder: "134217728" },
    { key: "duration_seconds", label: "Duration (s)", type: "number", placeholder: "30" },
  ],
  ec2_stop: [{ key: "instance_ids", label: "Instance IDs (comma-separated)", type: "text", placeholder: "i-abc123" }],
  rds_failover: [{ key: "db_cluster_id", label: "DB Cluster ID", type: "text", placeholder: "my-cluster" }],
  route_blackhole: [
    { key: "route_table_id", label: "Route Table ID", type: "text", placeholder: "rtb-abc123" },
    { key: "destination_cidr", label: "Destination CIDR", type: "text", placeholder: "10.0.0.0/24" },
  ],
};

const DEFAULT_SAFETY = {
  timeout_seconds: 30,
  max_blast_radius: 0.3,
  dry_run: false,
  require_confirmation: false,
  health_check_interval: 10,
  health_check_failure_threshold: 3,
};

export default function ExperimentForm({ onCreated }) {
  const [name, setName] = useState("");
  const [chaosType, setChaosType] = useState("pod_delete");
  const [namespace, setNamespace] = useState("default");
  const [labels, setLabels] = useState("");
  const [params, setParams] = useState({});
  const [safety, setSafety] = useState(DEFAULT_SAFETY);
  const [aiEnabled, setAiEnabled] = useState(false);
  const [showSafety, setShowSafety] = useState(false);
  const [result, setResult] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  const isK8s = CHAOS_TYPES["K8s"].includes(chaosType);

  const buildConfig = (dryRun = false) => {
    const config = {
      name: name || `${chaosType}-test`,
      chaos_type: chaosType,
      parameters: {},
      safety: { ...safety, dry_run: dryRun },
      ai_enabled: aiEnabled,
    };
    if (isK8s) {
      config.target_namespace = namespace;
      if (labels.trim()) {
        config.target_labels = Object.fromEntries(
          labels.split(",").map((kv) => kv.trim().split("="))
        );
      }
    }
    // Build parameters
    const fields = PARAM_FIELDS[chaosType] || [];
    for (const f of fields) {
      if (params[f.key] !== undefined && params[f.key] !== "") {
        let val = params[f.key];
        if (f.type === "number") val = Number(val);
        if (f.key === "instance_ids") val = val.split(",").map((s) => s.trim());
        config.parameters[f.key] = val;
      }
    }
    return config;
  };

  const handleSubmit = async (dryRun) => {
    setSubmitting(true);
    setResult(null);
    const config = buildConfig(dryRun);
    const res = dryRun
      ? await dryRunExperiment(config)
      : await createExperiment(config);
    setResult(res);
    setSubmitting(false);
    if (!res.error && !dryRun) onCreated?.();
  };

  return (
    <div className="mb-6 rounded-lg border border-gray-200 bg-white p-4">
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        {/* Name */}
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-600">
            Name
          </label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="my-experiment"
            className="w-full rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
          />
        </div>

        {/* Chaos Type */}
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-600">
            Chaos Type
          </label>
          <select
            value={chaosType}
            onChange={(e) => {
              setChaosType(e.target.value);
              setParams({});
            }}
            className="w-full rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
          >
            {Object.entries(CHAOS_TYPES).map(([group, types]) => (
              <optgroup key={group} label={group}>
                {types.map((t) => (
                  <option key={t} value={t}>
                    {t.replace(/_/g, " ")}
                  </option>
                ))}
              </optgroup>
            ))}
          </select>
        </div>

        {/* Namespace (K8s only) */}
        {isK8s && (
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-600">
              Namespace
            </label>
            <input
              value={namespace}
              onChange={(e) => setNamespace(e.target.value)}
              className="w-full rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
            />
          </div>
        )}

        {/* Labels (K8s only) */}
        {isK8s && (
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-600">
              Labels
            </label>
            <input
              value={labels}
              onChange={(e) => setLabels(e.target.value)}
              placeholder="app=nginx,tier=web"
              className="w-full rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
            />
          </div>
        )}
      </div>

      {/* Dynamic Parameters */}
      {(PARAM_FIELDS[chaosType] || []).length > 0 && (
        <div className="mt-3 grid grid-cols-2 gap-4 md:grid-cols-4">
          {PARAM_FIELDS[chaosType].map((f) => (
            <div key={f.key}>
              <label className="mb-1 block text-xs font-medium text-gray-600">
                {f.label}
              </label>
              <input
                type={f.type}
                value={params[f.key] || ""}
                onChange={(e) =>
                  setParams((p) => ({ ...p, [f.key]: e.target.value }))
                }
                placeholder={f.placeholder}
                className="w-full rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
              />
            </div>
          ))}
        </div>
      )}

      {/* AI Toggle + Safety Toggle */}
      <div className="mt-3 flex items-center gap-4">
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={aiEnabled}
            onChange={(e) => setAiEnabled(e.target.checked)}
            className="rounded"
          />
          <span className="text-gray-600">AI Analysis</span>
        </label>
        <button
          onClick={() => setShowSafety((v) => !v)}
          className="text-xs text-gray-500 hover:text-gray-700"
        >
          {showSafety ? "Hide" : "Show"} Safety Settings
        </button>
      </div>

      {/* Safety Settings */}
      {showSafety && (
        <div className="mt-3 grid grid-cols-2 gap-3 rounded border border-gray-100 bg-gray-50 p-3 md:grid-cols-3">
          <div>
            <label className="mb-1 block text-xs text-gray-500">
              Timeout (s)
            </label>
            <input
              type="number"
              value={safety.timeout_seconds}
              onChange={(e) =>
                setSafety((s) => ({
                  ...s,
                  timeout_seconds: Number(e.target.value),
                }))
              }
              min={1}
              max={120}
              className="w-full rounded border border-gray-300 px-2 py-1 text-sm"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs text-gray-500">
              Max Blast Radius
            </label>
            <input
              type="number"
              value={safety.max_blast_radius}
              onChange={(e) =>
                setSafety((s) => ({
                  ...s,
                  max_blast_radius: Number(e.target.value),
                }))
              }
              min={0}
              max={1}
              step={0.1}
              className="w-full rounded border border-gray-300 px-2 py-1 text-sm"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs text-gray-500">
              Health Check Interval (s)
            </label>
            <input
              type="number"
              value={safety.health_check_interval}
              onChange={(e) =>
                setSafety((s) => ({
                  ...s,
                  health_check_interval: Number(e.target.value),
                }))
              }
              min={1}
              max={60}
              className="w-full rounded border border-gray-300 px-2 py-1 text-sm"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs text-gray-500">
              Failure Threshold
            </label>
            <input
              type="number"
              value={safety.health_check_failure_threshold}
              onChange={(e) =>
                setSafety((s) => ({
                  ...s,
                  health_check_failure_threshold: Number(e.target.value),
                }))
              }
              min={1}
              max={10}
              className="w-full rounded border border-gray-300 px-2 py-1 text-sm"
            />
          </div>
          <label className="flex items-center gap-2 text-sm text-gray-500">
            <input
              type="checkbox"
              checked={safety.require_confirmation}
              onChange={(e) =>
                setSafety((s) => ({
                  ...s,
                  require_confirmation: e.target.checked,
                }))
              }
            />
            Require Confirmation
          </label>
        </div>
      )}

      {/* Action Buttons */}
      <div className="mt-4 flex items-center gap-3">
        <button
          onClick={() => handleSubmit(false)}
          disabled={submitting}
          className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {submitting ? "Running..." : "Run Experiment"}
        </button>
        <button
          onClick={() => handleSubmit(true)}
          disabled={submitting}
          className="rounded border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
        >
          Dry Run
        </button>
      </div>

      {/* Result */}
      {result && (
        <div
          className={`mt-3 rounded p-3 text-sm ${
            result.error
              ? "border border-red-200 bg-red-50 text-red-700"
              : "border border-green-200 bg-green-50 text-green-700"
          }`}
        >
          {result.error ? (
            <span>Error: {result.error}</span>
          ) : (
            <span>
              Experiment <strong>{result.experiment_id}</strong> created
              ({result.status})
            </span>
          )}
        </div>
      )}
    </div>
  );
}
