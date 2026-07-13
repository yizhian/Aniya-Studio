/**
 * Tests for editorApi.ts — focused on the import pipeline:
 *   - ANIYA_DECK_SHELL_CSS constant
 *   - importHtmlDocument normalization + shell injection
 *   - normalizeToSlides behaviour
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Editor } from 'grapesjs';

// We import the whole module so that our mock canvas doc can be wired in.
import * as api from '../editorApi';

// ── helpers ──────────────────────────────────────────────

/** Build a minimal mock GrapesJS Editor whose Canvas.getDocument()
 *  returns a live jsdom Document that reflects the last setComponents() call. */
function mockEditor(): Editor {
  const canvasDoc = document.implementation.createHTMLDocument('canvas');

  const ed = {
    setStyle: vi.fn(),
    setComponents: vi.fn((html: string) => {
      canvasDoc.body.innerHTML = html;
    }),
    getHtml: vi.fn(() => canvasDoc.body.innerHTML),
    getCss: vi.fn(() => ''),
    Canvas: {
      getDocument: vi.fn(() => canvasDoc),
    },
    refresh: vi.fn(),
    select: vi.fn(),
    getWrapper: vi.fn(() => null),
  } as unknown as Editor;

  return ed;
}

/** Helper to read the CSS that was passed to editor.setStyle (last call). */
function lastSetStyleCss(ed: Editor): string {
  const fn = (ed as any).setStyle as ReturnType<typeof vi.fn>;
  const calls = fn.mock.calls;
  if (calls.length === 0) return '';
  return calls[calls.length - 1][0] as string;
}

/** Helper to read the HTML that was passed to editor.setComponents (last call). */
function lastSetComponentsHtml(ed: Editor): string {
  const fn = (ed as any).setComponents as ReturnType<typeof vi.fn>;
  const calls = fn.mock.calls;
  if (calls.length === 0) return '';
  return calls[calls.length - 1][0] as string;
}

const mockT = { errors: { importHtmlEmpty: "errors.importHtmlEmpty", unknownComponentType: "errors.unknownComponentType" }, elementLabels: { heading1: "HEADING 1", heading2: "HEADING 2", editableText: "EDITABLE", tryNow: "TRY NOW" } } as any;

// ── ANIYA_DECK_SHELL_CSS ─────────────────────────────────

