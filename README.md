# ChaosDuck

[한국어](docs/README.ko.md)

Chaos engineering platform for Kubernetes and AWS. AI-powered analysis with Claude, safety-first design, and a 5-phase experiment lifecycle.

## Architecture

```
┌─────────────────┐
│  React Frontend  │  :5173
│  (Vite + TW)     │
└────────┬────────┘
         │
┌────────▼────────┐     ┌─────────────────┐
│   Go Backend    │────▶│   AI Service    │  :8001
│   (Gin)  :8080  │     │ (FastAPI+Claude)│
│                 │     └─────────────────┘
│  ┌───────────┐  │
│  │  Safety   │  │     ┌─────────────────┐
│  │ Rollback  │  │────▶│  K8s Engine     │  client-go
│  │ Snapshot  │  │     │  AWS Engine     │  aws-sdk-go-v2
│  │ Guardrail │  │     └─────────────────┘
│  └───────────┘  │
│                 │     ┌─────────────────┐
│  sqlc + pgx     │────▶│  PostgreSQL 16  │  :5432
└─────────────────┘     └─────────────────┘
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25, Gin, sqlc + pgx, golang-migrate |
| K8s | client-go (pod delete, network chaos, CPU/memory stress) |
| AWS | aws-sdk-go-v2 (EC2 stop, RDS failover, route blackhole) |
| AI | FastAPI microservice + Anthropic Claude API |
| Frontend | React 18, Vite 6, Tailwind CSS 4, Recharts, react-force-graph |
| Observability | Prometheus client_golang, Grafana dashboards |
| Legacy Backend | Python 3.11, FastAPI (available via `legacy` profile) |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- (Optional) `kubectl` configured for K8s chaos
- (Optional) AWS credentials for AWS chaos
- (Optional) Anthropic API key for AI analysis

### Run with Docker Compose

```bash
# 1. Configure environment
cp .env.example .env
# Edit .env — set ANTHROPIC_API_KEY for AI features

# 2. Start all services
docker compose up --build

# Backend:   http://localhost:8080
# Frontend:  http://localhost:5173
# AI Service: http://localhost:8001
# Health:    http://localhost:8080/health
```

### Enable Monitoring (Prometheus + Grafana)

```bash
docker compose --profile monitoring up --build

# Prometheus: http://localhost:9090
# Grafana:    http://localhost:3000 (admin/admin)
```

## Operations Guide

### Connecting to a Kubernetes Cluster

ChaosDuck mounts `~/.kube/config` into the backend container. Make sure your kubeconfig is set to the target cluster:

```bash
# Verify current context
kubectl config current-context

# Switch context if needed
kubectl config use-context my-cluster
```

### Running a Chaos Experiment

**1. Create an experiment via API:**

```bash
curl -X POST http://localhost:8080/api/chaos/experiments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "pod-kill-test",
    "chaos_type": "pod_delete",
    "target": {
      "namespace": "default",
      "labels": {"app": "nginx"}
    },
    "safety": {
      "timeout_seconds": 60,
      "max_blast_radius": 1
    }
  }'
```

**2. Dry-run first (recommended):**

```bash
curl -X POST http://localhost:8080/api/chaos/dry-run \
  -H "Content-Type: application/json" \
  -d '{
    "name": "network-latency-test",
    "chaos_type": "network_latency",
    "target": {
      "namespace": "default",
      "labels": {"app": "web"}
    },
    "parameters": {
      "latency_ms": 200,
      "duration_seconds": 30
    }
  }'
```

**3. Stream experiment progress via SSE:**

```bash
# Experiment creation returns an ID; stream its events:
curl -N http://localhost:8080/api/chaos/experiments/{id}
# SSE events: phase transitions, probe results, rollback status
```

**4. Manual rollback (if needed):**

```bash
curl -X POST http://localhost:8080/api/chaos/experiments/{id}/rollback
```

**5. Emergency stop (rolls back ALL active experiments):**

```bash
curl -X POST http://localhost:8080/emergency-stop
```

### AI-Powered Analysis

Requires `ANTHROPIC_API_KEY` in `.env`.

```bash
# Analyze an experiment
curl -X POST http://localhost:8080/api/analysis/experiment/{id}

# Generate failure hypotheses
curl -X POST http://localhost:8080/api/analysis/hypotheses \
  -H "Content-Type: application/json" \
  -d '{"topology": {...}, "steady_state": {...}}'

# Natural language experiment creation
curl -X POST http://localhost:8080/api/analysis/nl-experiment \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Kill a random nginx pod and observe recovery time"}'

# Resilience score
curl -X POST http://localhost:8080/api/analysis/resilience-score \
  -H "Content-Type: application/json" \
  -d '{"experiments": [...]}'
```

## AWS Testing with LocalStack

[LocalStack](https://localstack.cloud/) provides a local AWS mock so you can test AWS chaos experiments without a real AWS account.

### Setup

```bash
# 1. Start LocalStack alongside the default services
docker compose --profile testing up --build

