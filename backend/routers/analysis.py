from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession

from database import get_session
from db_models import AnalysisResultRecord, ExperimentRecord
from engines.ai_engine import AiEngine
from models.experiment import ExperimentConfig, ExperimentResult

router = APIRouter()
ai_engine = AiEngine()


def _record_to_result(rec: ExperimentRecord) -> ExperimentResult:
    """Convert a DB record to an ExperimentResult Pydantic model."""
    from models.experiment import ExperimentPhase, ExperimentStatus

    return ExperimentResult(
        experiment_id=rec.id,
        config=ExperimentConfig(**rec.config),
        status=ExperimentStatus(rec.status),
        phase=ExperimentPhase(rec.phase),
        started_at=rec.started_at,
        completed_at=rec.completed_at,
        steady_state=rec.steady_state,
        hypothesis=rec.hypothesis,
        injection_result=rec.injection_result,
        observations=rec.observations,
        rollback_result=rec.rollback_result,
        error=rec.error,
        ai_insights=rec.ai_insights,
    )


@router.post("/experiment/{experiment_id}")
async def analyze_experiment(
    experiment_id: str,
    session: AsyncSession = Depends(get_session),
):
    """Analyze a completed experiment using AI."""
    rec = await session.get(ExperimentRecord, experiment_id)
    if not rec:
        raise HTTPException(status_code=404, detail="Experiment not found")

    exp = _record_to_result(rec)
    result = await ai_engine.analyze_experiment(
        experiment_data=exp.model_dump(),
        steady_state=exp.steady_state or {},
        observations=exp.observations or {},
    )

    # Persist analysis result
    analysis_rec = AnalysisResultRecord(
        experiment_id=experiment_id,
        severity=result.severity,
        root_cause=result.root_cause,
        confidence=result.confidence,
        recommendations=result.recommendations,
        resilience_score=result.resilience_score,
    )
    session.add(analysis_rec)
    await session.commit()

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


@router.post("/generate-experiments")
async def generate_experiments(request: dict):
    """Generate experiment configs from topology using AI."""
    topology = request.get("topology", {})
    target_namespace = request.get("target_namespace", "default")
    count = request.get("count", 3)

    experiments = await ai_engine.generate_experiments(topology, target_namespace, count)
    return {"experiments": experiments, "count": len(experiments)}
