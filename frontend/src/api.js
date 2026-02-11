const BASE = "";

async function request(method, path, body) {
  try {
    const opts = {
      method,
      headers: body ? { "Content-Type": "application/json" } : {},
    };
    if (body) opts.body = JSON.stringify(body);
    const res = await fetch(`${BASE}${path}`, opts);
    if (!res.ok) {
      const err = await res.json().catch(() => ({ detail: res.statusText }));
      return { error: err.detail || res.statusText };
    }
    return res.json();
  } catch (e) {
    return { error: e.message };
  }
}

const get = (path) => request("GET", path);
const post = (path, body) => request("POST", path, body);

// Health & System
export const fetchHealth = () => get("/health");
export const triggerEmergencyStop = () => post("/emergency-stop");

// Experiments
export const listExperiments = () => get("/api/chaos/experiments");
export const getExperiment = (id) => get(`/api/chaos/experiments/${id}`);
export const createExperiment = (config) =>
  post("/api/chaos/experiments", config);
export const rollbackExperiment = (id) =>
  post(`/api/chaos/experiments/${id}/rollback`);
export const dryRunExperiment = (config) =>
  post("/api/chaos/dry-run", config);

// Topology
export const getK8sTopology = (ns = "default") =>
  get(`/api/topology/k8s?namespace=${ns}`);
export const getAwsTopology = () => get("/api/topology/aws");
export const getCombinedTopology = (ns = "default") =>
  get(`/api/topology/combined?namespace=${ns}`);
export const getSteadyState = (ns = "default") =>
  get(`/api/topology/steady-state?namespace=${ns}`);

// Analysis & AI
export const analyzeExperiment = (id) =>
  post(`/api/analysis/experiment/${id}`);
export const generateHypotheses = (data) =>
  post("/api/analysis/hypotheses", data);
export const calcResilienceScore = (data) =>
  post("/api/analysis/resilience-score", data);
export const generateReport = (data) =>
  post("/api/analysis/report", data);
export const generateExperiments = (data) =>
  post("/api/analysis/generate-experiments", data);
export const nlExperiment = (data) =>
  post("/api/analysis/nl-experiment", data);
export const getResilienceTrend = (ns, days = 30) => {
  const params = new URLSearchParams({ days });
  if (ns) params.set("namespace", ns);
  return get(`/api/analysis/resilience-trend?${params}`);
};
export const getResilienceTrendSummary = (ns, days = 30) => {
  const params = new URLSearchParams({ days });
  if (ns) params.set("namespace", ns);
  return get(`/api/analysis/resilience-trend/summary?${params}`);
};
