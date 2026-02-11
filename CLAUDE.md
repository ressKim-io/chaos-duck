# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ChaosDuck - K8s & AWS 대상 카오스 엔지니어링 플랫폼.
Claude AI 기반 자동 분석, Safety-first 설계, 5-Phase 실험 라이프사이클.

## Architecture

- **Backend**: Python / FastAPI (backend/)
- **Frontend**: React + Vite (frontend/)
- **CLI**: Click-based CLI (cli/)
- **Engines**: K8s (kubernetes client), AWS (boto3), AI (anthropic SDK)
- **Safety Stack**: Rollback Manager (LIFO), Snapshot, Guardrails, Emergency Stop

## Development Commands

```bash
# Backend
cd backend && pip install -r requirements.txt
cd backend && uvicorn main:app --reload --port 8000

# Frontend
cd frontend && npm install
cd frontend && npm run dev

# CLI
python cli/chaosduck.py --help

# Lint & Format
make lint          # ruff check backend/
make format        # ruff format backend/

# Tests
make test          # pytest -v --cov=backend --cov-report=term-missing
make check         # lint + test 통합

# Pre-commit hooks
pre-commit install
pre-commit run --all-files

# Docker
docker-compose up
```

## Safety Rules (CRITICAL)

1. All engine functions return `(result, rollback_fn)` tuple
2. Rollback is LIFO ordered - always
3. Emergency Stop triggers rollback of ALL active experiments
4. Production namespaces require explicit confirmation
5. Timeout enforced on all experiments (max 120s)
6. Blast radius validation before injection
7. State snapshot before any mutation

## Git Conventions

- Language: responses in Korean, code comments in English, commit messages in English
- Commit format: `<type>(<scope>): <subject>` (feat, fix, docs, style, refactor, test, chore)
- Branch naming: `feature/#123-description`, `fix/#456-description`
- PR max 400 lines, Squash and Merge
- No secrets in code, test coverage 80%+

## Key Patterns

- Experiment lifecycle: STEADY_STATE -> HYPOTHESIS -> INJECT -> OBSERVE -> ROLLBACK
- `ExperimentContext` context manager for auto snapshot + rollback on exception
- `@with_timeout(seconds)` decorator for timeout enforcement
- `@require_confirmation(namespace_pattern)` for production safety
