import asyncio
from unittest.mock import AsyncMock, patch

import pytest

from models.experiment import ChaosType, ExperimentConfig, SafetyConfig
from probes.base import ProbeMode, ProbeResult
from safety.guardrails import (
    EmergencyStopManager,
    ExperimentContext,
    HealthCheckLoop,
    emergency_stop_manager,
    require_confirmation,
    validate_blast_radius,
    with_timeout,
)


# ──────────────────────────────────────────────
# RollbackManager
# ──────────────────────────────────────────────
class TestRollbackManager:
    async def test_push_and_stack_size(self, rollback_mgr):
        rollback_mgr.push("exp1", AsyncMock(), "action-1")
        rollback_mgr.push("exp1", AsyncMock(), "action-2")
        assert rollback_mgr.get_stack_size("exp1") == 2

    async def test_lifo_order(self, rollback_mgr):
        order = []

        async def fn_a():
            order.append("a")

        async def fn_b():
            order.append("b")

        async def fn_c():
            order.append("c")

        rollback_mgr.push("exp1", fn_a, "first")
        rollback_mgr.push("exp1", fn_b, "second")
        rollback_mgr.push("exp1", fn_c, "third")

        results = await rollback_mgr.rollback("exp1")

        assert order == ["c", "b", "a"]
        assert len(results) == 3
        assert all(r["status"] == "success" for r in results)

    async def test_rollback_clears_stack(self, rollback_mgr):
        rollback_mgr.push("exp1", AsyncMock(), "action")
        await rollback_mgr.rollback("exp1")
        assert rollback_mgr.get_stack_size("exp1") == 0

    async def test_rollback_empty_experiment(self, rollback_mgr):
        results = await rollback_mgr.rollback("nonexistent")
        assert results == []

    async def test_rollback_failure_continues(self, rollback_mgr):
        ok_fn = AsyncMock(return_value="ok")
        fail_fn = AsyncMock(side_effect=RuntimeError("boom"))

        rollback_mgr.push("exp1", ok_fn, "first-ok")
        rollback_mgr.push("exp1", fail_fn, "will-fail")
        rollback_mgr.push("exp1", ok_fn, "second-ok")

        results = await rollback_mgr.rollback("exp1")
        statuses = [r["status"] for r in results]
        assert statuses == ["success", "failed", "success"]

    async def test_rollback_all(self, rollback_mgr):
        rollback_mgr.push("exp1", AsyncMock(), "a1")
        rollback_mgr.push("exp2", AsyncMock(), "a2")

        all_results = await rollback_mgr.rollback_all()
        assert "exp1" in all_results
        assert "exp2" in all_results
        assert rollback_mgr.get_active_experiments() == []

    async def test_get_active_experiments(self, rollback_mgr):
        rollback_mgr.push("exp1", AsyncMock(), "a")
        rollback_mgr.push("exp2", AsyncMock(), "b")
        active = rollback_mgr.get_active_experiments()
        assert set(active) == {"exp1", "exp2"}

    async def test_rollback_returns_result_value(self, rollback_mgr):
        async def fn():
            return {"restored": 3}

        rollback_mgr.push("exp1", fn, "restore")
        results = await rollback_mgr.rollback("exp1")
        assert results[0]["result"] == {"restored": 3}


# ──────────────────────────────────────────────
# SnapshotManager
# ──────────────────────────────────────────────
class TestSnapshotManager:
    async def test_capture_k8s_snapshot(self, snapshot_mgr):
        snap = await snapshot_mgr.capture_k8s_snapshot("exp1", "default", {"app": "web"})
        assert snap["type"] == "k8s"
        assert snap["namespace"] == "default"
        assert "captured_at" in snap
        assert "pods" in snap["resources"]

    async def test_capture_aws_snapshot(self, snapshot_mgr):
        snap = await snapshot_mgr.capture_aws_snapshot("exp1", "ec2", "i-123")
        assert snap["type"] == "aws"
        assert snap["resource_id"] == "i-123"

    async def test_get_snapshot(self, snapshot_mgr):
        await snapshot_mgr.capture_k8s_snapshot("exp1", "ns")
        assert snapshot_mgr.get_snapshot("exp1") is not None
        assert snapshot_mgr.get_snapshot("nonexistent") is None

    async def test_delete_snapshot(self, snapshot_mgr):
        await snapshot_mgr.capture_k8s_snapshot("exp1", "ns")
        snapshot_mgr.delete_snapshot("exp1")
        assert snapshot_mgr.get_snapshot("exp1") is None

    async def test_delete_nonexistent(self, snapshot_mgr):
        snapshot_mgr.delete_snapshot("nope")  # should not raise

    async def test_list_snapshots(self, snapshot_mgr):
        await snapshot_mgr.capture_k8s_snapshot("exp1", "ns1")
        await snapshot_mgr.capture_aws_snapshot("exp2", "ec2", "i-1")
        snaps = snapshot_mgr.list_snapshots()
        assert set(snaps.keys()) == {"exp1", "exp2"}


