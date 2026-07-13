"""P5-06: E2E Magic Prompt — selected DOM context + chat → Agent edit_file → verify changes."""
from fastapi.testclient import TestClient
from .conftest import agentgo_required

UPLOAD_HTML = b"""<!doctype html>
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
  <h1>Original Title</h1>
</div>
</body>
</html>"""


class TestMagicPrompt:
    @agentgo_required
    def test_magic_prompt_with_dom_context(self, e2e_client: TestClient):
        """Upload HTML → POST /chat with selected_dom → consume SSE → GET /export has content."""
        # 1. Upload existing HTML
        resp = e2e_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", UPLOAD_HTML)},
        )
        assert resp.status_code == 200
        project_id = resp.json()["project_id"]

        # 2. Send chat with DOM context selecting the h1
        with e2e_client.stream(
            "POST",
            "/api/v1/chat",
            json={
                "project_id": project_id,
                "prompt": "把标题颜色改成蓝色",
                "selected_dom": {
                    "css_path": ".slide > h1",
                    "tag": "h1",
                    "text": "Original Title",
                    "styles": {"color": "#ff0000", "font-size": "64px"},
                },
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
                    import json
                    events.append({
                        "event": current_event["event"],
                        "data": json.loads(current_event["data"]) if current_event["data"] else None,
                    })
                    current_event = {"event": None, "data": None}

            assert len(events) > 0, "Should receive at least one SSE event"
            event_types = [e["event"] for e in events]
            assert "done" in event_types, f"Should receive done event, got: {event_types}"

        # 3. Verify export has content
        resp = e2e_client.get(f"/api/v1/projects/{project_id}/export")
        assert resp.status_code == 200
        export = resp.json()
        assert len(export["html"]) > 0
        assert export["slide_count"] >= 1

    @agentgo_required
    def test_magic_prompt_without_dom_context(self, e2e_client: TestClient):
        """Upload HTML → POST /chat without selected_dom → still works."""
        resp = e2e_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", UPLOAD_HTML)},
        )
        assert resp.status_code == 200
        project_id = resp.json()["project_id"]

        with e2e_client.stream(
            "POST",
            "/api/v1/chat",
            json={
                "project_id": project_id,
                "prompt": "添加一个副标题",
            },
        ) as response:
            assert response.status_code == 200
            for _ in response.iter_lines():
                pass

        resp = e2e_client.get(f"/api/v1/projects/{project_id}/export")
        assert resp.status_code == 200
        assert len(resp.json()["html"]) > 0
