import { useCallback, useMemo, useRef } from "react";
import ForceGraph2D from "react-force-graph-2d";

const HEALTH_COLORS = {
  healthy: "#22c55e",
  degraded: "#eab308",
  unhealthy: "#ef4444",
  unknown: "#9ca3af",
};

const TYPE_SIZES = {
  node: 8,
  namespace: 7,
  deployment: 6,
  service: 5,
  pod: 4,
  ec2: 6,
  rds: 6,
  vpc: 8,
  subnet: 5,
};

export default function TopologyGraph({ topology, onNodeClick }) {
  const fgRef = useRef();

  const graphData = useMemo(() => {
    if (!topology) return { nodes: [], links: [] };

    const nodes = (topology.nodes || []).map((n) => ({
      id: n.id,
      name: n.name,
      resource_type: n.resource_type,
      health: n.health,
      namespace: n.namespace,
      labels: n.labels,
      val: TYPE_SIZES[n.resource_type] || 4,
    }));

    const nodeIds = new Set(nodes.map((n) => n.id));
    const links = (topology.edges || [])
      .filter((e) => nodeIds.has(e.source) && nodeIds.has(e.target))
      .map((e) => ({
        source: e.source,
        target: e.target,
        relationship: e.relationship,
      }));

    return { nodes, links };
  }, [topology]);

  const nodeCanvasObject = useCallback((node, ctx, globalScale) => {
    const size = node.val || 4;
    const color = HEALTH_COLORS[node.health] || HEALTH_COLORS.unknown;
    const fontSize = Math.max(10 / globalScale, 2);
    const label = node.name?.length > 16 ? node.name.slice(0, 15) + "..." : node.name;

    // Draw node shape based on resource type
    ctx.beginPath();
    if (node.resource_type === "service" || node.resource_type === "vpc") {
      // Diamond
      ctx.moveTo(node.x, node.y - size);
      ctx.lineTo(node.x + size, node.y);
      ctx.lineTo(node.x, node.y + size);
      ctx.lineTo(node.x - size, node.y);
      ctx.closePath();
    } else if (node.resource_type === "pod" || node.resource_type === "ec2") {
      // Circle
      ctx.arc(node.x, node.y, size, 0, 2 * Math.PI);
    } else {
      // Rounded rect
      const r = 2;
      ctx.moveTo(node.x - size + r, node.y - size);
      ctx.lineTo(node.x + size - r, node.y - size);
      ctx.quadraticCurveTo(node.x + size, node.y - size, node.x + size, node.y - size + r);
      ctx.lineTo(node.x + size, node.y + size - r);
      ctx.quadraticCurveTo(node.x + size, node.y + size, node.x + size - r, node.y + size);
      ctx.lineTo(node.x - size + r, node.y + size);
      ctx.quadraticCurveTo(node.x - size, node.y + size, node.x - size, node.y + size - r);
      ctx.lineTo(node.x - size, node.y - size + r);
      ctx.quadraticCurveTo(node.x - size, node.y - size, node.x - size + r, node.y - size);
      ctx.closePath();
    }
    ctx.fillStyle = color;
    ctx.fill();
    ctx.strokeStyle = "#fff";
    ctx.lineWidth = 1;
    ctx.stroke();

    // Label
    ctx.font = `${fontSize}px sans-serif`;
    ctx.textAlign = "center";
    ctx.textBaseline = "top";
    ctx.fillStyle = "#374151";
    ctx.fillText(label || "", node.x, node.y + size + 2);
  }, []);

  const handleNodeClick = useCallback((node) => {
    onNodeClick?.(node);
  }, [onNodeClick]);

  if (graphData.nodes.length === 0) {
    return (
      <p className="py-8 text-center text-sm text-gray-400">
        No topology data for graph view
      </p>
    );
  }

  return (
    <div className="overflow-hidden rounded-lg border border-gray-200 bg-white">
      <ForceGraph2D
        ref={fgRef}
        graphData={graphData}
        width={900}
        height={500}
        nodeCanvasObject={nodeCanvasObject}
        onNodeClick={handleNodeClick}
        linkDirectionalArrowLength={4}
        linkDirectionalArrowRelPos={1}
        linkColor={() => "#d1d5db"}
        linkWidth={1}
        cooldownTicks={100}
        enableZoomInteraction={true}
        enablePanInteraction={true}
      />
    </div>
  );
}
