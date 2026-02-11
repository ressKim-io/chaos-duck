"""Initial schema

Revision ID: 001
Revises:
Create Date: 2026-02-11
"""
from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "001"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.create_table(
        "experiments",
        sa.Column("id", sa.String(8), primary_key=True),
        sa.Column("config", sa.JSON, nullable=False),
        sa.Column("status", sa.String(30), server_default="pending"),
        sa.Column("phase", sa.String(30), server_default="steady_state"),
        sa.Column("started_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("completed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("steady_state", sa.JSON, nullable=True),
        sa.Column("hypothesis", sa.Text, nullable=True),
        sa.Column("injection_result", sa.JSON, nullable=True),
        sa.Column("observations", sa.JSON, nullable=True),
        sa.Column("rollback_result", sa.JSON, nullable=True),
        sa.Column("error", sa.Text, nullable=True),
    )

    op.create_table(
        "snapshots",
        sa.Column("id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column("experiment_id", sa.String(8), nullable=False, index=True),
        sa.Column("type", sa.String(10), nullable=False),
        sa.Column("namespace", sa.String(255), nullable=True),
        sa.Column("data", sa.JSON, nullable=False),
        sa.Column(
            "captured_at", sa.DateTime(timezone=True), server_default=sa.func.now()
        ),
    )

    op.create_table(
        "probe_results",
        sa.Column("id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column("experiment_id", sa.String(8), nullable=False, index=True),
        sa.Column("probe_type", sa.String(30), nullable=False),
        sa.Column("mode", sa.String(20), nullable=False),
        sa.Column("result", sa.JSON, nullable=False),
        sa.Column("passed", sa.Boolean, nullable=False),
        sa.Column(
            "executed_at", sa.DateTime(timezone=True), server_default=sa.func.now()
        ),
    )

    op.create_table(
        "analysis_results",
        sa.Column("id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column("experiment_id", sa.String(8), nullable=False, index=True),
        sa.Column("severity", sa.String(10), nullable=False),
        sa.Column("root_cause", sa.Text, nullable=False),
        sa.Column("confidence", sa.Float, nullable=False),
        sa.Column("recommendations", sa.JSON, nullable=False),
        sa.Column("resilience_score", sa.Float, nullable=True),
        sa.Column(
            "created_at", sa.DateTime(timezone=True), server_default=sa.func.now()
        ),
    )


def downgrade() -> None:
    op.drop_table("analysis_results")
    op.drop_table("probe_results")
    op.drop_table("snapshots")
    op.drop_table("experiments")