describe('ANIYA_DECK_SHELL_CSS', () => {
  it('targets .slide:not(.active) with display:none !important (verified via import behaviour)', () => {
    // Import minimal HTML — the shell CSS is always injected at the end of setStyle.
    const ed = mockEditor();
    api.importHtmlDocument(ed, '<!DOCTYPE html><html><head></head><body><p>x</p></body></html>', mockT);
    const css = lastSetStyleCss(ed).trim();
    // Shell must be the only rule.
    expect(css).toBe('html[data-aniya-editor] .slide:not(.active) { display: none !important; }');
    // Spot-checks on the rule content.
    expect(css).toContain(':not(.active)');
    expect(css).toContain('!important');
    // Must NOT force display:block on .active (author controls display of active slide).
    expect(css).not.toMatch(/\.slide\.active\s*\{/);
  });
});

// ── importHtmlDocument ────────────────────────────────────

describe('importHtmlDocument', () => {
  let editor: Editor;

  beforeEach(() => {
    editor = mockEditor();
  });

  // ── basic guards ──

  it('throws on empty HTML', () => {
    expect(() => api.importHtmlDocument(editor, '', mockT)).toThrow('errors.importHtmlEmpty');
    expect(() => api.importHtmlDocument(editor, '   ', mockT)).toThrow('errors.importHtmlEmpty');
  });

  // ── shell CSS injection ──

  it('always appends ANIYA_DECK_SHELL_CSS to setStyle, even for trivial HTML', () => {
    api.importHtmlDocument(editor, '<!DOCTYPE html><html><head></head><body><p>hi</p></body></html>', mockT);

    const css = lastSetStyleCss(editor);
    expect(css).toContain('.slide:not(.active)');
    expect(css).toContain('display: none !important');
    // Shell must be last.
    expect(css.trimEnd()).toMatch(/display:\s*none\s*!important;\s*\}$/);
  });

  it('shell CSS is the only CSS rule when source has no styles', () => {
    api.importHtmlDocument(editor, '<!DOCTYPE html><html><head></head><body><p>text</p></body></html>', mockT);

    const css = lastSetStyleCss(editor);
    // Should be exactly the shell rule (with maybe a trailing newline).
    expect(css.trim()).toBe('html[data-aniya-editor] .slide:not(.active) { display: none !important; }');
  });

  // ── <head> <style> extraction ──

  it('extracts <head> <style> text into setStyle before the shell', () => {
    const html = `<!DOCTYPE html>
<html><head>
<style>body { color: red; }</style>
</head><body><div class="slide">slide 1</div></body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const css = lastSetStyleCss(editor);
    expect(css).toContain('body { color: red; }');
    // head style must come before shell.
    const headIdx = css.indexOf('body { color: red; }');
    const shellIdx = css.indexOf('.slide:not(.active)');
    expect(headIdx).toBeLessThan(shellIdx);
  });

  // ── custom-code <style> normalization ──

  it('extracts <style> from inside custom-code blocks and merges them into setStyle', () => {
    const html = `<!DOCTYPE html>
<html><head></head><body>
<div class="slide">slide</div>
<div data-gjs-type="custom-code">
  <style>.nav { color: white; }</style>
  <script>console.log(1);</script>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const css = lastSetStyleCss(editor);
    expect(css).toContain('.nav { color: white; }');
  });

  it('merge order: head CSS → custom-code CSS → shell CSS', () => {
    const html = `<!DOCTYPE html>
<html><head>
<style>/* head */ .a { color: red; }</style>
</head><body>
<div data-gjs-type="custom-code">
  <style>/* cc */ .b { color: blue; }</style>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const css = lastSetStyleCss(editor);
    const headIdx = css.indexOf('.a {');
    const ccIdx = css.indexOf('.b {');
    const shellIdx = css.indexOf('.slide:not(.active)');

    expect(headIdx).toBeGreaterThan(-1);
    expect(ccIdx).toBeGreaterThan(-1);
    expect(shellIdx).toBeGreaterThan(-1);
    expect(headIdx).toBeLessThan(ccIdx);
    expect(ccIdx).toBeLessThan(shellIdx);
  });

  // ── <style> removal from body ──

  it('removes ALL <style> elements from body before setComponents', () => {
    const html = `<!DOCTYPE html>
<html><head>
<style>.head-style { }</style>
</head><body>
<div data-gjs-type="custom-code">
  <style>.cc-style { }</style>
  <script>console.log(1);</script>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = lastSetComponentsHtml(editor);
    // Neither head style nor custom-code style should appear in the body HTML.
    expect(bodyHtml).not.toContain('<style>');
    expect(bodyHtml).not.toContain('.head-style');
    expect(bodyHtml).not.toContain('.cc-style');
  });

  // ── script preservation ──

  it('preserves <script> inside custom-code blocks', () => {
    const html = `<!DOCTYPE html>
<html><head></head><body>
<div data-gjs-type="custom-code">
  <script>console.log("kept");</script>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = lastSetComponentsHtml(editor);
    expect(bodyHtml).toContain('<script>');
    expect(bodyHtml).toContain('console.log("kept")');
  });

  it('strips <script> outside custom-code blocks', () => {
    const html = `<!DOCTYPE html>
<html><head></head><body>
<script>console.log("removed");</script>
<div data-gjs-type="custom-code">
  <script>console.log("kept");</script>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = lastSetComponentsHtml(editor);
    expect(bodyHtml).not.toContain('console.log("removed")');
    expect(bodyHtml).toContain('console.log("kept")');
  });

  // ── realistic scenario: proj-95a3482a62c1 style ──

  it('proj-95: normalizes navigation CSS trapped inside custom-code', () => {
    // This is the actual failure mode: all CSS inside custom-code, no <head> <style>.
    const html = `<!DOCTYPE html>
<html><head></head><body>
<div id="stage"><div id="scale-wrap">
<section class="slide" style="display:none">Slide 1</section>
<section class="slide" style="display:none">Slide 2</section>
</div></div>
<div data-gjs-type="custom-code">
<style>
  .slide.active { display: block; }
  #nav-controls { position: fixed; }
</style>
<script>
  var slides = document.querySelectorAll('.slide');
  function showSlide(idx) {
    for (var i = 0; i < slides.length; i++) {
      slides[i].classList.remove('active');
      slides[i].style.display = 'none';
    }
    slides[idx].classList.add('active');
    slides[idx].style.display = 'block';
  }
</script>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const css = lastSetStyleCss(editor);
    // Custom-code styles must be normalized into CssComposer.
    expect(css).toContain('.slide.active');
    expect(css).toContain('#nav-controls');
    // Shell CSS must still be last.
    expect(css).toContain('.slide:not(.active) { display: none !important; }');

    // Body must preserve the script (inside custom-code).
    const bodyHtml = lastSetComponentsHtml(editor);
    expect(bodyHtml).toContain('showSlide');
    // But styles must NOT be in body.
    expect(bodyHtml).not.toContain('<style>');
  });

  // ── realistic scenario: proj-214395c01f15 style ──

  it('proj-214: head styles preserved, custom-code styles merged, shell last', () => {
    const html = `<!DOCTYPE html>
<html><head>
<style>
  .slide { display: none; }
  .slide.active { display: block; }
  .slide-t1 { display: flex; }
  #nav { position: fixed; }
</style>
</head><body>
<div class="slide slide-t1 active">Slide 1</div>
<div class="slide slide-m2">Slide 2</div>
<div data-gjs-type="custom-code">
<style>.nav-extra { opacity: 0.9; }</style>
<script>var current = 0;</script>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const css = lastSetStyleCss(editor);

    // Head styles present.
    expect(css).toContain('.slide-t1 { display: flex; }');
    // Custom-code styles merged after head.
    expect(css).toContain('.nav-extra { opacity: 0.9; }');
    // Merge order verified.
    const t1Idx = css.indexOf('.slide-t1');
    const extraIdx = css.indexOf('.nav-extra');
    const shellIdx = css.indexOf('.slide:not(.active)');
    expect(t1Idx).toBeLessThan(extraIdx);
    expect(extraIdx).toBeLessThan(shellIdx);

    // Shell is present and will override .slide { display: none } for non-active slides.
    expect(css).toContain('display: none !important');

    // Body has script but no style tags.
    const bodyHtml = lastSetComponentsHtml(editor);
    expect(bodyHtml).toContain('var current = 0;');
    expect(bodyHtml).not.toContain('<style>');
  });

  // ── edge: multiple custom-code blocks ──

  it('extracts styles from multiple custom-code blocks', () => {
    const html = `<!DOCTYPE html>
<html><head></head><body>
<div data-gjs-type="custom-code"><style>.a { }</style></div>
<div data-gjs-type="custom-code"><style>.b { }</style></div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const css = lastSetStyleCss(editor);
    expect(css).toContain('.a { }');
    expect(css).toContain('.b { }');
  });

  // ── edge: no aniyaFonts plugin ──

  it('does not throw when aniyaFonts plugin is absent', () => {
    expect(() => {
      api.importHtmlDocument(editor, '<!DOCTYPE html><html><head></head><body><p>hi</p></body></html>', mockT);
    }).not.toThrow();
  });
});

// ── normalizeToSlides ─────────────────────────────────────

describe('normalizeToSlides', () => {
  let editor: Editor;

  beforeEach(() => {
    editor = mockEditor();
  });

  function setCanvasBody(html: string) {
    const doc = (editor as any).Canvas.getDocument() as Document;
    doc.body.innerHTML = html;
  }

  it('returns early when .slide elements already exist', () => {
    setCanvasBody('<div class="slide">s1</div><div class="slide">s2</div>');

    // Call importHtmlDocument which internally calls normalizeToSlides.
    // We verify normalizeToSlides did NOT re-wrap by checking setComponents
    // was only called once (by importHtmlDocument itself).
    const setComp = (editor as any).setComponents as ReturnType<typeof vi.fn>;
    setComp.mockClear();

    setCanvasBody('<div class="slide">existing</div>');
    // Trigger normalizeToSlides by importing HTML that already has .slide.
    api.importHtmlDocument(
      editor,
      '<!DOCTYPE html><html><head></head><body><div class="slide">s1</div><div class="slide">s2</div></body></html>',
      mockT
    );

    // importHtmlDocument calls setComponents once. If normalizeToSlides also
    // called setComponents, we'd see two calls. Let's verify the body contains
    // our original slides (not wrapped in extra slide divs).
    const bodyHtml = lastSetComponentsHtml(editor);
    // Should contain the .slide elements directly, NOT wrapped in another .slide.
    const slideMatches = bodyHtml.match(/class="slide"/g);
    // The outer slide divs from normalizeToSlides would add extra .slide classes.
    // Since .slide already existed, it returns early → no extra wrapping.
    expect(slideMatches?.length).toBe(2);
  });

  it('does NOT inject inline .slide CSS when creating wrapper', () => {
    // Import HTML without .slide → normalizeToSlides wraps content.
    // Verify the wrapper HTML does NOT contain a <style> tag.
    const html = `<!DOCTYPE html>
<html><head></head><body>
<section><h1>Page 1</h1></section>
<section><h1>Page 2</h1></section>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = lastSetComponentsHtml(editor);
    // normalizeToSlides wraps sections in .slide divs.
    expect(bodyHtml).toContain('class="slide active"');
    // Must NOT contain the old inline CSS fragment.
    expect(bodyHtml).not.toContain('.slide{display:none}');
    expect(bodyHtml).not.toContain('.slide.active{display:block}');
  });

  it('wraps <section> elements in .slide divs when no .slide exists', () => {
    // Clear canvas.
    const doc = (editor as any).Canvas.getDocument() as Document;
    doc.body.innerHTML = '';

    const html = `<!DOCTYPE html>
<html><head></head><body>
<section><h1>A</h1></section>
<section><h1>B</h1></section>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = lastSetComponentsHtml(editor);
    // Should see .slide wrapper.
    expect(bodyHtml).toContain('slide active');
    expect(bodyHtml).toContain('slide-wrapper');
    // Both sections wrapped.
    const slideCount = (bodyHtml.match(/class="slide/g) || []).length;
    expect(slideCount).toBe(2);
  });

  it('first wrapped slide gets active class', () => {
    const doc = (editor as any).Canvas.getDocument() as Document;
    doc.body.innerHTML = '';

    const html = `<!DOCTYPE html>
<html><head></head><body>
<section><h1>One</h1></section>
<section><h1>Two</h1></section>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = lastSetComponentsHtml(editor);
    expect(bodyHtml).toContain('class="slide active"');
  });

  // ── smoke: shell CSS covers both normalizeToSlides and existing .slide paths ──

  it('shell CSS is in setStyle regardless of whether normalizeToSlides wraps or returns early', () => {
    // Path A: HTML without .slide (normalizeToSlides wraps).
    const edA = mockEditor();
    api.importHtmlDocument(
      edA,
      '<!DOCTYPE html><html><head></head><body><section>s1</section><section>s2</section></body></html>',
      mockT
    );
    expect(lastSetStyleCss(edA)).toContain('.slide:not(.active)');

    // Path B: HTML with .slide (normalizeToSlides returns early).
    const edB = mockEditor();
    api.importHtmlDocument(
      edB,
      '<!DOCTYPE html><html><head></head><body><div class="slide">s1</div><div class="slide">s2</div></body></html>',
      mockT
    );
    expect(lastSetStyleCss(edB)).toContain('.slide:not(.active)');
  });
});