# ──────────────────────────────────────────────
# Guardrails
# ──────────────────────────────────────────────
class TestEmergencyStopManager:
    def test_initial_state(self):
        mgr = EmergencyStopManager()
        assert mgr.is_triggered() is False

    def test_trigger_and_reset(self):
        mgr = EmergencyStopManager()
        mgr.trigger()
        assert mgr.is_triggered() is True
        mgr.reset()
        assert mgr.is_triggered() is False

    async def test_wait_resolves(self):
        mgr = EmergencyStopManager()

        async def trigger_later():
            await asyncio.sleep(0.05)
            mgr.trigger()

        asyncio.create_task(trigger_later())
        await asyncio.wait_for(mgr.wait(), timeout=1.0)
        assert mgr.is_triggered() is True


class TestWithTimeout:
    async def test_completes_within_timeout(self):
        @with_timeout(seconds=5)
        async def fast():
            return "done"

        assert await fast() == "done"

    async def test_raises_on_timeout(self):
        @with_timeout(seconds=1)
        async def slow():
            await asyncio.sleep(10)

        with pytest.raises(TimeoutError, match="timed out"):
            await slow()

    async def test_timeout_clamped_to_max(self):
        @with_timeout(seconds=999)
        async def fn():
            return "ok"

        # Should not raise; decorator clamps to 120s
        assert await fn() == "ok"


class TestRequireConfirmation:
    async def test_blocks_prod_namespace(self):
        @require_confirmation("prod*")
        async def dangerous(config=None):
            return "executed"

        config = ExperimentConfig(
            name="t",
            chaos_type=ChaosType.POD_DELETE,
            target_namespace="production",
            safety=SafetyConfig(require_confirmation=False),
        )
        with pytest.raises(PermissionError, match="production"):
            await dangerous(config=config)

    async def test_allows_with_confirmation(self):
        @require_confirmation("prod*")
        async def dangerous(config=None):
            return "executed"

        config = ExperimentConfig(
            name="t",
            chaos_type=ChaosType.POD_DELETE,
            target_namespace="production",
            safety=SafetyConfig(require_confirmation=True),
        )
        result = await dangerous(config=config)
        assert result == "executed"

    async def test_non_matching_namespace(self):
        @require_confirmation("prod*")
        async def safe(config=None):
            return "ok"

        config = ExperimentConfig(
            name="t",
            chaos_type=ChaosType.POD_DELETE,
            target_namespace="staging",
        )
        assert await safe(config=config) == "ok"

    async def test_no_config(self):
        @require_confirmation("prod*")
        async def fn(config=None):
            return "ok"

        assert await fn() == "ok"


class TestValidateBlastRadius:
    def test_within_limit(self):
        assert validate_blast_radius(1, 10, 0.3) is True

    def test_exceeds_limit(self):
        assert validate_blast_radius(5, 10, 0.3) is False

    def test_zero_total(self):
        assert validate_blast_radius(0, 0, 0.3) is True

    def test_exact_boundary(self):
        # ratio == max_ratio (not >) => passes validation
        assert validate_blast_radius(3, 10, 0.3) is True
        # just over the limit => fails
        assert validate_blast_radius(4, 10, 0.3) is False


class TestExperimentContext:
    async def test_captures_snapshot(self, sample_config):
        with patch("safety.guardrails.snapshot_manager") as mock_snap:
            mock_snap.capture_k8s_snapshot = AsyncMock()
            async with ExperimentContext("exp1", sample_config):
                pass
            mock_snap.capture_k8s_snapshot.assert_called_once()

    async def test_blocks_when_emergency_stop(self, sample_config):
        emergency_stop_manager.trigger()
        with pytest.raises(RuntimeError, match="Emergency stop"):
            async with ExperimentContext("exp1", sample_config):
                pass

    async def test_rollback_on_exception(self, sample_config):
        with (
            patch("safety.guardrails.snapshot_manager") as mock_snap,
            patch("safety.guardrails.rollback_manager") as mock_rb,
        ):
            mock_snap.capture_k8s_snapshot = AsyncMock()
            mock_rb.rollback = AsyncMock()

            with pytest.raises(ValueError):
                async with ExperimentContext("exp1", sample_config):
                    raise ValueError("boom")

            mock_rb.rollback.assert_called_once_with("exp1")

    async def test_no_rollback_on_success(self, sample_config):
        with (
            patch("safety.guardrails.snapshot_manager") as mock_snap,
            patch("safety.guardrails.rollback_manager") as mock_rb,
        ):
            mock_snap.capture_k8s_snapshot = AsyncMock()
            mock_rb.rollback = AsyncMock()

            async with ExperimentContext("exp1", sample_config):
                pass

            mock_rb.rollback.assert_not_called()


