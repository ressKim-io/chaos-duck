from fastapi import APIRouter, HTTPException

from engines.ai_engine import AiEngine

router = APIRouter()
ai_engine = AiEngine()


@router.post("/experiment/{experiment_id}")
async def analyze_experiment(experiment_id: str):
    """Analyze a completed experiment using AI."""
    # Import here to avoid circular dependency
    from routers.chaos import experiments

    if experiment_id not in experiments:
        raise HTTPException(status_code=404, detail="Experiment not found")

    exp = experiments[experiment_id]
    result = await ai_engine.analyze_experiment(
        experiment_data=exp.model_dump(),
        steady_state=exp.steady_state or {},
        observations=exp.observations or {},
    )
    return result.model_dump()


@router.post("/hypotheses")
async def generate_hypotheses(request: dict):
    """Generate failure hypotheses for a target."""
    topology = request.get("topology", {})
    target = request.get("target", "")
    chaos_type = request.get("chaos_type", "")

    hypothesis = await ai_engine.generate_hypothesis(topology, target, chaos_type)
    return {"hypothesis": hypothesis}


@router.post("/resilience-score")
async def calculate_resilience_score(request: dict):
    """Calculate resilience score from experiment history."""
    experiments_data = request.get("experiments", [])
    score = await ai_engine.calculate_resilience_score(experiments_data)
    return score


@router.post("/report")
async def generate_report(request: dict):
    """Generate a human-readable experiment report."""
    experiment_data = request.get("experiment", {})
    analysis = request.get("analysis")
    report = await ai_engine.generate_report(experiment_data, analysis)
    return {"report": report}
