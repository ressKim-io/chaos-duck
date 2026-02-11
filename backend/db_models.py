import uuid
from datetime import UTC, datetime

from sqlalchemy import JSON, Boolean, DateTime, Float, Integer, String, Text
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


def _generate_short_id() -> str:
    return str(uuid.uuid4())[:8]


class ExperimentRecord(Base):
    """Persistent experiment record."""

    __tablename__ = "experiments"

    id: Mapped[str] = mapped_column(String(8), primary_key=True, default=_generate_short_id)
    config: Mapped[dict] = mapped_column(JSON, nullable=False)
    status: Mapped[str] = mapped_column(String(30), default="pending")
    phase: Mapped[str] = mapped_column(String(30), default="steady_state")
    started_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    completed_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    steady_state: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    hypothesis: Mapped[str | None] = mapped_column(Text, nullable=True)
    injection_result: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    observations: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    rollback_result: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    error: Mapped[str | None] = mapped_column(Text, nullable=True)
    ai_insights: Mapped[dict | None] = mapped_column(JSON, nullable=True)


class SnapshotRecord(Base):
    """Persistent snapshot record."""

    __tablename__ = "snapshots"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    experiment_id: Mapped[str] = mapped_column(String(8), nullable=False, index=True)
    type: Mapped[str] = mapped_column(String(10), nullable=False)  # k8s / aws
    namespace: Mapped[str | None] = mapped_column(String(255), nullable=True)
    data: Mapped[dict] = mapped_column(JSON, nullable=False)
    captured_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(UTC)
    )


class ProbeResultRecord(Base):
    """Persistent probe execution result."""

    __tablename__ = "probe_results"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    experiment_id: Mapped[str] = mapped_column(String(8), nullable=False, index=True)
    probe_type: Mapped[str] = mapped_column(String(30), nullable=False)
    mode: Mapped[str] = mapped_column(String(20), nullable=False)
    result: Mapped[dict] = mapped_column(JSON, nullable=False)
    passed: Mapped[bool] = mapped_column(Boolean, nullable=False)
    executed_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(UTC)
    )


class AnalysisResultRecord(Base):
    """Persistent AI analysis result."""

    __tablename__ = "analysis_results"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    experiment_id: Mapped[str] = mapped_column(String(8), nullable=False, index=True)
    severity: Mapped[str] = mapped_column(String(10), nullable=False)
    root_cause: Mapped[str] = mapped_column(Text, nullable=False)
    confidence: Mapped[float] = mapped_column(Float, nullable=False)
    recommendations: Mapped[list] = mapped_column(JSON, nullable=False, default=list)
    resilience_score: Mapped[float | None] = mapped_column(Float, nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(UTC)
    )