# ──────────────────────────────────────────────
# HealthCheckLoop
# ──────────────────────────────────────────────
class TestHealthCheckLoop:
    def _make_probe(self, passed=True):
        """Create a mock probe that returns the given result."""
        mock = AsyncMock()
        mock.mode = "continuous"
        mock.safe_execute = AsyncMock(
            return_value=ProbeResult(
                probe_name="mock",
                probe_type="mock",
                mode=ProbeMode.CONTINUOUS,
                passed=passed,
            )
        )
        return mock

    async def test_start_and_stop(self):
        probe = self._make_probe(passed=True)
        loop = HealthCheckLoop("exp1", [probe], interval=1, failure_threshold=3)
        loop.start()
        assert loop.is_running is True
        await asyncio.sleep(0.1)
        await loop.stop()
        assert loop.is_running is False

    async def test_passing_probes_no_rollback(self):
        probe = self._make_probe(passed=True)
        rollback_fn = AsyncMock()

        loop = HealthCheckLoop(
            "exp1", [probe], interval=1, failure_threshold=3, on_failure=rollback_fn
        )
        loop.start()
        await asyncio.sleep(0.2)
        await loop.stop()

        rollback_fn.assert_not_called()

    async def test_failure_triggers_rollback(self):
        probe = self._make_probe(passed=False)
        rollback_fn = AsyncMock()

        loop = HealthCheckLoop(
            "exp1", [probe], interval=0, failure_threshold=2, on_failure=rollback_fn
        )
        loop.start()

        # Wait for threshold to be reached
        await asyncio.sleep(0.5)
        # Ensure the loop stopped itself after failure
        rollback_fn.assert_called_once()

    async def test_results_collected(self):
        probe = self._make_probe(passed=True)
        loop = HealthCheckLoop("exp1", [probe], interval=0, failure_threshold=3)
        loop.start()
        await asyncio.sleep(0.3)
        await loop.stop()
        assert len(loop.results) >= 1

    async def test_no_probes_always_passes(self):
        rollback_fn = AsyncMock()
        loop = HealthCheckLoop(
            "exp1", [], interval=0, failure_threshold=1, on_failure=rollback_fn
        )
        loop.start()
        await asyncio.sleep(0.2)
        await loop.stop()
        rollback_fn.assert_not_called()

    async def test_consecutive_reset_on_success(self):
        """After a failure followed by success, counter resets."""
        call_count = 0

        async def alternating_execute():
            nonlocal call_count
            call_count += 1
            return ProbeResult(
                probe_name="alt",
                probe_type="mock",
                mode=ProbeMode.CONTINUOUS,
                passed=(call_count % 2 == 0),  # fail, pass, fail, pass...
            )

        probe = AsyncMock()
        probe.mode = "continuous"
        probe.safe_execute = alternating_execute

        rollback_fn = AsyncMock()
        loop = HealthCheckLoop(
            "exp1", [probe], interval=0, failure_threshold=3, on_failure=rollback_fn
        )
        loop.start()
        await asyncio.sleep(0.5)
        await loop.stop()
        # Should not trigger because failures are never consecutive enough
        rollback_fn.assert_not_called()


class TestExperimentContextWithHealthCheck:
    async def test_context_starts_health_loop_for_continuous_probes(self, sample_config):
        probe = AsyncMock()
        probe.mode = "continuous"
        probe.safe_execute = AsyncMock(
            return_value=ProbeResult(
                probe_name="test",
                probe_type="mock",
                mode=ProbeMode.CONTINUOUS,
                passed=True,
            )
        )

        with patch("safety.guardrails.snapshot_manager") as mock_snap:
            mock_snap.capture_k8s_snapshot = AsyncMock()
            async with ExperimentContext("exp1", sample_config, probes=[probe]) as ctx:
                assert ctx._health_loop is not None
                assert ctx._health_loop.is_running is True

    async def test_context_stops_health_loop_on_exit(self, sample_config):
        probe = AsyncMock()
        probe.mode = "continuous"
        probe.safe_execute = AsyncMock(
            return_value=ProbeResult(
                probe_name="test",
                probe_type="mock",
                mode=ProbeMode.CONTINUOUS,
                passed=True,
            )
        )

        with patch("safety.guardrails.snapshot_manager") as mock_snap:
            mock_snap.capture_k8s_snapshot = AsyncMock()
            async with ExperimentContext("exp1", sample_config, probes=[probe]) as ctx:
                health_loop = ctx._health_loop
            assert health_loop.is_running is False
