import asyncio
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import PlainTextResponse
from prometheus_client import generate_latest

from database import close_db, init_db
from observability.middleware import PrometheusMiddleware
from routers import analysis, chaos, topology

# Global emergency stop event
emergency_stop_event = asyncio.Event()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan: startup and shutdown."""
    emergency_stop_event.clear()
    await init_db()
    yield
    # Trigger emergency stop on shutdown to rollback active experiments
    emergency_stop_event.set()
    await close_db()


app = FastAPI(
    title="ChaosDuck",
    description="K8s & AWS Chaos Engineering Platform with AI Analysis",
    version="0.1.0",
    lifespan=lifespan,
)

app.add_middleware(PrometheusMiddleware)
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:5173"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(chaos.router, prefix="/api/chaos", tags=["chaos"])
app.include_router(topology.router, prefix="/api/topology", tags=["topology"])
app.include_router(analysis.router, prefix="/api/analysis", tags=["analysis"])


@app.get("/health")
async def health_check():
    return {
        "status": "healthy",
        "emergency_stop": emergency_stop_event.is_set(),
    }


@app.get("/metrics")
async def metrics():
    """Prometheus metrics endpoint."""
    return PlainTextResponse(generate_latest(), media_type="text/plain; version=0.0.4")


@app.post("/emergency-stop")
async def trigger_emergency_stop():
    """Trigger emergency stop - rollback all active experiments."""
    emergency_stop_event.set()
    return {"status": "emergency_stop_triggered"}
