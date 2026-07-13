from playwright.async_api import async_playwright

INIT_SCRIPT = """
Object.defineProperty(window, 'innerWidth', { get: function() { return 1920; }, configurable: true });
Object.defineProperty(window, 'innerHeight', { get: function() { return 1080; }, configurable: true });
Object.defineProperty(window, 'outerWidth', { get: function() { return 1920; }, configurable: true });
Object.defineProperty(window, 'outerHeight', { get: function() { return 1080; }, configurable: true });
window.__PDF_EXPORT__ = true;
"""

PRINT_CSS = """
@page {
  size: 1920px 1080px;
  margin: 0;
}

/* Reset JS-applied inline position/transform on any scaling wrapper */
html, body,
#stage, #scale-wrap, #deck, #presentation, #presentation-wrapper, #slide-wrapper {
  position: static !important;
  top: auto !important;
  left: auto !important;
  transform: none !important;
  transform-origin: unset !important;
  overflow: visible !important;
  height: auto !important;
  margin: 0 !important;
}

html, body {
  background: transparent !important;
}

/* Each slide occupies exactly one 1920×1080 page */
.slide, section.slide {
  position: relative !important;
  top: auto !important;
  left: auto !important;
  overflow: hidden !important;
  opacity: 1 !important;
  visibility: visible !important;
  width: 1920px !important;
  height: 1080px !important;
  min-height: 0 !important;
  page-break-after: always;
  break-after: page;
}
.slide:last-child, section.slide:last-child {
  page-break-after: avoid;
  break-after: auto;
}

/* backdrop-filter fails in headless print mode — disable globally */
*, *::before, *::after {
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

/* Hide all navigation chrome */
#nav-controls, #dot-nav, .nav-btn, #progress-bar-wrap,
#slide-counter, #dots, #btn-prev, #btn-next,
#nav-dots, .edge-click-left, .edge-click-right,
#prev-btn, #next-btn, #nav-prev, #nav-next,
#kb-hint, .progress-bar, #progress-bar, #counter,
.nav-dot, .dot, .nav-arrow, .slide-indicator {
  display: none !important;
}
"""


async def generate_pdf(html_content: str) -> bytes:
    async with async_playwright() as pw:
        browser = await pw.chromium.launch(
            headless=True,
            args=["--no-sandbox", "--disable-setuid-sandbox"],
        )
        context = await browser.new_context(
            viewport={"width": 1920, "height": 1080}
        )
        page = await context.new_page()
        await page.add_init_script(INIT_SCRIPT)
        await page.set_content(html_content, timeout=30000)
        await page.add_style_tag(content=PRINT_CSS)
        await page.wait_for_load_state("networkidle", timeout=30000)
        pdf_bytes = await page.pdf(
            print_background=True,
            width="1920px",
            height="1080px",
            margin={"top": "0px", "right": "0px", "bottom": "0px", "left": "0px"},
        )
        await browser.close()
        return pdf_bytes
