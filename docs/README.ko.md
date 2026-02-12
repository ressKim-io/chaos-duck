# ChaosDuck

[English](../README.md)

Kubernetes와 AWS를 위한 카오스 엔지니어링 플랫폼. Claude AI 기반 자동 분석, Safety-first 설계, 5단계 실험 라이프사이클.

## 아키텍처

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

## 기술 스택

| 레이어 | 기술 |
|--------|------|
| Backend | Go 1.25, Gin, sqlc + pgx, golang-migrate |
| K8s | client-go (pod 삭제, 네트워크 카오스, CPU/메모리 스트레스) |
| AWS | aws-sdk-go-v2 (EC2 중지, RDS 페일오버, 라우트 블랙홀) |
| AI | FastAPI 마이크로서비스 + Anthropic Claude API |
| Frontend | React 18, Vite 6, Tailwind CSS 4, Recharts, react-force-graph |
| Observability | Prometheus client_golang, Grafana 대시보드 |
| Legacy Backend | Python 3.11, FastAPI (`legacy` 프로파일로 사용 가능) |

## 빠른 시작

### 사전 요구사항

- Docker & Docker Compose
- (선택) K8s 카오스를 위한 `kubectl` 설정
- (선택) AWS 카오스를 위한 AWS 자격 증명
- (선택) AI 분석을 위한 Anthropic API 키

### Docker Compose로 실행

```bash
# 1. 환경 변수 설정
cp .env.example .env
# .env 파일에 ANTHROPIC_API_KEY 입력 (AI 기능 사용 시)

# 2. 전체 서비스 시작
docker compose up --build

# Backend:    http://localhost:8080
# Frontend:   http://localhost:5173
# AI Service: http://localhost:8001
# Health:     http://localhost:8080/health
```

### 모니터링 활성화 (Prometheus + Grafana)

```bash
docker compose --profile monitoring up --build

# Prometheus: http://localhost:9090
# Grafana:    http://localhost:3000 (admin/admin)
```

## 운영 가이드

### Kubernetes 클러스터 연결

ChaosDuck은 `~/.kube/config`를 백엔드 컨테이너에 마운트합니다. kubeconfig가 대상 클러스터를 가리키도록 설정하세요:

```bash
# 현재 컨텍스트 확인
kubectl config current-context

# 컨텍스트 전환
kubectl config use-context my-cluster
```

### 카오스 실험 실행

**1. API로 실험 생성:**

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

**2. 드라이런 먼저 실행 (권장):**

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

**3. SSE로 실험 진행 상황 스트리밍:**

```bash
# 실험 생성 시 반환된 ID로 이벤트 스트리밍:
curl -N http://localhost:8080/api/chaos/experiments/{id}
# SSE 이벤트: 단계 전환, 프로브 결과, 롤백 상태
```

**4. 수동 롤백 (필요시):**

```bash
curl -X POST http://localhost:8080/api/chaos/experiments/{id}/rollback
```

**5. 긴급 정지 (모든 활성 실험 롤백):**

```bash
curl -X POST http://localhost:8080/emergency-stop
```

### AI 기반 분석

`.env`에 `ANTHROPIC_API_KEY` 필요.

```bash
# 실험 분석
curl -X POST http://localhost:8080/api/analysis/experiment/{id}

# 장애 가설 생성
curl -X POST http://localhost:8080/api/analysis/hypotheses \
  -H "Content-Type: application/json" \
  -d '{"topology": {...}, "steady_state": {...}}'

# 자연어로 실험 생성
curl -X POST http://localhost:8080/api/analysis/nl-experiment \
  -H "Content-Type: application/json" \
  -d '{"prompt": "nginx pod 하나를 랜덤으로 죽이고 복구 시간 관찰"}'

# 회복탄력성 점수
curl -X POST http://localhost:8080/api/analysis/resilience-score \
  -H "Content-Type: application/json" \
  -d '{"experiments": [...]}'
```

## LocalStack으로 AWS 테스트

