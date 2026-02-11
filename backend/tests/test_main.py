from main import emergency_stop_event


class TestHealthEndpoint:
    async def test_health_check(self, client):
        emergency_stop_event.clear()
        resp = await client.get("/health")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "healthy"
        assert data["emergency_stop"] is False

    async def test_health_with_emergency_stop(self, client):
        emergency_stop_event.set()
        resp = await client.get("/health")
        data = resp.json()
        assert data["emergency_stop"] is True
        emergency_stop_event.clear()


class TestEmergencyStopEndpoint:
    async def test_trigger_emergency_stop(self, client):
        resp = await client.post("/emergency-stop")
        assert resp.status_code == 200
        assert resp.json()["status"] == "emergency_stop_triggered"
        emergency_stop_event.clear()