// ── importHtmlDocument: editor.refresh() call ──────────────

describe('importHtmlDocument calls editor.refresh()', () => {
  it('calls editor.refresh() after completing the import pipeline', () => {
    const editor = mockEditor();
    const refreshFn = (editor as any).refresh as ReturnType<typeof vi.fn>;
    refreshFn.mockClear();

    api.importHtmlDocument(
      editor,
      '<!DOCTYPE html><html><head></head><body><div class="slide active">s1</div></body></html>',
      mockT
    );

    expect(refreshFn).toHaveBeenCalledTimes(1);
  });

  it('calls refresh even when normalizeToSlides returns early (slides exist)', () => {
    const editor = mockEditor();
    const refreshFn = (editor as any).refresh as ReturnType<typeof vi.fn>;
    refreshFn.mockClear();

    api.importHtmlDocument(
      editor,
      '<!DOCTYPE html><html><head></head><body><div class="slide">a</div><div class="slide">b</div></body></html>',
      mockT
    );

    expect(refreshFn).toHaveBeenCalled();
  });

  it('calls refresh when normalizeToSlides wraps content (no slides)', () => {
    const editor = mockEditor();
    // For this test, normalizeToSlides wraps sections into slides,
    // which also calls editor.refresh() internally. We verify that
    // the final refresh from importHtmlDocument itself also fires.
    const refreshFn = (editor as any).refresh as ReturnType<typeof vi.fn>;
    refreshFn.mockClear();

    api.importHtmlDocument(
      editor,
      '<!DOCTYPE html><html><head></head><body><section><h1>A</h1></section></body></html>',
      mockT
    );

    // normalizeToSlides calls refresh() once when it wraps content,
    // and importHtmlDocument calls it again at the end.
    expect(refreshFn).toHaveBeenCalled();
  });
});

