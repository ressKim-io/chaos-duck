import time

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

from .metrics import METRICS


class PrometheusMiddleware(BaseHTTPMiddleware):
    """FastAPI middleware that records HTTP request metrics."""

    async def dispatch(self, request: Request, call_next) -> Response:
        # Normalize path to avoid high-cardinality labels
        path = self._normalize_path(request.url.path)
        method = request.method

        start = time.perf_counter()
        response = await call_next(request)
        duration = time.perf_counter() - start

        METRICS.http_requests_total.labels(
            method=method, path=path, status_code=response.status_code
        ).inc()
        METRICS.http_request_duration_seconds.labels(method=method, path=path).observe(duration)

        return response

    @staticmethod
    def _normalize_path(path: str) -> str:
        """Replace dynamic path segments with placeholders."""
        parts = path.strip("/").split("/")
        normalized = []
        for part in parts:
            # Replace UUIDs and short IDs with placeholder
            if len(part) == 8 and all(c in "0123456789abcdef-" for c in part):
                normalized.append("{id}")
            elif part.startswith("dry-"):
                normalized.append("{id}")
            else:
                normalized.append(part)
        return "/" + "/".join(normalized)
