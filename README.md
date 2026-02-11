# ChaosDuck

K8s & AWS 대상 카오스 엔지니어링 플랫폼. Claude AI 기반 자동 분석과 Safety-first 설계.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  React GUI  │────▶│  FastAPI     │────▶│  K8s Engine │
│  Dashboard  │     │  Backend     │     │  AWS Engine │
└─────────────┘     │              │     │  AI Engine  │
┌─────────────┐     │  Safety:     │     └─────────────┘
│  Click CLI  │────▶│  - Rollback  │
│             │     │  - Snapshot  │
└─────────────┘     │  - Guardrail │
                    └──────────────┘
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Python 3.11, FastAPI, Pydantic |
| K8s | kubernetes python client |
| AWS | boto3 |
| AI | Anthropic Claude API (anthropic SDK) |
| Frontend | React 18, Vite |
| CLI | Click |
| Lint | Ruff |
| Test | pytest, pytest-asyncio, pytest-cov |
| CI | GitHub Actions, pre-commit |

## Experiment Lifecycle (5-Phase)

1. **STEADY_STATE** - Capture baseline metrics
2. **HYPOTHESIS** - AI generates failure hypothesis
3. **INJECT** - Execute chaos (pod kill, network latency, etc.)
4. **OBSERVE** - Monitor and collect results
5. **ROLLBACK** - LIFO rollback to restore state

## Safety Rules

- All engine functions return `(result, rollback_fn)` tuple
- Rollback is always LIFO ordered
- Emergency Stop triggers rollback of ALL active experiments
- Production namespaces require explicit confirmation
- Timeout enforced on all experiments (max 120s)
- Blast radius validation before injection
- State snapshot before any mutation

## Chaos Types

### Kubernetes
- Pod deletion
- Network latency injection
- Network packet loss
- CPU stress
- Memory stress

### AWS
- EC2 instance stop
- RDS failover
- VPC route blackhole

## Getting Started

### Prerequisites

- Python 3.11+
- Node.js 20+
- Docker & Docker Compose (for containerized setup)
- kubectl configured (for K8s chaos)
- AWS credentials (for AWS chaos)
- Anthropic API key (for AI analysis)

### Quick Start with Docker Compose

```bash
# 1. 환경변수 설정
cp .env.example .env
# .env 파일에 ANTHROPIC_API_KEY 입력

# 2. 실행
docker compose up --build

# Backend:  http://localhost:8000
# Frontend: http://localhost:5173
# Health:   http://localhost:8000/health
```

### Local Development

```bash
# Backend
cd backend
pip install -r requirements.txt
uvicorn main:app --reload --port 8000

# Frontend (별도 터미널)
cd frontend
npm install
npm run dev

# CLI
python cli/chaosduck.py --help
```

## Development

### Lint & Format

```bash
make lint          # ruff check backend/
make format        # ruff format backend/
```

### Test

```bash
make test          # pytest with coverage (threshold: 80%)
make check         # lint + test
```

현재 테스트 현황:
- **93 tests** passing
- **83.87% coverage**

### Pre-commit Hooks

```bash
pip install pre-commit
pre-commit install

# 수동 실행
pre-commit run --all-files
```

### CI/CD

GitHub Actions (`ci.yml`)가 PR/push 시 자동으로 실행:
1. Python 3.11 setup
2. `ruff check` (lint)
3. `ruff format --check` (formatting)
4. `pytest --cov --cov-fail-under=80` (test + coverage)

### Git Conventions

- Commit format: `<type>(<scope>): <subject>` (feat, fix, docs, style, refactor, test, chore)
- Branch naming: `feature/#123-description`, `fix/#456-description`
- PR max 400 lines, Squash and Merge
- Test coverage 80%+
- No secrets in code

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/emergency-stop` | Trigger emergency stop |
| `POST` | `/api/chaos/experiments` | Create and run experiment |
| `GET` | `/api/chaos/experiments` | List all experiments |
| `GET` | `/api/chaos/experiments/{id}` | Get experiment detail |
| `POST` | `/api/chaos/experiments/{id}/rollback` | Manual rollback |
| `POST` | `/api/chaos/dry-run` | Dry-run experiment |
| `GET` | `/api/topology/k8s` | K8s topology |
| `GET` | `/api/topology/aws` | AWS topology |
| `GET` | `/api/topology/combined` | Combined topology |
| `GET` | `/api/topology/steady-state` | Current metrics |
| `POST` | `/api/analysis/experiment/{id}` | AI analysis |
| `POST` | `/api/analysis/hypotheses` | Generate hypothesis |
| `POST` | `/api/analysis/resilience-score` | Resilience score |
| `POST` | `/api/analysis/report` | Generate report |

## Project Structure

```
ChaosDuck/
├── backend/
│   ├── main.py                 # FastAPI app entry point
│   ├── models/                 # Pydantic data models
│   │   ├── experiment.py       # Experiment, SafetyConfig, ChaosType
│   │   └── topology.py         # TopologyNode, ResourceType, ResilienceScore
│   ├── engines/                # Chaos execution engines
│   │   ├── k8s_engine.py       # Kubernetes chaos (pod delete, network, stress)
│   │   ├── aws_engine.py       # AWS chaos (EC2 stop, RDS failover, route blackhole)
│   │   └── ai_engine.py        # Claude AI analysis and hypothesis generation
│   ├── safety/                 # Safety-first stack
│   │   ├── rollback.py         # LIFO rollback manager
│   │   ├── snapshot.py         # State capture before mutation
│   │   └── guardrails.py       # Emergency stop, timeout, confirmation, blast radius
│   ├── routers/                # FastAPI route handlers
│   │   ├── chaos.py            # Experiment lifecycle CRUD
│   │   ├── topology.py         # Infrastructure discovery
│   │   └── analysis.py         # AI analysis endpoints
│   ├── tests/                  # Test suite (93 tests, 83%+ coverage)
│   │   ├── conftest.py         # Shared fixtures
│   │   ├── test_models.py      # Model validation tests
│   │   ├── test_safety.py      # Rollback, snapshot, guardrails tests
│   │   ├── test_engines.py     # Engine tests (mocked)
│   │   ├── test_routers.py     # API endpoint tests
│   │   └── test_main.py        # Health & emergency stop tests
│   ├── requirements.txt
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── App.jsx             # Dashboard (Topology, Experiments, Analysis tabs)
│   │   └── main.jsx
│   ├── vite.config.js
│   ├── package.json
│   └── Dockerfile
├── cli/
│   └── chaosduck.py            # Click-based CLI
├── docker-compose.yml
├── pyproject.toml              # Ruff, pytest, coverage config
├── Makefile                    # lint/format/test/check shortcuts
├── .pre-commit-config.yaml     # Ruff pre-commit hooks
├── .github/
│   └── workflows/ci.yml        # GitHub Actions CI pipeline
└── CLAUDE.md                   # AI assistant guidelines
```
