import logging
import re

import httpx

from .base import BaseProbe, ProbeMode, ProbeResult

logger = logging.getLogger(__name__)


class HttpProbe(BaseProbe):
    """HTTP endpoint health check probe.

    Validates that a URL returns the expected status code and optionally
    matches a pattern in the response body.
    """

    def __init__(
        self,
        name: str,
        mode: ProbeMode,
        url: str,
        expected_status: int = 200,
        timeout_seconds: float = 5.0,
        body_pattern: str | None = None,
        method: str = "GET",
        headers: dict[str, str] | None = None,
    ):
        super().__init__(name, mode)
        self.url = url
        self.expected_status = expected_status
        self.timeout_seconds = timeout_seconds
        self.body_pattern = body_pattern
        self.method = method.upper()
        self.headers = headers or {}

    @property
    def probe_type(self) -> str:
        return "http"

    async def execute(self) -> ProbeResult:
        async with httpx.AsyncClient(timeout=self.timeout_seconds) as client:
            resp = await client.request(self.method, self.url, headers=self.headers)

        status_ok = resp.status_code == self.expected_status
        body_ok = True
        if self.body_pattern and status_ok:
            body_ok = bool(re.search(self.body_pattern, resp.text))

        passed = status_ok and body_ok
        detail = {
            "url": self.url,
            "status_code": resp.status_code,
            "expected_status": self.expected_status,
            "body_match": body_ok,
            "response_time_ms": resp.elapsed.total_seconds() * 1000,
        }

        return ProbeResult(
            probe_name=self.name,
            probe_type=self.probe_type,
            mode=self.mode,
            passed=passed,
            detail=detail,
        )
