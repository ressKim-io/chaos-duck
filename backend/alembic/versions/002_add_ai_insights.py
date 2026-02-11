"""Add ai_insights column to experiments table

Revision ID: 002
Revises: 001
Create Date: 2026-02-11
"""
from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "002"
down_revision: str | None = "001"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column("experiments", sa.Column("ai_insights", sa.JSON, nullable=True))


def downgrade() -> None:
    op.drop_column("experiments", "ai_insights")
