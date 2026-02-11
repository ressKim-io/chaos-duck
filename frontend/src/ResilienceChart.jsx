import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";

export default function ResilienceChart({ data }) {
  if (!data || data.length === 0) {
    return (
      <p className="py-8 text-center text-sm text-gray-400">
        No trend data available
      </p>
    );
  }

  const chartData = data.map((d) => ({
    date: new Date(d.created_at).toLocaleDateString(),
    score: d.resilience_score ?? 0,
    severity: d.severity,
    id: d.experiment_id,
  }));

  return (
    <ResponsiveContainer width="100%" height={280}>
      <LineChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
        <XAxis
          dataKey="date"
          tick={{ fontSize: 11, fill: "#6b7280" }}
          tickLine={false}
        />
        <YAxis
          domain={[0, 100]}
          tick={{ fontSize: 11, fill: "#6b7280" }}
          tickLine={false}
          label={{
            value: "Score",
            angle: -90,
            position: "insideLeft",
            style: { fontSize: 11, fill: "#9ca3af" },
          }}
        />
        <Tooltip
          formatter={(val) => [`${val.toFixed(1)}`, "Resilience Score"]}
          labelFormatter={(label) => `Date: ${label}`}
          contentStyle={{
            fontSize: 12,
            borderRadius: 8,
            border: "1px solid #e5e7eb",
          }}
        />
        <Line
          type="monotone"
          dataKey="score"
          stroke="#3b82f6"
          strokeWidth={2}
          dot={{ fill: "#3b82f6", r: 3 }}
          activeDot={{ r: 5 }}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
