import asyncio
from unittest.mock import AsyncMock, patch

from models.experiment import ChaosType, ExperimentConfig
from probes.base import ProbeMode, ProbeResult
from safety.guardrails import ExperimentContext
from safety.health_check import HealthCheckLoop


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
        loop = HealthCheckLoop("exp1", [], interval=0, failure_threshold=1, on_failure=rollback_fn)
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
    async def test_context_starts_health_loop_for_continuous_probes(self):
        config = ExperimentConfig(
            name="test",
            chaos_type=ChaosType.POD_DELETE,
            target_namespace="default",
        )
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
            async with ExperimentContext("exp1", config, probes=[probe]) as ctx:
                assert ctx._health_loop is not None
                assert ctx._health_loop.is_running is True

    async def test_context_stops_health_loop_on_exit(self):
        config = ExperimentConfig(
            name="test",
            chaos_type=ChaosType.POD_DELETE,
            target_namespace="default",
        )
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
            async with ExperimentContext("exp1", config, probes=[probe]) as ctx:
                health_loop = ctx._health_loop
            assert health_loop.is_running is False
