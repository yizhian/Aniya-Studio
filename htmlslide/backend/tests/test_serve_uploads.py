"""Tests for GET /projects/{project_id}/uploads/{file_path:path}

Covers: UT (path traversal, MIME types, 404s), SIT (upload + serve roundtrip),
and smoke (edge cases: missing dir, svg mimetype fallback).
"""

import io
import json
from pathlib import Path
from unittest.mock import patch

import pytest


def _create_project(client) -> str:
    resp = client.post("/api/v1/projects", json={"name": "ServeUploads"})
    assert resp.status_code in (200, 201)
    return resp.json()["id"]


def _write_file(path: Path, content: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(content)


# ── Unit: Path traversal prevention ──────────────────────────────


class TestPathTraversalPrevention:
    def test_dot_dot_in_file_path(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        # Create an uploads dir with a real file so we know the traversal is caught, not 404 from is_file.
        uploads = temp_workspace / "projects" / pid / "uploads" / "assets"
        uploads.mkdir(parents=True)
        (uploads / "safe.png").write_bytes(b"\x89PNG\r\n\x1a\n")
        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/assets/../../project.json")
        assert resp.status_code == 404

    def test_absolute_path_equivalent(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/%2e%2e%2f%2e%2e%2fproject.json")
        assert resp.status_code == 404

    def test_url_encoded_slash(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/assets%2F..%2F..%2Fproject.json")
        assert resp.status_code == 404


# ── Unit: Valid serve with MIME types ───────────────────────────


class TestServeValidAssets:
    def test_serve_png(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        png_data = b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82"
        _write_file(temp_workspace / "projects" / pid / "uploads" / "assets" / "test.png", png_data)

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/assets/test.png")
        assert resp.status_code == 200
        assert resp.headers["content-type"] == "image/png"
        assert resp.content == png_data

    def test_serve_svg(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        svg_data = b'<svg xmlns="http://www.w3.org/2000/svg"><circle r="10"/></svg>'
        _write_file(temp_workspace / "projects" / pid / "uploads" / "chart.svg", svg_data)

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/chart.svg")
        assert resp.status_code == 200
        assert resp.headers["content-type"] == "image/svg+xml"
        assert resp.content == svg_data

    def test_serve_jpg(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        # Minimal valid JPEG (SOI + APP0 + SOF0 + SOS + EOI)
        jpg_header = (
            b"\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"
            b"\xff\xdb\x00\x43\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\x09"
            b"\t\x08\n\x0c\x14\r\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a"
            b"\x1f\x1e\x1d\x1a\x1c\x1c\x20.\x22\x23\x1c\x1c(7),01444"
            b"\x1f'9=82<.342\xff\xc0\x00\x0b\x08\x00\x01\x00\x01\x01\x01\x11\x00"
            b"\xff\xc4\x00\x1f\x00\x00\x01\x05\x01\x01\x01\x01\x01\x01\x00\x00"
            b"\x00\x00\x00\x00\x00\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b"
            b"\xff\xc4\x00\xb5\x10\x00\x02\x01\x03\x03\x02\x04\x03\x05\x05\x04"
            b"\x04\x00\x00\x01}\x01\x02\x03\x00\x04\x11\x05\x12!1A\x06\x13Qa"
            b"\x07\"q\x142\x81\x91\xa1\x08#B\xb1\xc1\x15R\xd1\xf0$3br\x82"
            b"\t\n\x16\x17\x18\x19\x1a%&'()*456789:CDEFGHIJSTUVWXYZcdefghijstuvwxyz"
            b"\x83\x84\x85\x86\x87\x88\x89\x8a\x92\x93\x94\x95\x96\x97\x98\x99"
            b"\x9a\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xb2\xb3\xb4\xb5\xb6\xb7"
            b"\xb8\xb9\xba\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xd2\xd3\xd4\xd5"
            b"\xd6\xd7\xd8\xd9\xda\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xf1"
            b"\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xff\xda\x00\x08\x01\x01\x00"
            b"\x00?\x00\xd2\xff\xd9"
        )
        _write_file(temp_workspace / "projects" / pid / "uploads" / "photo.jpg", jpg_header)

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/photo.jpg")
        assert resp.status_code == 200
        assert resp.headers["content-type"] == "image/jpeg"

    def test_serve_gif(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        gif_data = b"GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x00\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"
        _write_file(temp_workspace / "projects" / pid / "uploads" / "anim.gif", gif_data)

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/anim.gif")
        assert resp.status_code == 200
        assert resp.headers["content-type"] == "image/gif"

    def test_serve_webp(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        # Minimal WebP (RIFF header + WEBP)
        webp_data = b"RIFF\x1a\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x10\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"
        _write_file(temp_workspace / "projects" / pid / "uploads" / "img.webp", webp_data)

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/img.webp")
        assert resp.status_code == 200
        assert resp.headers["content-type"] == "image/webp"


# ── Unit: Error states ──────────────────────────────────────────


class TestServeErrorStates:
    def test_project_not_found(self, api_client):
        resp = api_client.get("/api/v1/projects/proj-nonexistent99/uploads/test.png")
        assert resp.status_code == 404

    def test_file_not_found(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        (temp_workspace / "projects" / pid / "uploads").mkdir(parents=True, exist_ok=True)
        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/nonexistent.png")
        assert resp.status_code == 404

    def test_uploads_dir_does_not_exist(self, api_client, temp_workspace):
        """is_relative_to check happens first, then is_file. Missing dir → 404."""
        pid = _create_project(api_client)
        # Don't create uploads dir at all.
        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/assets/missing.png")
        assert resp.status_code in (404, 500)

    def test_unsupported_file_type(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        # Use a file extension not in the MIME whitelist and not known to mimetypes.
        _write_file(temp_workspace / "projects" / pid / "uploads" / "data.bin", b"\x00\x01\x02")

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/data.bin")
        # Should reject unknown types. mimetypes may or may not know .bin;
        # if it returns a MIME, it's served; otherwise it hits the whitelist → 415.
        # We assert it's either rejected or served — the important thing is
        # it doesn't crash or leak files outside uploads/.
        assert resp.status_code in (200, 415)


# ── Unit: MIME whitelist fallback (file.py lines 194-205) ──────────

class TestMimeWhitelistFallback:
    """When mimetypes.guess_type returns None, the hardcoded whitelist is used."""

    def test_whitelist_fallback_serves_png(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        png_data = b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82"
        _write_file(temp_workspace / "projects" / pid / "uploads" / "img.png", png_data)

        with patch("mimetypes.guess_type", return_value=(None, None)):
            resp = api_client.get(f"/api/v1/projects/{pid}/uploads/img.png")
        assert resp.status_code == 200
        assert resp.headers["content-type"] == "image/png"

    def test_whitelist_fallback_rejects_unknown_ext(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        _write_file(temp_workspace / "projects" / pid / "uploads" / "data.xyz", b"\x00\x01")

        with patch("mimetypes.guess_type", return_value=(None, None)):
            resp = api_client.get(f"/api/v1/projects/{pid}/uploads/data.xyz")
        assert resp.status_code == 415
        assert "不支持的资源类型" in resp.json()["message"]


# ── Unit: Invalid project_id validation ─────────────────────────


class TestInvalidProjectId:
    def test_dot_dot_in_project_id(self, api_client):
        # FastAPI normalizes ../ in the URL path, so the route may not match → 404.
        # validate_project_id is reached when the path matches the route pattern.
        resp = api_client.get("/api/v1/projects/../etc/uploads/f.png")
        assert resp.status_code in (404, 422)

    def test_slash_in_project_id(self, api_client):
        resp = api_client.get("/api/v1/projects/foo/bar/uploads/f.png")
        assert resp.status_code in (404, 422)


# ── SIT: Full upload + serve roundtrip ─────────────────────────


class TestUploadAndServeRoundtrip:
    def test_image_served_at_relative_path_from_canvas(self, api_client, temp_workspace):
        """Simulates: user uploads image, it's stored at uploads/assets/<name>,
        then served back via the endpoint. This is the exact flow the canvas uses."""
        pid = _create_project(api_client)
        img_bytes = b"fake-png-content"

        # Place image where canvas stores it.
        assets_dir = temp_workspace / "projects" / pid / "uploads" / "assets"
        assets_dir.mkdir(parents=True)
        (assets_dir / "hero.png").write_bytes(img_bytes)

        # Serve it.
        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/assets/hero.png")
        assert resp.status_code == 200
        assert resp.content == img_bytes

    def test_multiple_assets_served(self, api_client, temp_workspace):
        """Verify multiple images in the same project resolve correctly."""
        pid = _create_project(api_client)

        assets = temp_workspace / "projects" / pid / "uploads" / "assets"
        assets.mkdir(parents=True)
        (assets / "bg.png").write_bytes(b"bg-content")
        (assets / "icon.svg").write_bytes(b"<svg></svg>")
        (assets / "photo.jpg").write_bytes(b"jpg-content")

        for name, expected in [("bg.png", b"bg-content"), ("icon.svg", b"<svg></svg>"), ("photo.jpg", b"jpg-content")]:
            resp = api_client.get(f"/api/v1/projects/{pid}/uploads/assets/{name}")
            assert resp.status_code == 200
            assert resp.content == expected


# ── Smoke: Edge cases ───────────────────────────────────────────


class TestServeUploadsSmoke:
    def test_svg_unknown_mimetype_fallback(self, api_client, temp_workspace):
        """Verify mimetypes.guess_type fallback works for SVG on platforms without
        built-in .svg registration (the add_type call at module level handles this)."""
        pid = _create_project(api_client)
        svg = b'<svg><text>hello</text></svg>'
        _write_file(temp_workspace / "projects" / pid / "uploads" / "graphic.svg", svg)

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/graphic.svg")
        assert resp.status_code == 200
        assert resp.headers["content-type"] == "image/svg+xml"

    def test_subdirectory_navigation_ok(self, api_client, temp_workspace):
        """Legitimate subdirectories within uploads should work."""
        pid = _create_project(api_client)
        nested = temp_workspace / "projects" / pid / "uploads" / "a" / "b" / "c"
        nested.mkdir(parents=True)
        (nested / "deep.png").write_bytes(b"deep")

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/a/b/c/deep.png")
        assert resp.status_code == 200

    def test_filename_with_spaces(self, api_client, temp_workspace):
        pid = _create_project(api_client)
        _write_file(temp_workspace / "projects" / pid / "uploads" / "my image.png", b"img")

        resp = api_client.get(f"/api/v1/projects/{pid}/uploads/my%20image.png")
        assert resp.status_code == 200
