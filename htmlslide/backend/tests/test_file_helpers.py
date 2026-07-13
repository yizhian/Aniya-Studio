"""Direct tests for _name_from_file and _validate_upload helpers."""
import io

import pytest
from fastapi import HTTPException
from fastapi import UploadFile

from src.api.file import _name_from_file, _validate_upload


class TestNameFromFile:
    def test_normal_filename(self):
        assert _name_from_file("deck.html") == "deck"

    def test_dotfile(self):
        assert _name_from_file(".html") == "未命名项目"

    def test_empty_string(self):
        assert _name_from_file("") == "未命名项目"

    def test_none_input(self):
        assert _name_from_file(None) == "未命名项目"

    def test_no_extension(self):
        assert _name_from_file("README") == "README"

    def test_multiple_dots(self):
        assert _name_from_file("my.deck.html") == "my.deck"

    def test_htm_extension(self):
        assert _name_from_file("index.htm") == "index"


class TestValidateUpload:
    def _make_upload(self, filename="test.html"):
        return UploadFile(filename=filename, file=io.BytesIO(b""))

    def test_html_mime_passes(self):
        content = b"<!doctype html><html><body><p>hello</p></body></html>"
        upload = self._make_upload("test.html")
        _validate_upload(upload, content)  # should not raise

    def test_text_plain_mime_passes(self):
        """text/plain is accepted (some editors save HTML as text/plain)."""
        content = b"<div>hello</div>"
        upload = self._make_upload("test.html")
        _validate_upload(upload, content)

    def test_wrong_extension_fails(self):
        upload = self._make_upload("data.txt")
        with pytest.raises(HTTPException) as exc:
            _validate_upload(upload, b"data")
        assert exc.value.status_code == 400

    def test_pdf_mime_fails(self):
        upload = self._make_upload("test.html")
        # PDF magic bytes
        content = b"%PDF-1.4\n%\xe2\xe3\xcf\xd3\n1 0 obj\n<<\n/Type /Catalog\n>>\nendobj\n"
        with pytest.raises(HTTPException) as exc:
            _validate_upload(upload, content)
        assert exc.value.status_code == 400

    def test_binary_mime_fails(self):
        upload = self._make_upload("test.html")
        # PNG magic bytes
        content = b"\x89PNG\r\n\x1a\n" + b"\x00" * 200
        with pytest.raises(HTTPException) as exc:
            _validate_upload(upload, content)
        assert exc.value.status_code == 400

    def test_empty_content_fails(self):
        upload = self._make_upload("test.html")
        with pytest.raises(HTTPException) as exc:
            _validate_upload(upload, b"")
        assert exc.value.status_code == 422

    def test_whitespace_only_content_fails(self):
        upload = self._make_upload("test.html")
        with pytest.raises(HTTPException) as exc:
            _validate_upload(upload, b"   \n\t  ")
        assert exc.value.status_code == 422

    def test_no_filename_fails(self):
        upload = UploadFile(filename=None, file=io.BytesIO(b"data"))
        with pytest.raises(HTTPException) as exc:
            _validate_upload(upload, b"data")
        assert exc.value.status_code == 422

    def test_oversized_file_fails(self):
        from src.config import settings
        upload = self._make_upload("big.html")
        content = b"x" * (settings.upload_max_size + 1)
        with pytest.raises(HTTPException) as exc:
            _validate_upload(upload, content)
        assert exc.value.status_code == 413

    def test_htm_extension_passes(self):
        content = b"<html><body>hello</body></html>"
        upload = UploadFile(filename="page.htm", file=io.BytesIO(b""))
        _validate_upload(upload, content)  # should not raise