// ── fixViewportContainers: cross-document getComputedStyle ──

describe('fixViewportContainers', () => {
  /** Create a canvas doc with a mock defaultView whose getComputedStyle is spiable. */
  function mockCanvasDocWithView() {
    const canvasDoc = document.implementation.createHTMLDocument('canvas');

    // jsdom's standalone document has defaultView: null. We attach a mock view
    // so that fixViewportContainers can call defaultView.getComputedStyle().
    const mockView = { getComputedStyle: vi.fn((_el: Element) => ({})) };
    Object.defineProperty(canvasDoc, 'defaultView', {
      value: mockView,
      writable: true,
      configurable: true,
    });

    return canvasDoc;
  }

  it('uses canvas document defaultView.getComputedStyle (not main window)', () => {
    const canvasDoc = mockCanvasDocWithView();
    const mockView = canvasDoc.defaultView as { getComputedStyle: ReturnType<typeof vi.fn> };
    mockView.getComputedStyle.mockReturnValue({});

    // Insert a viewport container into the canvas doc.
    const stage = canvasDoc.createElement('div');
    stage.id = 'stage';
    canvasDoc.body.appendChild(stage);

    const editor = {
      setStyle: vi.fn(),
      setComponents: vi.fn((html: string) => {
        canvasDoc.body.innerHTML = html;
      }),
      getHtml: vi.fn(() => canvasDoc.body.innerHTML),
      getCss: vi.fn(() => ''),
      Canvas: {
        getDocument: vi.fn(() => canvasDoc),
      },
      refresh: vi.fn(),
      select: vi.fn(),
      getWrapper: vi.fn(() => null),
    } as unknown as Editor;

    // Import HTML that includes a #stage element.
    api.importHtmlDocument(
      editor,
      '<!DOCTYPE html><html><head></head><body><div id="stage"><div class="slide active">x</div></div></body></html>',
      mockT
    );

    // The mock getComputedStyle (on the canvas doc's defaultView) should
    // have been called, NOT the main window's getComputedStyle.
    expect(mockView.getComputedStyle).toHaveBeenCalled();
  });

  it('resets viewport container positioning when computed style is absolute+centered', () => {
    const canvasDoc = mockCanvasDocWithView();
    const mockView = canvasDoc.defaultView as { getComputedStyle: ReturnType<typeof vi.fn> };

    // Simulate #stage having centered-absolute computed style.
    mockView.getComputedStyle.mockImplementation((el: Element) => {
      if ((el as HTMLElement).id === 'stage') {
        return {
          position: 'absolute',
          top: '50%',
          left: '50%',
          right: '',
          bottom: '',
          transform: '',
        } as unknown as CSSStyleDeclaration;
      }
      return {} as CSSStyleDeclaration;
    });

    const editor = {
      setStyle: vi.fn(),
      setComponents: vi.fn((html: string) => {
        canvasDoc.body.innerHTML = html;
      }),
      getHtml: vi.fn(() => canvasDoc.body.innerHTML),
      getCss: vi.fn(() => ''),
      Canvas: {
        getDocument: vi.fn(() => canvasDoc),
      },
      refresh: vi.fn(),
      select: vi.fn(),
      getWrapper: vi.fn(() => null),
    } as unknown as Editor;

    api.importHtmlDocument(
      editor,
      '<!DOCTYPE html><html><head></head><body><div id="stage"><div class="slide active">x</div></div></body></html>',
      mockT
    );

    // After fix, #stage should have position:relative and cleared offsets.
    const stageEl = canvasDoc.getElementById('stage');
    expect(stageEl).not.toBeNull();
    expect(stageEl!.style.position).toBe('relative');
    expect(stageEl!.style.top).toBe('');
    expect(stageEl!.style.left).toBe('');
    expect(stageEl!.style.margin).toMatch(/^0(px)? auto$/);
  });

  it('does not modify viewport container when computed style is not centered-absolute', () => {
    const canvasDoc = mockCanvasDocWithView();
    const mockView = canvasDoc.defaultView as { getComputedStyle: ReturnType<typeof vi.fn> };

    // Simulate normal flow positioning (not centered-absolute).
    mockView.getComputedStyle.mockReturnValue({
      position: 'relative',
      top: '0px',
      left: '0px',
    } as unknown as CSSStyleDeclaration);

    const editor = {
      setStyle: vi.fn(),
      setComponents: vi.fn((html: string) => {
        canvasDoc.body.innerHTML = html;
      }),
      getHtml: vi.fn(() => canvasDoc.body.innerHTML),
      getCss: vi.fn(() => ''),
      Canvas: {
        getDocument: vi.fn(() => canvasDoc),
      },
      refresh: vi.fn(),
      select: vi.fn(),
      getWrapper: vi.fn(() => null),
    } as unknown as Editor;

    api.importHtmlDocument(
      editor,
      '<!DOCTYPE html><html><head></head><body><div id="stage"><div class="slide active">x</div></div></body></html>',
      mockT
    );

    // No change expected → inline style should not be modified.
    const stageEl = canvasDoc.getElementById('stage');
    expect(stageEl!.style.position).toBe('');
  });
});

