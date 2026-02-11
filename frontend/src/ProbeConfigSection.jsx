const PROBE_TYPES = ["http", "cmd", "k8s", "prometheus"];
const PROBE_MODES = ["sot", "eot", "continuous", "on_chaos"];

const PROBE_FIELDS = {
  http: [
    { key: "url", label: "URL", type: "text", placeholder: "http://service:8080/health" },
    { key: "method", label: "Method", type: "select", options: ["GET", "POST"] },
    { key: "expected_status", label: "Expected Status", type: "number", placeholder: "200" },
    { key: "body_pattern", label: "Body Pattern (regex)", type: "text", placeholder: "" },
  ],
  cmd: [
    { key: "command", label: "Command", type: "text", placeholder: "curl -s localhost:8080" },
    { key: "exit_code", label: "Expected Exit Code", type: "number", placeholder: "0" },
  ],
  k8s: [
    { key: "namespace", label: "Namespace", type: "text", placeholder: "default" },
    { key: "kind", label: "Kind", type: "select", options: ["Pod", "Deployment", "Service", "StatefulSet"] },
    { key: "name", label: "Resource Name", type: "text", placeholder: "my-app" },
  ],
  prometheus: [
    { key: "endpoint", label: "Prometheus Endpoint", type: "text", placeholder: "http://prometheus:9090" },
    { key: "query", label: "PromQL Query", type: "text", placeholder: "up{job=\"my-app\"}" },
    { key: "comparator", label: "Comparator", type: "select", options: [">=", "<=", "==", ">", "<"] },
    { key: "threshold", label: "Threshold", type: "number", placeholder: "1" },
  ],
};

function ProbeCard({ probe, index, onChange, onRemove }) {
  const fields = PROBE_FIELDS[probe.type] || [];

  const update = (key, value) => {
    onChange(index, { ...probe, [key]: value });
  };

  const updateProp = (key, value) => {
    onChange(index, {
      ...probe,
      properties: { ...probe.properties, [key]: value },
    });
  };

  return (
    <div className="rounded border border-gray-200 bg-white p-3">
      <div className="mb-3 flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700">Probe #{index + 1}</span>
        <button
          onClick={() => onRemove(index)}
          className="text-xs text-red-500 hover:text-red-700"
        >
          Remove
        </button>
      </div>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {/* Name */}
        <div>
          <label className="mb-1 block text-xs text-gray-500">Name</label>
          <input
            value={probe.name}
            onChange={(e) => update("name", e.target.value)}
            placeholder="probe-name"
            className="w-full rounded border border-gray-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none"
          />
        </div>

        {/* Type */}
        <div>
          <label className="mb-1 block text-xs text-gray-500">Type</label>
          <select
            value={probe.type}
            onChange={(e) => update("type", e.target.value)}
            className="w-full rounded border border-gray-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none"
          >
            {PROBE_TYPES.map((t) => (
              <option key={t} value={t}>{t.toUpperCase()}</option>
            ))}
          </select>
        </div>

        {/* Mode */}
        <div>
          <label className="mb-1 block text-xs text-gray-500">Mode</label>
          <select
            value={probe.mode}
            onChange={(e) => update("mode", e.target.value)}
            className="w-full rounded border border-gray-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none"
          >
            {PROBE_MODES.map((m) => (
              <option key={m} value={m}>{m.toUpperCase()}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Dynamic properties */}
      {fields.length > 0 && (
        <div className="mt-3 grid grid-cols-2 gap-3 md:grid-cols-4">
          {fields.map((f) => (
            <div key={f.key}>
              <label className="mb-1 block text-xs text-gray-500">{f.label}</label>
              {f.type === "select" ? (
                <select
                  value={probe.properties?.[f.key] || f.options[0]}
                  onChange={(e) => updateProp(f.key, e.target.value)}
                  className="w-full rounded border border-gray-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none"
                >
                  {f.options.map((o) => (
                    <option key={o} value={o}>{o}</option>
                  ))}
                </select>
              ) : (
                <input
                  type={f.type}
                  value={probe.properties?.[f.key] ?? ""}
                  onChange={(e) =>
                    updateProp(f.key, f.type === "number" ? Number(e.target.value) : e.target.value)
                  }
                  placeholder={f.placeholder}
                  className="w-full rounded border border-gray-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none"
                />
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

const DEFAULT_PROBE = { name: "", type: "http", mode: "sot", properties: {} };

export default function ProbeConfigSection({ probes, onChange }) {
  const addProbe = () => onChange([...probes, { ...DEFAULT_PROBE }]);
  const removeProbe = (index) => onChange(probes.filter((_, i) => i !== index));
  const updateProbe = (index, updated) =>
    onChange(probes.map((p, i) => (i === index ? updated : p)));

  return (
    <div className="space-y-3">
      {probes.map((probe, i) => (
        <ProbeCard
          key={i}
          probe={probe}
          index={i}
          onChange={updateProbe}
          onRemove={removeProbe}
        />
      ))}
      <button
        onClick={addProbe}
        className="rounded border border-dashed border-gray-300 px-3 py-1.5 text-sm text-gray-500 hover:border-blue-400 hover:text-blue-600"
      >
        + Add Probe
      </button>
    </div>
  );
}
