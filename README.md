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
| Backend | Python, FastAPI, Pydantic |
| K8s | kubernetes python client |
| AWS | boto3 |
| AI | Anthropic Claude API (anthropic SDK) |
| Frontend | React, Vite |
| CLI | Click |

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

```bash
# Backend
cd backend
pip install -r requirements.txt
uvicorn main:app --reload

# Frontend
cd frontend
npm install
npm run dev

# CLI
python cli/chaosduck.py --help

# Docker Compose
docker-compose up
```

## Development

- Commit format: `<type>(<scope>): <subject>`
- Branch naming: `feature/#123-description`, `fix/#456-description`
- PR max 400 lines, Squash and Merge
- Test coverage 80%+
- No secrets in code
