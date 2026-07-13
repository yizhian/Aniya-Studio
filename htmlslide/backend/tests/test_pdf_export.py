import io
from unittest.mock import AsyncMock, patch

FAKE_PDF = b"%PDF-1.4 fake pdf content"


class TestExportPdf:
    def test_export_pdf_returns_pdf(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Deck"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><style>.slide{}</style><div class='slide'>x</div></html>"
        upload_resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )
        assert upload_resp.status_code == 200

        with patch(
            "src.api.export.generate_pdf", new_callable=AsyncMock
        ) as mock_gen:
            mock_gen.return_value = FAKE_PDF
            resp = api_client.get(f"/api/v1/projects/{pid}/export/pdf")

        assert resp.status_code == 200
        assert resp.headers["content-type"] == "application/pdf"
        assert "attachment" in resp.headers["content-disposition"]
        assert resp.content == FAKE_PDF
        mock_gen.assert_awaited_once()

    def test_export_pdf_empty_project_returns_404(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Empty"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/export/pdf")
        assert resp.status_code == 404
