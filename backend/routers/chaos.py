import uuid
from datetime import UTC, datetime

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from database import get_session
from db_models import ExperimentRecord
from engines.aws_engine import AwsEngine
from engines.k8s_engine import K8sEngine
from models.experiment import (
    ExperimentConfig,
    ExperimentPhase,
    ExperimentResult,
    ExperimentStatus,
)
from safety.guardrails import ExperimentContext, emergency_stop_manager
from safety.rollback import rollback_manager

router = APIRouter()

k8s_engine = K8sEngine()
aws_engine = AwsEngine()


def _record_to_result(rec: ExperimentRecord) -> ExperimentResult:
    """Convert a DB record to an ExperimentResult Pydantic model."""
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
    )


@router.post("/experiments", response_model=ExperimentResult)
async def create_experiment(
    config: ExperimentConfig,
    session: AsyncSession = Depends(get_session),
):
    """Create and run a chaos experiment."""
    if emergency_stop_manager.is_triggered():
        raise HTTPException(status_code=503, detail="Emergency stop is active")

    experiment_id = str(uuid.uuid4())[:8]
    now = datetime.now(UTC)

    rec = ExperimentRecord(
        id=experiment_id,
        config=config.model_dump(),
        status=ExperimentStatus.RUNNING.value,
        phase=ExperimentPhase.STEADY_STATE.value,
        started_at=now,
    )
    session.add(rec)
    await session.commit()

    async with ExperimentContext(experiment_id, config):
        try:
            # Phase 1: Steady State
            if config.target_namespace:
                rec.steady_state = await k8s_engine.get_steady_state(config.target_namespace)

            # Phase 2: Inject
            rec.phase = ExperimentPhase.INJECT.value
            chaos_fn = _get_chaos_function(config)
            injection_result, rollback_fn = await chaos_fn(config)
            rec.injection_result = injection_result

            if rollback_fn:
                rollback_manager.push(experiment_id, rollback_fn, f"{config.chaos_type.value}")

            # Phase 3: Observe
            rec.phase = ExperimentPhase.OBSERVE.value
            if config.target_namespace:
                rec.observations = await k8s_engine.get_steady_state(config.target_namespace)

            rec.status = ExperimentStatus.COMPLETED.value
            rec.phase = ExperimentPhase.ROLLBACK.value
            rec.completed_at = datetime.now(UTC)

        except Exception as e:
            rec.status = ExperimentStatus.FAILED.value
            rec.error = str(e)
            await session.commit()
            raise

    await session.commit()
    return _record_to_result(rec)


@router.get("/experiments")
async def list_experiments(session: AsyncSession = Depends(get_session)):
    """List all experiments."""
    result = await session.execute(
        select(ExperimentRecord).order_by(ExperimentRecord.started_at.desc())
    )
    records = result.scalars().all()
    return [_record_to_result(r) for r in records]


@router.get("/experiments/{experiment_id}", response_model=ExperimentResult)
async def get_experiment(
    experiment_id: str,
    session: AsyncSession = Depends(get_session),
):
    """Get a specific experiment."""
    rec = await session.get(ExperimentRecord, experiment_id)
    if not rec:
        raise HTTPException(status_code=404, detail="Experiment not found")
    return _record_to_result(rec)


@router.post("/experiments/{experiment_id}/rollback")
async def rollback_experiment(
    experiment_id: str,
    session: AsyncSession = Depends(get_session),
):
    """Rollback a specific experiment."""
    rec = await session.get(ExperimentRecord, experiment_id)
    if not rec:
        raise HTTPException(status_code=404, detail="Experiment not found")

    results = await rollback_manager.rollback(experiment_id)
    rec.status = ExperimentStatus.ROLLED_BACK.value
    await session.commit()
    return {"experiment_id": experiment_id, "rollback_results": results}