[LocalStack](https://localstack.cloud/)은 로컬 AWS 모의 환경을 제공하여 실제 AWS 계정 없이 AWS 카오스 실험을 테스트할 수 있습니다.

### 설정

```bash
# 1. 기본 서비스와 함께 LocalStack 시작
docker compose --profile testing up --build

# LocalStack: http://localhost:4566

# 2. .env에 환경 변수 설정하여 백엔드가 LocalStack을 사용하도록 함
#    AWS_ENDPOINT_URL=http://localstack:4566
#    AWS_ACCESS_KEY_ID=test
#    AWS_SECRET_ACCESS_KEY=test

# 또는 인라인 오버라이드:
AWS_ENDPOINT_URL=http://localstack:4566 \
AWS_ACCESS_KEY_ID=test \
AWS_SECRET_ACCESS_KEY=test \
docker compose --profile testing up --build
```

백엔드는 `AWS_ENDPOINT_URL` 환경 변수를 자동 감지하여 모든 AWS SDK 호출을 LocalStack으로 라우팅합니다.

### AWS 리소스 시드

```bash
# 모의 EC2 인스턴스 생성
aws --endpoint-url=http://localhost:4566 ec2 run-instances \
  --image-id ami-12345678 \
  --instance-type t2.micro \
  --count 1

# 모의 RDS 인스턴스 생성
aws --endpoint-url=http://localhost:4566 rds create-db-instance \
  --db-instance-identifier test-db \
  --db-instance-class db.t3.micro \
  --engine postgres

# 라우트 테이블이 있는 VPC 생성
aws --endpoint-url=http://localhost:4566 ec2 create-vpc --cidr-block 10.0.0.0/16
```

### AWS 카오스 실험 실행

```bash
# EC2 중지 실험
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

# RDS 페일오버 실험
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

## kind로 K8s 테스트

[kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker)를 사용하여 로컬 K8s 클러스터에서 테스트할 수 있습니다.

### 설정

```bash
# kind 설치
brew install kind  # macOS
# 또는: go install sigs.k8s.io/kind@latest

# 클러스터 생성
kind create cluster --name chaosduck-test

# 확인
kubectl cluster-info --context kind-chaosduck-test

# 샘플 워크로드 배포
kubectl create deployment nginx --image=nginx --replicas=3
kubectl create deployment redis --image=redis --replicas=2
kubectl expose deployment nginx --port=80
```

### K8s 카오스 실험 실행

```bash
# Pod 삭제
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

# 네트워크 지연 주입
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

# CPU 스트레스
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

### 정리

```bash
kind delete cluster --name chaosduck-test
```

## API 레퍼런스

| 메서드 | 경로 | 설명 |
|--------|------|------|
| `GET` | `/health` | 헬스 체크 |
| `GET` | `/metrics` | Prometheus 메트릭 |
| `POST` | `/emergency-stop` | 모든 실험 긴급 정지 |
| `POST` | `/api/chaos/experiments` | 실험 생성 및 실행 (SSE 스트림) |
| `GET` | `/api/chaos/experiments` | 실험 목록 조회 |
| `GET` | `/api/chaos/experiments/:id` | 실험 상세 조회 |
| `POST` | `/api/chaos/experiments/:id/rollback` | 수동 롤백 |
| `POST` | `/api/chaos/dry-run` | 드라이런 실험 |
| `GET` | `/api/topology/k8s` | K8s 클러스터 토폴로지 |
| `GET` | `/api/topology/aws` | AWS 리소스 토폴로지 |
| `GET` | `/api/topology/combined` | 통합 토폴로지 |
| `GET` | `/api/topology/steady-state` | 현재 Steady-state 메트릭 |
| `POST` | `/api/analysis/experiment/:id` | AI 실험 분석 |
| `POST` | `/api/analysis/hypotheses` | AI 장애 가설 생성 |
| `POST` | `/api/analysis/resilience-score` | 회복탄력성 점수 |
| `POST` | `/api/analysis/report` | AI 리포트 생성 |
| `POST` | `/api/analysis/generate-experiments` | AI 실험 생성 |
| `POST` | `/api/analysis/nl-experiment` | 자연어 → 실험 변환 |
| `GET` | `/api/analysis/resilience-trend` | 회복탄력성 점수 추이 |
| `GET` | `/api/analysis/resilience-trend/summary` | 추이 요약 |

## 실험 라이프사이클 (5단계)

```
STEADY_STATE ──▶ HYPOTHESIS ──▶ INJECT ──▶ OBSERVE ──▶ ROLLBACK
     │                │            │           │            │
  기준 메트릭     AI가 장애       카오스      시스템       LIFO 순서로
  수집           예측 생성       액션 실행    모니터링     원상 복구
```

1. **STEADY_STATE** — 프로브(HTTP, Cmd, K8s, Prometheus)로 기준 메트릭 수집
2. **HYPOTHESIS** — AI가 장애 예측 생성
3. **INJECT** — 카오스 액션 실행 (pod 삭제, 네트워크 장애, 리소스 스트레스 등)
4. **OBSERVE** — 시스템 동작 모니터링 및 결과 수집
5. **ROLLBACK** — LIFO 순서로 원래 상태 복원

## 개발

### Go Backend

```bash
cd backend-go

# 빌드
go build ./cmd/server

# 테스트
go test ./... -v -race

# 린트
golangci-lint run

# sqlc 코드 생성 (쿼리 변경 후)
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
npm run build     # 프로덕션 빌드
```

### Legacy Python Backend

```bash
# 린트 & 포맷
make lint          # ruff check backend/
make format        # ruff format backend/

# 테스트 (159개, 87%+ 커버리지)
make test          # pytest with coverage
make check         # lint + test

# Docker로 실행
docker compose --profile legacy up backend-python
# Legacy backend: http://localhost:8000
```

## 프로젝트 구조

```
ChaosDuck/
├── backend-go/                    # Go 백엔드 (메인)
│   ├── cmd/server/main.go         # 진입점
│   ├── internal/
│   │   ├── config/                # 설정
│   │   ├── db/                    # sqlc + pgx, 마이그레이션, 쿼리
│   │   ├── domain/                # 도메인 모델 (experiment, topology)
│   │   ├── engine/                # 카오스 엔진 (k8s, aws, runner)
│   │   ├── handler/               # HTTP 핸들러 (Gin 라우트)
│   │   ├── probe/                 # 헬스 프로브 (HTTP, Cmd, K8s, Prom)
│   │   ├── safety/                # 롤백, 스냅샷, 가드레일, 헬스체크
│   │   └── observability/         # Prometheus 메트릭
│   ├── Dockerfile
│   ├── go.mod / go.sum
│   └── .golangci.yml
├── ai-service/                    # AI 마이크로서비스 (Python)
│   ├── main.py                    # FastAPI 앱
│   ├── ai_engine.py               # Anthropic Claude 연동
│   ├── requirements.txt
│   └── Dockerfile
├── frontend/                      # React 대시보드
│   ├── src/
│   │   ├── App.jsx                # 메인 앱 (탭: Topology, Experiments, Analysis)
│   │   ├── Dashboard.jsx          # 대시보드 개요
│   │   ├── TopologyGraph.jsx      # Force-graph 시각화
│   │   ├── ExperimentForm.jsx     # 실험 생성
│   │   ├── ExperimentList.jsx     # 실험 목록
│   │   ├── AnalysisPanel.jsx      # AI 분석 패널
│   │   ├── useSSE.js              # SSE 스트리밍 훅
│   │   └── api.js                 # API 클라이언트
│   ├── Dockerfile
│   └── package.json
├── backend/                       # Legacy Python 백엔드
│   ├── main.py                    # FastAPI 진입점
│   ├── engines/                   # K8s, AWS, AI 엔진
│   ├── safety/                    # 롤백, 스냅샷, 가드레일
│   ├── probes/                    # 헬스 프로브
│   ├── routers/                   # API 라우트
│   └── tests/                     # 159개 테스트, 87%+ 커버리지
├── cli/
│   └── chaosduck.py               # Click 기반 CLI
├── prometheus/                    # Prometheus 설정
├── grafana/                       # Grafana 대시보드 & 프로비저닝
├── docker-compose.yml             # 멀티 서비스 오케스트레이션
├── Makefile                       # Python 린트/테스트 단축키
├── pyproject.toml                 # Ruff, pytest 설정
├── .env.example                   # 환경 변수 템플릿
└── CLAUDE.md                      # AI 어시스턴트 가이드라인
```

## 안전 규칙

모든 카오스 액션은 엄격한 안전 보장을 따릅니다:

1. **롤백 튜플** — 모든 엔진 함수는 `(result, rollback_fn)` 반환
2. **LIFO 롤백** — 롤백 작업은 역순으로 실행
3. **긴급 정지** — 모든 활성 실험의 롤백 트리거
4. **프로덕션 가드** — 프로덕션 네임스페이스는 명시적 확인 필요
5. **타임아웃 적용** — 모든 실험에 최대 타임아웃 (기본: 120초)
6. **블래스트 반경 검증** — 주입 전 영향 범위 제한 확인
7. **상태 스냅샷** — 모든 변경 전 전체 상태 캡처

## 카오스 유형

### Kubernetes
| 유형 | 설명 |
|------|------|
| `pod_delete` | 대상 Pod 삭제 |
| `network_latency` | 네트워크 지연 주입 (tc netem) |
| `network_loss` | 패킷 손실 주입 |
| `cpu_stress` | stress-ng를 통한 CPU 스트레스 |
| `memory_stress` | stress-ng를 통한 메모리 스트레스 |

### AWS
| 유형 | 설명 |
|------|------|
| `ec2_stop` | EC2 인스턴스 중지 |
| `rds_failover` | RDS 페일오버 트리거 |
| `route_blackhole` | VPC 라우트 블랙홀 주입 |

## 라이선스

MIT