// ── inline style preservation through import cycle ─────────

describe('inline style preservation', () => {
  it('preserves inline styles on elements after importHtmlDocument', () => {
    const canvasDoc = document.implementation.createHTMLDocument('canvas');
    const editor = {
      setStyle: vi.fn(),
      setComponents: vi.fn((html: string) => {
        canvasDoc.body.innerHTML = html;
      }),
      getHtml: vi.fn(() => canvasDoc.body.innerHTML),
      getCss: vi.fn(() => ''),
      Canvas: {
        getDocument: vi.fn(() => canvasDoc),
      },
      refresh: vi.fn(),
      select: vi.fn(),
      getWrapper: vi.fn(() => null),
    } as unknown as Editor;

    const html = `<!DOCTYPE html>
<html><head></head><body>
<div class="slide active">
  <h1 style="font-size: 48px; font-weight: 700; color: #ff0000;">标题</h1>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = (editor as any).getHtml();
    expect(bodyHtml).toContain('font-size: 48px');
    expect(bodyHtml).toContain('font-weight: 700');
    expect(bodyHtml).toContain('color: #ff0000');
  });

  it('preserves inline styles on text components after slide-wrapping', () => {
    const canvasDoc = document.implementation.createHTMLDocument('canvas');
    const editor = {
      setStyle: vi.fn(),
      setComponents: vi.fn((html: string) => {
        canvasDoc.body.innerHTML = html;
      }),
      getHtml: vi.fn(() => canvasDoc.body.innerHTML),
      getCss: vi.fn(() => ''),
      Canvas: {
        getDocument: vi.fn(() => canvasDoc),
      },
      refresh: vi.fn(),
      select: vi.fn(),
      getWrapper: vi.fn(() => null),
    } as unknown as Editor;

    // HTML without .slide → normalizeToSlides will wrap in slides.
    const html = `<!DOCTYPE html>
<html><head></head><body>
<section>
  <h2 style="font-size: 32px; font-weight: 600; color: #3366ff;">二级标题</h2>
  <p style="font-size: 18px; line-height: 1.8; color: #333333;">正文内容</p>
</section>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = (editor as any).getHtml();
    // Inline styles must survive the normalizeToSlides wrapping.
    expect(bodyHtml).toContain('font-size: 32px');
    expect(bodyHtml).toContain('color: #3366ff');
    expect(bodyHtml).toContain('line-height: 1.8');
  });

  it('preserves multiple styled elements with distinct styles', () => {
    const canvasDoc = document.implementation.createHTMLDocument('canvas');
    const editor = {
      setStyle: vi.fn(),
      setComponents: vi.fn((html: string) => {
        canvasDoc.body.innerHTML = html;
      }),
      getHtml: vi.fn(() => canvasDoc.body.innerHTML),
      getCss: vi.fn(() => ''),
      Canvas: {
        getDocument: vi.fn(() => canvasDoc),
      },
      refresh: vi.fn(),
      select: vi.fn(),
      getWrapper: vi.fn(() => null),
    } as unknown as Editor;

    const html = `<!DOCTYPE html>
<html><head></head><body>
<div class="slide active">
  <h1 style="font-size: 48px; color: #ff0000;">红标题</h1>
  <p style="font-size: 16px; color: #cccccc;">灰段落</p>
  <button style="background-color: #ddb7ff; color: #490080; border-radius: 8px;">按钮</button>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = (editor as any).getHtml();
    // Each element's unique styles must be intact.
    expect(bodyHtml).toContain('color: #ff0000');
    expect(bodyHtml).toContain('color: #cccccc');
    expect(bodyHtml).toContain('color: #490080');
    expect(bodyHtml).toContain('background-color: #ddb7ff');
    expect(bodyHtml).toContain('border-radius: 8px');
  });

  it('handles nested elements with cascading inline styles', () => {
    const canvasDoc = document.implementation.createHTMLDocument('canvas');
    const editor = {
      setStyle: vi.fn(),
      setComponents: vi.fn((html: string) => {
        canvasDoc.body.innerHTML = html;
      }),
      getHtml: vi.fn(() => canvasDoc.body.innerHTML),
      getCss: vi.fn(() => ''),
      Canvas: {
        getDocument: vi.fn(() => canvasDoc),
      },
      refresh: vi.fn(),
      select: vi.fn(),
      getWrapper: vi.fn(() => null),
    } as unknown as Editor;

    const html = `<!DOCTYPE html>
<html><head></head><body>
<div class="slide active">
  <div style="padding: 20px; background-color: #1a1a1a; border-radius: 12px;">
    <h3 style="font-size: 24px; color: #ffffff;">嵌套标题</h3>
    <span style="font-size: 14px; color: #888888;">描述文字</span>
  </div>
</div>
</body></html>`;

    api.importHtmlDocument(editor, html, mockT);

    const bodyHtml = (editor as any).getHtml();
    expect(bodyHtml).toContain('background-color: #1a1a1a');
    expect(bodyHtml).toContain('border-radius: 12px');
    expect(bodyHtml).toContain('color: #ffffff');
    expect(bodyHtml).toContain('color: #888888');
  });
});

// ── serializePublishHtml ────────────────────────────────────

function mockEditorForSerialize(opts?: { getHtml?: string; getCss?: string }): Editor {
  const canvasDoc = document.implementation.createHTMLDocument('canvas');
  return {
    setStyle: vi.fn(),
    setComponents: vi.fn((html: string) => { canvasDoc.body.innerHTML = html; }),
    getHtml: vi.fn(() => opts?.getHtml ?? canvasDoc.body.innerHTML),
    getCss: vi.fn(() => opts?.getCss ?? ''),
    Canvas: { getDocument: vi.fn(() => canvasDoc) },
    refresh: vi.fn(),
    select: vi.fn(),
    getWrapper: vi.fn(() => null),
  } as unknown as Editor;
}

describe('serializePublishHtml', () => {
  it('preserves author <style> blocks unchanged', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active"><h1>Hello</h1></div>',
      getCss: '.custom { color: red; }',
    });
    const persisted = `<!DOCTYPE html><html><head>
<style>body { color: blue; }</style>
</head><body>
<div class="slide active">old</div>
</body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    // Author style block must be preserved verbatim.
    expect(result).toContain('body { color: blue; }');
  });

  it('strips legacy unscoped shell from author AND delta blocks', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.foo { color: red; }',
    });
    api.resetCssBaseline(editor);
    // Both author and delta blocks contain legacy unscoped shell.
    const persisted = `<!DOCTYPE html><html><head>
<style>.slide:not(.active) { display: none !important; } .foo { color: red; }</style>
<style data-aniya-editor-delta="">.bar { opacity: 0; } .slide:not(.active) { display: none !important; }</style>
</head><body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    // Legacy shell must be stripped from ALL <style> blocks.
    expect(result).not.toMatch(/\.slide:not\(\.active\)\s*\{[^}]*display\s*:\s*none[^}]*!important/);
    // Non-shell rules preserved.
    expect(result).toContain('.bar { opacity: 0; }');
    expect(result).toContain('.foo { color: red; }');
  });

  it('replaces old delta when CSS is dirty (branch A)', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.new-rule { color: green; }',
    });
    // prime baseline to something different so cssDirty = true
    api.resetCssBaseline(editor);
    // now change getCss
    (editor.getCss as ReturnType<typeof vi.fn>).mockReturnValue('.new-rule { color: green; } .changed { }');

    const persisted = `<!DOCTYPE html><html><head>
<style>/* author */</style>
<style data-aniya-editor-delta="">/* old delta */ .old { color: gray; }</style>
</head><body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    // Old delta removed, new delta written.
    expect(result).not.toContain('.old { color: gray; }');
    expect(result).toContain('.new-rule { color: green; }');
    expect(result).toContain('/* author */');
    // Exactly one delta block.
    const deltaCount = (result.match(/data-aniya-editor-delta/g) || []).length;
    expect(deltaCount).toBe(1);
  });

  it('keeps existing delta when CSS unchanged (branch C)', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.same { color: red; }',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head>
<style>/* author */</style>
<style data-aniya-editor-delta="">.delta-rule { color: blue; }</style>
</head><body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    // Existing delta preserved.
    expect(result).toContain('.delta-rule { color: blue; }');
    const deltaCount = (result.match(/data-aniya-editor-delta/g) || []).length;
    expect(deltaCount).toBe(1);
  });

  it('first materialization: writes delta when no existing delta but cleanDelta non-empty (branch B)', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.custom-code-style { color: orange; }',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head>
<style>/* author */</style>
</head><body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    // First materialization writes a delta.
    expect(result).toContain('data-aniya-editor-delta');
    expect(result).toContain('.custom-code-style { color: orange; }');
    const deltaCount = (result.match(/data-aniya-editor-delta/g) || []).length;
    expect(deltaCount).toBe(1);
  });

  it('replaces body with editor.getHtml()', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active"><h1>New Body</h1></div>',
      getCss: '',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head></head>
<body><div class="slide active">Old Body</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    expect(result).toContain('<h1>New Body</h1>');
    expect(result).not.toContain('Old Body');
  });

  it('preserves body scripts from persistedHtml (fingerprint dedup)', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head></head><body>
<div class="slide active">x</div>
<script>/* unique-script-123 */ console.log("persisted");</script>
</body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    expect(result).toContain('unique-script-123');
  });

  it('injects Google Fonts <link> for detected font families', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active"><h1 style="font-family:\'Roboto\',sans-serif">Hi</h1></div>',
      getCss: '',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head></head>
<body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    expect(result).toContain('fonts.googleapis.com');
    expect(result).toContain('Roboto');
  });

  it('removes data-aniya-editor attribute from <html>', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html data-aniya-editor=""><head></head>
<body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    expect(result).not.toContain('data-aniya-editor');
  });

  it('produces byte-identical output for two calls with same inputs', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active"><p>A</p></div>',
      getCss: '.rule { color: red; }',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head>
<style>/* author */ .a { }</style>
</head><body><div class="slide active">x</div></body></html>`;

    const r1 = api.serializePublishHtml(persisted, editor);
    const r2 = api.serializePublishHtml(persisted, editor);
    expect(r1).toBe(r2);
  });
});

// ── delta lifecycle ─────────────────────────────────────────

describe('delta lifecycle', () => {
  it('first save without prior delta: exactly one delta block', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.custom { color: red; }',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head>
<style>/* author */</style>
</head><body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    const deltaCount = (result.match(/data-aniya-editor-delta/g) || []).length;
    expect(deltaCount).toBe(1);
  });

  it('second save without CSS edits: exactly one delta block (preserved)', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.same { color: blue; }',
    });
    // First "save" primes the baseline and creates a delta.
    api.resetCssBaseline(editor);
    const persisted1 = `<!DOCTYPE html><html><head>
<style>/* author */</style>
</head><body><div class="slide active">x</div></body></html>`;
    const result1 = api.serializePublishHtml(persisted1, editor);
    // Now "save" again without CSS changes.
    const result2 = api.serializePublishHtml(result1, editor);
    const deltaCount = (result2.match(/data-aniya-editor-delta/g) || []).length;
    expect(deltaCount).toBe(1);
  });

  it('CSS edit then save: exactly one delta with updated content', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.v1 { color: red; }',
    });
    api.resetCssBaseline(editor);
    const persisted1 = `<!DOCTYPE html><html><head>
<style>/* author */</style>
</head><body><div class="slide active">x</div></body></html>`;
    const result1 = api.serializePublishHtml(persisted1, editor);

    // Simulate CSS edit: change getCss (baseline not reset yet — that happens on save success).
    (editor.getCss as ReturnType<typeof vi.fn>).mockReturnValue('.v2 { color: green; }');

    const result2 = api.serializePublishHtml(result1, editor);
    const deltaCount = (result2.match(/data-aniya-editor-delta/g) || []).length;
    expect(deltaCount).toBe(1);
    expect(result2).toContain('.v2 { color: green; }');
    expect(result2).not.toContain('.v1 { color: red; }');
  });

  it('third save without CSS edits: still exactly one delta block', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.stable { color: blue; }',
    });
    api.resetCssBaseline(editor);
    const persisted1 = `<!DOCTYPE html><html><head>
<style>/* author */</style>
</head><body><div class="slide active">x</div></body></html>`;
    let result = api.serializePublishHtml(persisted1, editor);
    // Second save (no CSS change).
    result = api.serializePublishHtml(result, editor);
    // Third save (no CSS change).
    result = api.serializePublishHtml(result, editor);
    const deltaCount = (result.match(/data-aniya-editor-delta/g) || []).length;
    expect(deltaCount).toBe(1);
  });
});

// ── stripAllLegacyShell (via serializePublishHtml) ──────────

describe('stripAllLegacyShell', () => {
  it('removes unscoped shell from author <style> blocks', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head>
<style>.slide:not(.active) { display: none !important; } .keep { color: red; }</style>
</head><body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    expect(result).not.toMatch(/display\s*:\s*none\s*!important/);
    expect(result).toContain('.keep { color: red; }');
  });

  it('removes unscoped shell from <style data-aniya-editor-delta> blocks', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.author { color: blue; }',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head>
<style>/* author */</style>
<style data-aniya-editor-delta="">.delta-keep { opacity: 1; } .slide:not(.active) { display: none !important; }</style>
</head><body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    expect(result).toContain('.delta-keep { opacity: 1; }');
    expect(result).not.toMatch(/display\s*:\s*none\s*!important/);
  });

  it('preserves opacity/transform rules', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.slide { opacity: 0; transition: opacity 0.5s; } .slide.active { opacity: 1; }',
    });
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head>
<style>.slide { opacity: 0; transform: translateX(100%); }</style>
</head><body><div class="slide active">x</div></body></html>`;

    const result = api.serializePublishHtml(persisted, editor);
    expect(result).toContain('opacity');
    expect(result).toContain('transform');
  });
});

