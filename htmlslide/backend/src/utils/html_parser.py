import re


def extract_style_content(html: str) -> str:
    """Extract and merge all <style> tag contents."""
    matches = re.findall(r"<style[^>]*>(.*?)</style>", html, re.DOTALL)
    return "\n".join(m.strip() for m in matches if m.strip())


def strip_style_tags(html: str) -> str:
    """Remove <style> tags, returning the processed HTML."""
    return re.sub(r"<style[^>]*>.*?</style>", "", html, flags=re.DOTALL)


def count_slides(html: str) -> int:
    """Count .slide elements in HTML."""
    return len(re.findall(r'class=["\'][^"\']*\bslide\b', html))


def has_slide_elements(html: str) -> bool:
    """Check whether the HTML contains .slide elements (for is_deck field)."""
    return count_slides(html) > 0


def extract_title_from_html(html: str) -> str:
    """Extract text content from <title>...</title>, empty string if missing."""
    m = re.search(r"<title[^>]*>(.*?)</title>", html, re.DOTALL | re.IGNORECASE)
    return m.group(1).strip() if m else ""


def inject_base_tag(html: str, base_href: str) -> str:
    """Inject <base> tag so relative asset paths resolve via the given href."""
    base_tag = f'<base href="{base_href}">'
    if "<head>" in html:
        return html.replace("<head>", f"<head>{base_tag}", 1)
    if "<HEAD>" in html:
        return html.replace("<HEAD>", f"<HEAD>{base_tag}", 1)
    return base_tag + html
