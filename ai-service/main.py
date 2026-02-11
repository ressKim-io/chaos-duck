"""ChaosDuck AI Analysis Microservice.

Standalone FastAPI service that handles all AI-powered analysis.
Called by the Go backend via HTTP proxy.
"""

import logging

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware

from ai_engine import AiEngine

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(
    title="ChaosDuck AI Service",
    description="AI-powered chaos engineering analysis",
    version="0.1.0",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

ai_engine = AiEngine()


@app.get("/health")
async def health_check():
    return {"status": "healthy", "service": "ai-service"}


@app.post("/analyze")
async def analyze_experiment(request: dict):
    """Analyze a completed experiment using AI."""
    experiment_data = request.get("experiment_data", {})
    steady_state = request.get("steady_state", {})
    observations = request.get("observations", {})

    result = await ai_engine.analyze_experiment(experiment_data, steady_state, observations)
    return result.model_dump()


@app.post("/hypotheses")
async def generate_hypotheses(request: dict):
    """Generate failure hypotheses for a target."""
    topology = request.get("topology", {})
    target = request.get("target", "")
    chaos_type = request.get("chaos_type", "")

    hypothesis = await ai_engine.generate_hypothesis(topology, target, chaos_type)
    return {"hypothesis": hypothesis}


@app.post("/resilience-score")
async def calculate_resilience_score(request: dict):
    """Calculate resilience score from experiment history."""
    experiments_data = request.get("experiments", [])
    score = await ai_engine.calculate_resilience_score(experiments_data)
    return score


@app.post("/report")
async def generate_report(request: dict):
    """Generate a human-readable experiment report."""
    experiment_data = request.get("experiment", {})
    analysis = request.get("analysis")
    report = await ai_engine.generate_report(experiment_data, analysis)
    return {"report": report}


@app.post("/generate-experiments")
async def generate_experiments(request: dict):
    """Generate experiment configs from topology using AI."""
    topology = request.get("topology", {})
    target_namespace = request.get("target_namespace", "default")
    count = request.get("count", 3)

    experiments = await ai_engine.generate_experiments(topology, target_namespace, count)
    return {"experiments": experiments, "count": len(experiments)}


@app.post("/nl-experiment")
async def nl_experiment(request: dict):
    """Convert natural language to ExperimentConfig."""
    text = request.get("text", "").strip()
    if not text:
        raise HTTPException(status_code=400, detail="Text is required")

    topology = request.get("topology")
    config = await ai_engine.parse_natural_language(text, topology)
    return config


@app.post("/review-steady-state")
async def review_steady_state(request: dict):
    """Review steady state for anomalies."""
    steady_state = request.get("steady_state", {})
    result = await ai_engine.review_steady_state(steady_state)
    return result


@app.post("/compare-observations")
async def compare_observations(request: dict):
    """Compare observations against steady state."""
    steady_state = request.get("steady_state", {})
    observations = request.get("observations", {})
    hypothesis = request.get("hypothesis")
    result = await ai_engine.compare_observations(steady_state, observations, hypothesis)
    return result


@app.post("/verify-recovery")
async def verify_recovery(request: dict):
    """Verify recovery completeness after rollback."""
    original_state = request.get("original_state", {})
    current_state = request.get("current_state", {})
    result = await ai_engine.verify_recovery(original_state, current_state)
    return result
