import { useState } from "react";
import {
  getK8sTopology,
  getAwsTopology,
  getCombinedTopology,
  getSteadyState,
} from "./api";
import { useApi } from "./hooks";
import StatusBadge from "./StatusBadge";
import TopologyGraph from "./TopologyGraph";

const RESOURCE_ICONS = {
  pod: "P",
  service: "S",
  deployment: "D",
  node: "N",
  namespace: "NS",
  ec2: "EC2",
  rds: "RDS",
  vpc: "VPC",
  subnet: "Sub",
};

function ResourceCard({ node }) {
  const icon = RESOURCE_ICONS[node.resource_type] || "?";
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-3">
      <div className="mb-2 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="flex h-8 w-8 items-center justify-center rounded bg-gray-100 text-xs font-bold text-gray-600">
            {icon}
          </span>
          <div>
            <p className="text-sm font-medium">{node.name}</p>
            <p className="text-xs text-gray-400">{node.resource_type}</p>
          </div>
        </div>
        <StatusBadge value={node.health} />
      </div>
      {node.namespace && (
        <p className="text-xs text-gray-400">ns: {node.namespace}</p>
      )}
      {node.labels && Object.keys(node.labels).length > 0 && (
        <div className="mt-1 flex flex-wrap gap-1">
          {Object.entries(node.labels)
            .slice(0, 3)
            .map(([k, v]) => (
              <span
                key={k}
                className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500"
              >
                {k}={v}
              </span>
            ))}
        </div>
      )}
    </div>
  );
}

function NodeDetailPanel({ node, onClose }) {
  if (!node) return null;
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-700">{node.name}</h3>
        <button onClick={onClose} className="text-xs text-gray-400 hover:text-gray-600">Close</button>
      </div>
      <div className="space-y-2 text-sm">
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">Type:</span>
          <span>{node.resource_type}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">Health:</span>
          <StatusBadge value={node.health} />
        </div>
        {node.namespace && (
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-500">Namespace:</span>
            <span>{node.namespace}</span>
          </div>
        )}
        {node.labels && Object.keys(node.labels).length > 0 && (
          <div>
            <span className="text-xs text-gray-500">Labels:</span>
            <div className="mt-1 flex flex-wrap gap-1">
              {Object.entries(node.labels).map(([k, v]) => (
                <span key={k} className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500">
                  {k}={v}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default function TopologyView() {
  const [namespace, setNamespace] = useState("default");
  const [source, setSource] = useState("combined");
  const [viewMode, setViewMode] = useState("cards");
  const [selectedNode, setSelectedNode] = useState(null);

  const fetchFn =
    source === "k8s"
      ? () => getK8sTopology(namespace)
      : source === "aws"
        ? getAwsTopology
        : () => getCombinedTopology(namespace);

  const { data: topology, loading, error, reload } = useApi(fetchFn, [
    namespace,
    source,
  ]);
  const { data: steadyState } = useApi(() => getSteadyState(namespace), [
    namespace,
  ]);

  const nodes = topology?.nodes ?? [];
  const grouped = {};
  for (const n of nodes) {
    const type = n.resource_type || "other";
    if (!grouped[type]) grouped[type] = [];
    grouped[type].push(n);
  }

  return (
    <div>
      {/* Controls */}
      <div className="mb-4 flex items-center gap-4">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-600">
            Namespace
          </label>
          <input
            value={namespace}
            onChange={(e) => setNamespace(e.target.value)}
            onBlur={reload}
            onKeyDown={(e) => e.key === "Enter" && reload()}
            className="w-40 rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-600">
            Source
          </label>
          <select
            value={source}
            onChange={(e) => setSource(e.target.value)}
            className="rounded border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
          >
            <option value="combined">Combined</option>
            <option value="k8s">K8s Only</option>
            <option value="aws">AWS Only</option>
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-600">
            View
          </label>
          <div className="flex rounded border border-gray-300">
            <button
              onClick={() => setViewMode("cards")}
              className={`px-3 py-1.5 text-sm ${viewMode === "cards" ? "bg-blue-600 text-white" : "text-gray-600 hover:bg-gray-50"}`}
            >
              Cards
            </button>
            <button
              onClick={() => setViewMode("graph")}
              className={`px-3 py-1.5 text-sm ${viewMode === "graph" ? "bg-blue-600 text-white" : "text-gray-600 hover:bg-gray-50"}`}
            >
              Graph
            </button>
          </div>
        </div>
        <button
          onClick={reload}
          className="mt-5 rounded border border-gray-300 px-3 py-1.5 text-sm hover:bg-gray-50"
        >
          Refresh
        </button>
      </div>

      {loading && <p className="text-sm text-gray-400">Loading topology...</p>}
      {error && <p className="text-sm text-red-500">Error: {error}</p>}

      {/* Graph View */}
      {viewMode === "graph" && !loading && (
        <div className="mb-6">
          <TopologyGraph
            topology={topology}
            onNodeClick={(node) => setSelectedNode(node)}
          />
          {selectedNode && (
            <div className="mt-3">
              <NodeDetailPanel node={selectedNode} onClose={() => setSelectedNode(null)} />
            </div>
          )}
        </div>
      )}

      {/* Cards View */}
      {viewMode === "cards" && (
        <>
          {Object.entries(grouped).map(([type, resources]) => (
            <div key={type} className="mb-6">
              <h3 className="mb-2 text-sm font-semibold capitalize text-gray-700">
                {type}s ({resources.length})
              </h3>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {resources.map((node) => (
                  <ResourceCard key={node.id} node={node} />
                ))}
              </div>
            </div>
          ))}
        </>
      )}

      {!loading && nodes.length === 0 && !error && (
        <p className="text-sm text-gray-400">
          No resources found in namespace "{namespace}"
        </p>
      )}

      {/* Steady State */}
      {steadyState && !steadyState.error && (
        <div className="mt-6 rounded-lg border border-gray-200 bg-white p-4">
          <h3 className="mb-2 text-sm font-semibold text-gray-700">
            Steady State Snapshot
          </h3>
          <pre className="max-h-48 overflow-auto rounded bg-gray-50 p-3 text-xs">
            {JSON.stringify(steadyState, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}
