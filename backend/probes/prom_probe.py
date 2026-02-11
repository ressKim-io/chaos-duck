import logging

import httpx

from .base import BaseProbe, ProbeMode, ProbeResult

logger = logging.getLogger(__name__)


class PromProbe(BaseProbe):
    """Prometheus PromQL query probe.

    Executes a PromQL query and compares the result against an expected
    value using a comparison operator.
    """

    def __init__(
        self,
        name: str,
        mode: ProbeMode,
        endpoint: str,
        query: str,
        comparator: str = ">",
        threshold: float = 0.0,
        timeout_seconds: float = 5.0,
    ):
        super().__init__(name, mode)
        self.endpoint = endpoint.rstrip("/")
        self.query = query
        self.comparator = comparator
        self.threshold = threshold
        self.timeout_seconds = timeout_seconds

    @property
    def probe_type(self) -> str:
        return "prometheus"

    async def execute(self) -> ProbeResult:
        url = f"{self.endpoint}/api/v1/query"
        async with httpx.AsyncClient(timeout=self.timeout_seconds) as client:
            resp = await client.get(url, params={"query": self.query})

        if resp.status_code != 200:
            return ProbeResult(
                probe_name=self.name,
                probe_type=self.probe_type,
                mode=self.mode,
                passed=False,
                error=f"Prometheus returned {resp.status_code}",
            )

        data = resp.json()
        results = data.get("data", {}).get("result", [])

        if not results:
            return ProbeResult(
                probe_name=self.name,
                probe_type=self.probe_type,
                mode=self.mode,
                passed=False,
                detail={"query": self.query, "error": "No results returned"},
            )

        # Use the first result's value
        value = float(results[0]["value"][1])
        passed = self._compare(value)

        detail = {
            "query": self.query,
            "value": value,
            "comparator": self.comparator,
            "threshold": self.threshold,
            "result_count": len(results),
        }

        return ProbeResult(
            probe_name=self.name,
            probe_type=self.probe_type,
            mode=self.mode,
            passed=passed,
            detail=detail,
        )

    def _compare(self, value: float) -> bool:
        ops = {
            ">": value > self.threshold,
            ">=": value >= self.threshold,
            "<": value < self.threshold,
            "<=": value <= self.threshold,
            "==": value == self.threshold,
            "!=": value != self.threshold,
        }
        return ops.get(self.comparator, False)
