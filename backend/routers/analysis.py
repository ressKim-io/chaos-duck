from datetime import UTC, datetime, timedelta

from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy import select
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


@router.post("/nl-experiment")
async def nl_experiment(request: dict):
    """Convert natural language to ExperimentConfig."""
    text = request.get("text", "").strip()
    if not text:
        raise HTTPException(status_code=400, detail="Text is required")

    topology = request.get("topology")
    config = await ai_engine.parse_natural_language(text, topology)
    return config


@router.get("/resilience-trend")
async def resilience_trend(
    namespace: str | None = Query(default=None),
    days: int = Query(default=30, ge=1, le=365),
    session: AsyncSession = Depends(get_session),
):
    """Get resilience score trend from analysis history."""
    since = datetime.now(UTC) - timedelta(days=days)

    query = select(AnalysisResultRecord).where(AnalysisResultRecord.created_at >= since)

    if namespace:
        # Join with experiments to filter by namespace in config
        query = (
            select(AnalysisResultRecord)
            .join(ExperimentRecord, AnalysisResultRecord.experiment_id == ExperimentRecord.id)
            .where(
                AnalysisResultRecord.created_at >= since,
                ExperimentRecord.config["target_namespace"].as_string() == namespace,
            )
        )

    query = query.order_by(AnalysisResultRecord.created_at.asc())
    result = await session.execute(query)
    records = result.scalars().all()

    return {
        "trend": [
            {
                "experiment_id": r.experiment_id,
                "resilience_score": r.resilience_score,
                "severity": r.severity,
                "created_at": r.created_at.isoformat() if r.created_at else None,
            }
            for r in records
        ],
        "count": len(records),
        "period_days": days,
        "namespace": namespace,
    }


@router.get("/resilience-trend/summary")
async def resilience_trend_summary(
    namespace: str | None = Query(default=None),
    days: int = Query(default=30, ge=1, le=365),
    session: AsyncSession = Depends(get_session),
):
    """Get AI-generated summary of resilience score trend."""
    since = datetime.now(UTC) - timedelta(days=days)

    query = (
        select(AnalysisResultRecord)
        .where(AnalysisResultRecord.created_at >= since)
        .order_by(AnalysisResultRecord.created_at.asc())
    )
    result = await session.execute(query)
    records = result.scalars().all()

    experiments_data = [
        {
            "experiment_id": r.experiment_id,
            "resilience_score": r.resilience_score,
            "severity": r.severity,
            "root_cause": r.root_cause,
        }
        for r in records
    ]

    summary = await ai_engine.calculate_resilience_score(experiments_data)
    return {
        "summary": summary,
        "data_points": len(records),
        "period_days": days,
    }