// ── resetCssBaseline ────────────────────────────────────────

describe('resetCssBaseline', () => {
  it('updates baseline so subsequent cssDirty comparison is relative to last save', () => {
    const editor = mockEditorForSerialize({
      getHtml: '<div class="slide active">x</div>',
      getCss: '.rule-a { color: red; }',
    });
    // Initial baseline.
    api.resetCssBaseline(editor);

    const persisted = `<!DOCTYPE html><html><head></head>
<body><div class="slide active">x</div></body></html>`;
    // First serialize: branch B (first materialization) since no delta.
    const result1 = api.serializePublishHtml(persisted, editor);
    expect(result1).toContain('data-aniya-editor-delta');

    // Change CSS.
    (editor.getCss as ReturnType<typeof vi.fn>).mockReturnValue('.rule-a { color: red; } .rule-b { color: blue; }');
    const result2 = api.serializePublishHtml(result1, editor);
    // Baseline hasn't changed → cssDirty=true → branch A replaces delta.
    expect(result2).toContain('.rule-b');

    // Reset baseline (simulates save success).
    api.resetCssBaseline(editor);
    // Now serialize again without CSS changes → should preserve delta.
    const result3 = api.serializePublishHtml(result2, editor);
    const deltaCount = (result3.match(/data-aniya-editor-delta/g) || []).length;
    expect(deltaCount).toBe(1);
  });
});

// ── SVG removal regression ─────────────────────────────────────

describe('SVG removal regression', () => {
  it('extractSvgTextData is not exported', () => {
    expect((api as any).extractSvgTextData).toBeUndefined();
  });

  it('applySvgTextUpdates is not exported', () => {
    expect((api as any).applySvgTextUpdates).toBeUndefined();
  });
});
