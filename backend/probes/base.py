import logging
from abc import ABC, abstractmethod
from datetime import UTC, datetime
from enum import Enum
from typing import Any

from pydantic import BaseModel

logger = logging.getLogger(__name__)


class ProbeMode(str, Enum):
    """When a probe should be executed during the experiment lifecycle."""

    SOT = "sot"  # Start of Test - verify steady state before injection
    EOT = "eot"  # End of Test - verify recovery after rollback
    CONTINUOUS = "continuous"  # Polled at interval during experiment
    ON_CHAOS = "on_chaos"  # Immediately after fault injection


class ProbeResult(BaseModel):
    """Result of a single probe execution."""

    probe_name: str
    probe_type: str
    mode: ProbeMode
    passed: bool
    detail: dict[str, Any] = {}
    error: str | None = None
    executed_at: datetime = None

    def __init__(self, **data):
        if data.get("executed_at") is None:
            data["executed_at"] = datetime.now(UTC)
        super().__init__(**data)


class BaseProbe(ABC):
    """Abstract base class for all resilience probes."""

    def __init__(self, name: str, mode: ProbeMode, **kwargs):
        self.name = name
        self.mode = mode

    @property
    @abstractmethod
    def probe_type(self) -> str:
        """Return the probe type identifier."""

    @abstractmethod
    async def execute(self) -> ProbeResult:
        """Execute the probe and return the result."""

    async def safe_execute(self) -> ProbeResult:
        """Execute with error handling, never raises."""
        try:
            return await self.execute()
        except Exception as e:
            logger.error("Probe %s failed: %s", self.name, e)
            return ProbeResult(
                probe_name=self.name,
                probe_type=self.probe_type,
                mode=self.mode,
                passed=False,
                error=str(e),
            )
