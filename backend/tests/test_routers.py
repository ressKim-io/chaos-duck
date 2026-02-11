from unittest.mock import AsyncMock, MagicMock, patch


class TestChaosRouter:
    async def test_list_experiments_empty(self, client):
        resp = await client.get("/api/chaos/experiments")
        assert resp.status_code == 200
        assert resp.json() == []

    async def test_get_experiment_not_found(self, client):
        resp = await client.get("/api/chaos/experiments/nonexistent")
        assert resp.status_code == 404

    async def test_dry_run_ec2(self, client):
        with patch("routers.chaos.aws_engine") as mock_engine:
            mock_engine.stop_ec2 = AsyncMock(
                return_value=(
                    {"action": "stop_ec2", "instance_ids": ["i-1"], "dry_run": True},
                    None,
                )
            )
            resp = await client.post(
                "/api/chaos/dry-run",
                json={
                    "name": "test-dry",
                    "chaos_type": "ec2_stop",
                    "parameters": {"instance_ids": ["i-1"]},
                    "safety": {"dry_run": True},
                },
            )
        assert resp.status_code == 200
        data = resp.json()
        assert data["injection_result"]["dry_run"] is True
        assert data["status"] == "completed"

    async def test_dry_run_pod_delete(self, client):
        with patch("routers.chaos.k8s_engine") as mock_engine:
            mock_engine.pod_delete = AsyncMock(
                return_value=(
                    {"action": "pod_delete", "pods": ["p1"], "dry_run": True},
                    None,
                )
            )
            resp = await client.post(
                "/api/chaos/dry-run",
                json={
                    "name": "test-dry-k8s",
                    "chaos_type": "pod_delete",
                    "target_namespace": "default",
                    "target_labels": {"app": "nginx"},
                    "safety": {"dry_run": True},
                },
            )
        assert resp.status_code == 200
        assert resp.json()["injection_result"]["dry_run"] is True

    async def test_rollback_not_found(self, client):
        resp = await client.post("/api/chaos/experiments/fake/rollback")
        assert resp.status_code == 404

    async def test_emergency_stop_blocks_create(self, client):
        from safety.guardrails import emergency_stop_manager

        emergency_stop_manager.trigger()
        resp = await client.post(
            "/api/chaos/experiments",
            json={"name": "t", "chaos_type": "pod_delete"},
        )
        assert resp.status_code == 503

    async def test_create_experiment_full_lifecycle(self, client):
        """Test experiment creation with mocked K8s engine."""
        with (
            patch("routers.chaos.k8s_engine") as mock_k8s,
            patch("safety.guardrails.snapshot_manager") as mock_snap,
        ):
            mock_k8s.get_steady_state = AsyncMock(
                return_value={
                    "namespace": "default",
                    "pods_total": 3,
                    "pods_running": 3,
                    "pods_healthy_ratio": 1.0,
                }
            )
            mock_k8s.pod_delete = AsyncMock(
                return_value=(
                    {"action": "pod_delete", "pods": ["p1"]},
                    AsyncMock(),  # rollback_fn
                )
            )
            mock_snap.capture_k8s_snapshot = AsyncMock()

            resp = await client.post(
                "/api/chaos/experiments",
                json={
                    "name": "lifecycle-test",
                    "chaos_type": "pod_delete",
                    "target_namespace": "default",
                    "target_labels": {"app": "test"},
                },
            )

        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "completed"
        assert data["injection_result"]["action"] == "pod_delete"

    async def test_list_experiments_after_create(self, client):
        """Verify experiments are persisted in DB."""
        with (
            patch("routers.chaos.k8s_engine") as mock_k8s,
            patch("safety.guardrails.snapshot_manager") as mock_snap,
        ):
            mock_k8s.get_steady_state = AsyncMock(return_value={"pods_total": 1})
            mock_k8s.pod_delete = AsyncMock(
                return_value=({"action": "pod_delete", "pods": []}, None)
            )
            mock_snap.capture_k8s_snapshot = AsyncMock()

            await client.post(
                "/api/chaos/experiments",
                json={
                    "name": "persist-test",
                    "chaos_type": "pod_delete",
                    "target_namespace": "default",
                },
            )

        resp = await client.get("/api/chaos/experiments")
        assert resp.status_code == 200
        data = resp.json()
        assert len(data) >= 1
        assert any(e["config"]["name"] == "persist-test" for e in data)


class TestTopologyRouter:
    async def test_k8s_topology(self, client):
        with patch("routers.topology.k8s_engine") as mock_engine:
            mock_engine.get_topology = AsyncMock(
                return_value=MagicMock(
                    nodes=[],
                    edges=[],
                    timestamp=None,
                    model_dump=lambda **kw: {"nodes": [], "edges": [], "timestamp": None},
                )
            )
            resp = await client.get("/api/topology/k8s")
        assert resp.status_code == 200

    async def test_steady_state(self, client):
        with patch("routers.topology.k8s_engine") as mock_engine:
            mock_engine.get_steady_state = AsyncMock(
                return_value={"namespace": "default", "pods_total": 2, "pods_running": 2}
            )
            resp = await client.get("/api/topology/steady-state")
        assert resp.status_code == 200
        assert resp.json()["pods_total"] == 2


class TestAnalysisRouter:
    async def test_analyze_not_found(self, client):
        resp = await client.post("/api/analysis/experiment/fake")
        assert resp.status_code == 404

    async def test_generate_hypotheses(self, client):
        with patch("routers.analysis.ai_engine") as mock_engine:
            mock_engine.generate_hypothesis = AsyncMock(
                return_value="Pods will restart within 30s."
            )
            resp = await client.post(
                "/api/analysis/hypotheses",
                json={"topology": {}, "target": "nginx", "chaos_type": "pod_delete"},
            )
        assert resp.status_code == 200
        assert "hypothesis" in resp.json()

    async def test_generate_report(self, client):
        with patch("routers.analysis.ai_engine") as mock_engine:
            mock_engine.generate_report = AsyncMock(return_value="# Report")
            resp = await client.post(
                "/api/analysis/report",
                json={"experiment": {}, "analysis": None},
            )
        assert resp.status_code == 200
        assert resp.json()["report"] == "# Report"

    async def test_generate_experiments(self, client):
        with patch("routers.analysis.ai_engine") as mock_engine:
            mock_engine.generate_experiments = AsyncMock(
                return_value=[
                    {
                        "name": "kill-nginx",
                        "chaos_type": "pod_delete",
                        "target_namespace": "default",
                        "target_labels": {"app": "nginx"},
                        "parameters": {},
                        "description": "Test pod recovery",
                    }
                ]
            )
            resp = await client.post(
                "/api/analysis/generate-experiments",
                json={"topology": {"nodes": []}, "target_namespace": "default", "count": 1},
            )
        assert resp.status_code == 200
        data = resp.json()
        assert data["count"] == 1
        assert data["experiments"][0]["name"] == "kill-nginx"