@router.post("/dry-run", response_model=ExperimentResult)
async def dry_run(config: ExperimentConfig):
    """Execute a dry-run of a chaos experiment."""
    config.safety.dry_run = True
    experiment_id = f"dry-{str(uuid.uuid4())[:8]}"
    now = datetime.now(UTC)
    result = ExperimentResult(
        experiment_id=experiment_id,
        config=config,
        status=ExperimentStatus.COMPLETED,
        started_at=now,
        completed_at=now,
    )

    chaos_fn = _get_chaos_function(config)
    injection_result, _ = await chaos_fn(config)
    result.injection_result = injection_result
    return result


def _get_chaos_function(config: ExperimentConfig):
    """Route to the appropriate chaos function based on type."""
    from models.experiment import ChaosType

    k8s_types = {
        ChaosType.POD_DELETE: _run_pod_delete,
        ChaosType.NETWORK_LATENCY: _run_network_latency,
        ChaosType.NETWORK_LOSS: _run_network_loss,
        ChaosType.CPU_STRESS: _run_cpu_stress,
        ChaosType.MEMORY_STRESS: _run_memory_stress,
    }
    aws_types = {
        ChaosType.EC2_STOP: _run_ec2_stop,
        ChaosType.RDS_FAILOVER: _run_rds_failover,
        ChaosType.ROUTE_BLACKHOLE: _run_route_blackhole,
    }

    if config.chaos_type in k8s_types:
        return k8s_types[config.chaos_type]
    if config.chaos_type in aws_types:
        return aws_types[config.chaos_type]

    raise HTTPException(status_code=400, detail=f"Unknown chaos type: {config.chaos_type}")


async def _run_pod_delete(config: ExperimentConfig):
    label_selector = ",".join(f"{k}={v}" for k, v in (config.target_labels or {}).items())
    return await k8s_engine.pod_delete(
        config.target_namespace or "default",
        label_selector,
        config=config,
        dry_run=config.safety.dry_run,
    )


async def _run_network_latency(config: ExperimentConfig):
    label_selector = ",".join(f"{k}={v}" for k, v in (config.target_labels or {}).items())
    return await k8s_engine.network_latency(
        config.target_namespace or "default",
        label_selector,
        latency_ms=config.parameters.get("latency_ms", 100),
        config=config,
        dry_run=config.safety.dry_run,
    )


async def _run_network_loss(config: ExperimentConfig):
    label_selector = ",".join(f"{k}={v}" for k, v in (config.target_labels or {}).items())
    return await k8s_engine.network_loss(
        config.target_namespace or "default",
        label_selector,
        loss_percent=config.parameters.get("loss_percent", 10),
        config=config,
        dry_run=config.safety.dry_run,
    )


async def _run_cpu_stress(config: ExperimentConfig):
    label_selector = ",".join(f"{k}={v}" for k, v in (config.target_labels or {}).items())
    return await k8s_engine.cpu_stress(
        config.target_namespace or "default",
        label_selector,
        cores=config.parameters.get("cores", 1),
        duration_seconds=config.safety.timeout_seconds,
        config=config,
        dry_run=config.safety.dry_run,
    )


async def _run_memory_stress(config: ExperimentConfig):
    label_selector = ",".join(f"{k}={v}" for k, v in (config.target_labels or {}).items())
    return await k8s_engine.memory_stress(
        config.target_namespace or "default",
        label_selector,
        memory_bytes=config.parameters.get("memory_bytes", "256M"),
        duration_seconds=config.safety.timeout_seconds,
        config=config,
        dry_run=config.safety.dry_run,
    )


async def _run_ec2_stop(config: ExperimentConfig):
    instance_ids = config.parameters.get("instance_ids", [])
    return await aws_engine.stop_ec2(
        instance_ids,
        dry_run=config.safety.dry_run,
    )


async def _run_rds_failover(config: ExperimentConfig):
    db_cluster_id = config.parameters.get("db_cluster_id", "")
    return await aws_engine.failover_rds(
        db_cluster_id,
        dry_run=config.safety.dry_run,
    )


async def _run_route_blackhole(config: ExperimentConfig):
    return await aws_engine.blackhole_route(
        route_table_id=config.parameters.get("route_table_id", ""),
        destination_cidr=config.parameters.get("destination_cidr", ""),
        dry_run=config.safety.dry_run,
    )