# LocalStack runs at http://localhost:4566

# 2. Set env vars in .env to point the backend at LocalStack
#    AWS_ENDPOINT_URL=http://localstack:4566
#    AWS_ACCESS_KEY_ID=test
#    AWS_SECRET_ACCESS_KEY=test

# Or override inline:
AWS_ENDPOINT_URL=http://localstack:4566 \
AWS_ACCESS_KEY_ID=test \
AWS_SECRET_ACCESS_KEY=test \
docker compose --profile testing up --build
```

The backend auto-detects the `AWS_ENDPOINT_URL` environment variable and routes all AWS SDK calls to LocalStack.

### Seed AWS Resources

```bash
# Create a mock EC2 instance
aws --endpoint-url=http://localhost:4566 ec2 run-instances \
  --image-id ami-12345678 \
  --instance-type t2.micro \
  --count 1

# Create a mock RDS instance
aws --endpoint-url=http://localhost:4566 rds create-db-instance \
  --db-instance-identifier test-db \
  --db-instance-class db.t3.micro \
  --engine postgres

# Create a VPC with route table
aws --endpoint-url=http://localhost:4566 ec2 create-vpc --cidr-block 10.0.0.0/16
```

### Run AWS Chaos Experiments

```bash
# EC2 stop experiment
curl -X POST http://localhost:8080/api/chaos/experiments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ec2-stop-test",
    "chaos_type": "ec2_stop",
    "target": {
      "instance_ids": ["i-xxxxxxxxx"]
    },
    "safety": {
      "timeout_seconds": 60
    }
  }'

# RDS failover experiment
curl -X POST http://localhost:8080/api/chaos/experiments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "rds-failover-test",
    "chaos_type": "rds_failover",
    "target": {
      "db_instance_identifier": "test-db"
    }
  }'
```

## K8s Testing with kind

[kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker) lets you run a local K8s cluster for testing.

### Setup

```bash
# Install kind
brew install kind  # macOS
# or: go install sigs.k8s.io/kind@latest

# Create a cluster
kind create cluster --name chaosduck-test

# Verify
kubectl cluster-info --context kind-chaosduck-test

# Deploy sample workloads
kubectl create deployment nginx --image=nginx --replicas=3
kubectl create deployment redis --image=redis --replicas=2
kubectl expose deployment nginx --port=80
```

### Run K8s Chaos Experiments

```bash
# Pod deletion
curl -X POST http://localhost:8080/api/chaos/experiments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "pod-kill-nginx",
    "chaos_type": "pod_delete",
    "target": {
      "namespace": "default",
      "labels": {"app": "nginx"}
    },
    "safety": {
      "timeout_seconds": 60,
      "max_blast_radius": 1
    }
  }'

# Network latency injection
curl -X POST http://localhost:8080/api/chaos/experiments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "latency-redis",
    "chaos_type": "network_latency",
    "target": {
      "namespace": "default",
      "labels": {"app": "redis"}
    },
    "parameters": {
      "latency_ms": 500,
      "duration_seconds": 30
    }
  }'

# CPU stress
curl -X POST http://localhost:8080/api/chaos/experiments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "cpu-stress-nginx",
    "chaos_type": "cpu_stress",
    "target": {
      "namespace": "default",
      "labels": {"app": "nginx"}
    },
    "parameters": {
      "cpu_cores": 1,
      "duration_seconds": 30
    }
  }'
```

### Cleanup

```bash
kind delete cluster --name chaosduck-test
```

## API Reference

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/metrics` | Prometheus metrics |
| `POST` | `/emergency-stop` | Emergency stop all experiments |
| `POST` | `/api/chaos/experiments` | Create and run experiment (SSE stream) |
| `GET` | `/api/chaos/experiments` | List all experiments |
| `GET` | `/api/chaos/experiments/:id` | Get experiment detail |
| `POST` | `/api/chaos/experiments/:id/rollback` | Manual rollback |
| `POST` | `/api/chaos/dry-run` | Dry-run experiment |
| `GET` | `/api/topology/k8s` | K8s cluster topology |
| `GET` | `/api/topology/aws` | AWS resource topology |
| `GET` | `/api/topology/combined` | Combined topology |
| `GET` | `/api/topology/steady-state` | Current steady-state metrics |
| `POST` | `/api/analysis/experiment/:id` | AI experiment analysis |
| `POST` | `/api/analysis/hypotheses` | AI failure hypothesis generation |
| `POST` | `/api/analysis/resilience-score` | Resilience scoring |
| `POST` | `/api/analysis/report` | AI report generation |
| `POST` | `/api/analysis/generate-experiments` | AI experiment generation |
| `POST` | `/api/analysis/nl-experiment` | Natural language to experiment |
| `GET` | `/api/analysis/resilience-trend` | Resilience score trend |
| `GET` | `/api/analysis/resilience-trend/summary` | Trend summary |

## Experiment Lifecycle (5-Phase)

