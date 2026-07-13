"""P5-05: E2E — create project, chat with AgentGo, consume SSE, verify export."""
import json

from fastapi.testclient import TestClient
from .conftest import agentgo_required

MINIMAL_HTML = b"""<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<style>
  .slide { background: #111; color: #fff; width: 1920px; height: 1080px; }
  h1 { font-size: 64px; color: #ff0000; }
</style>
</head>
<body>
<div class="slide active">
  <h1>Hello</h1>
</div>
</body>
</html>"""


class TestCreateAndGenerate:
    @agentgo_required
    def test_full_generate_flow(self, e2e_client: TestClient):
        """Upload HTML → POST /chat SSE → consume events → GET /export has html."""
        # 1. Upload HTML (ensures workspace has content for export)
        resp = e2e_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", MINIMAL_HTML)},
        )
        assert resp.status_code == 200
        project_id = resp.json()["project_id"]

        # 2. Start chat with AgentGo
        with e2e_client.stream(
            "POST",
            "/api/v1/chat",
            json={
                "project_id": project_id,
                "prompt": "把标题改成Hello World，字体改为蓝色",
            },
        ) as response:
            assert response.status_code == 200

            events = []
            current_event = {"event": None, "data": None}

            for line in response.iter_lines():
                line_str = line if isinstance(line, str) else line.decode("utf-8")

                if line_str.startswith("event: "):
                    current_event["event"] = line_str[7:].strip()
                elif line_str.startswith("data: "):
                    current_event["data"] = line_str[6:]
                elif line_str == "" and current_event["event"] is not None:
                    events.append({
                        "event": current_event["event"],
                        "data": json.loads(current_event["data"]) if current_event["data"] else None,
                    })
                    current_event = {"event": None, "data": None}

            assert len(events) > 0, "Should receive at least one SSE event"

            event_types = [e["event"] for e in events]
            assert "done" in event_types, f"Should receive done event, got: {event_types}"

            # Verify done event structure (version may be "unknown" when AgentGo
            # writes to a separate Docker volume from the BFF temp workspace)
            done_event = next(e for e in events if e["event"] == "done")
            assert done_event["data"]["project_id"] == project_id
            assert "version" in done_event["data"]

        # 3. Export — should return 200 with HTML content (from upload)
        resp = e2e_client.get(f"/api/v1/projects/{project_id}/export")
        assert resp.status_code == 200
        export = resp.json()
        assert len(export["html"]) > 0

    @agentgo_required
    def test_multiple_rounds_produce_no_errors(self, e2e_client: TestClient):
        """Two chat rounds → both complete without errors."""
        resp = e2e_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", MINIMAL_HTML)},
        )
        assert resp.status_code == 200
        project_id = resp.json()["project_id"]

        # Round 1: modify
        with e2e_client.stream(
            "POST",
            "/api/v1/chat",
            json={
                "project_id": project_id,
                "prompt": "把标题改成Round1",
            },
        ) as resp1:
            assert resp1.status_code == 200
            events1 = []
            current = {"event": None, "data": None}
            for line in resp1.iter_lines():
                ls = line if isinstance(line, str) else line.decode("utf-8")
                if ls.startswith("event: "):
                    current["event"] = ls[7:].strip()
                elif ls.startswith("data: "):
                    current["data"] = ls[6:]
                elif ls == "" and current["event"] is not None:
                    events1.append(current["event"])
                    current = {"event": None, "data": None}
            assert "done" in events1, f"Round 1 should get done, got: {events1}"

        # Round 2: modify again
        with e2e_client.stream(
            "POST",
            "/api/v1/chat",
            json={
                "project_id": project_id,
                "prompt": "把标题改成Round2",
            },
        ) as resp2:
            assert resp2.status_code == 200
            events2 = []
            current = {"event": None, "data": None}
            for line in resp2.iter_lines():
                ls = line if isinstance(line, str) else line.decode("utf-8")
                if ls.startswith("event: "):
                    current["event"] = ls[7:].strip()
                elif ls.startswith("data: "):
                    current["data"] = ls[6:]
                elif ls == "" and current["event"] is not None:
                    events2.append(current["event"])
                    current = {"event": None, "data": None}
            assert "done" in events2, f"Round 2 should get done, got: {events2}"

        # Versions endpoint returns 200 with valid structure
        ver_resp = e2e_client.get(f"/api/v1/projects/{project_id}/versions")
        assert ver_resp.status_code == 200
        assert "versions" in ver_resp.json()