```
STEADY_STATE ──▶ HYPOTHESIS ──▶ INJECT ──▶ OBSERVE ──▶ ROLLBACK
     │                │            │           │            │
  Capture         AI generates   Execute    Monitor &    LIFO restore
  baseline        predictions    chaos      collect      to baseline
  metrics                        action     results
```

1. **STEADY_STATE** — Capture baseline metrics via probes (HTTP, Cmd, K8s, Prometheus)
2. **HYPOTHESIS** — AI generates failure predictions
3. **INJECT** — Execute chaos action (pod kill, network fault, resource stress, etc.)
4. **OBSERVE** — Monitor system behavior and collect results
5. **ROLLBACK** — LIFO-ordered rollback restores original state

## Development

### Go Backend

```bash
cd backend-go

# Build
go build ./cmd/server

# Run tests
go test ./... -v -race

# Lint
golangci-lint run

# Generate sqlc code (after changing queries)
~/go/bin/sqlc generate
```

### AI Service

```bash
cd ai-service
pip install -r requirements.txt
uvicorn main:app --reload --port 8001
```

### Frontend

```bash
cd frontend
npm install
npm run dev       # http://localhost:5173
npm run build     # production build
```

### Legacy Python Backend

```bash
# Lint & format
make lint          # ruff check backend/
make format        # ruff format backend/

# Tests (159 tests, 87%+ coverage)
make test          # pytest with coverage
make check         # lint + test

# Run with Docker
docker compose --profile legacy up backend-python
# Legacy backend: http://localhost:8000
```

## Project Structure

```
ChaosDuck/
├── backend-go/                    # Go backend (primary)
│   ├── cmd/server/main.go         # Entry point
│   ├── internal/
│   │   ├── config/                # Configuration
│   │   ├── db/                    # sqlc + pgx, migrations, queries
│   │   ├── domain/                # Domain models (experiment, topology)
│   │   ├── engine/                # Chaos engines (k8s, aws, runner)
│   │   ├── handler/               # HTTP handlers (Gin routes)
│   │   ├── probe/                 # Health probes (HTTP, Cmd, K8s, Prom)
│   │   ├── safety/                # Rollback, snapshot, guardrails, healthcheck
│   │   └── observability/         # Prometheus metrics
│   ├── Dockerfile
│   ├── go.mod / go.sum
│   └── .golangci.yml
├── ai-service/                    # AI microservice (Python)
│   ├── main.py                    # FastAPI app
│   ├── ai_engine.py               # Anthropic Claude integration
│   ├── requirements.txt
│   └── Dockerfile
├── frontend/                      # React dashboard
│   ├── src/
│   │   ├── App.jsx                # Main app (tabs: Topology, Experiments, Analysis)
│   │   ├── Dashboard.jsx          # Dashboard overview
│   │   ├── TopologyGraph.jsx      # Force-graph visualization
│   │   ├── ExperimentForm.jsx     # Experiment creation
│   │   ├── ExperimentList.jsx     # Experiments list
│   │   ├── AnalysisPanel.jsx      # AI analysis panel
│   │   ├── useSSE.js              # SSE streaming hook
│   │   └── api.js                 # API client
│   ├── Dockerfile
│   └── package.json
├── backend/                       # Legacy Python backend
│   ├── main.py                    # FastAPI entry point
│   ├── engines/                   # K8s, AWS, AI engines
│   ├── safety/                    # Rollback, snapshot, guardrails
│   ├── probes/                    # Health probes
│   ├── routers/                   # API routes
│   └── tests/                     # 159 tests, 87%+ coverage
├── cli/
│   └── chaosduck.py               # Click-based CLI
├── prometheus/                    # Prometheus config
├── grafana/                       # Grafana dashboards & provisioning
├── docker-compose.yml             # Multi-service orchestration
├── Makefile                       # Python lint/test shortcuts
├── pyproject.toml                 # Ruff, pytest config
├── .env.example                   # Environment template
└── CLAUDE.md                      # AI assistant guidelines
```

## Safety Rules

All chaos actions follow strict safety guarantees:

1. **Rollback tuple** — Every engine function returns `(result, rollback_fn)`
2. **LIFO rollback** — Rollback operations execute in reverse order
3. **Emergency stop** — Triggers rollback of ALL active experiments
4. **Production guard** — Production namespaces require explicit confirmation
5. **Timeout enforcement** — All experiments have a max timeout (default: 120s)
6. **Blast radius validation** — Pre-injection check limits scope of impact
7. **State snapshot** — Full state capture before any mutation

## Chaos Types

### Kubernetes
| Type | Description |
|------|-------------|
| `pod_delete` | Delete target pods |
| `network_latency` | Inject network latency (tc netem) |
| `network_loss` | Inject packet loss |
| `cpu_stress` | CPU stress via stress-ng |
| `memory_stress` | Memory stress via stress-ng |

### AWS
| Type | Description |
|------|-------------|
| `ec2_stop` | Stop EC2 instances |
| `rds_failover` | Trigger RDS failover |
| `route_blackhole` | Inject VPC route blackhole |

## License

MIT
