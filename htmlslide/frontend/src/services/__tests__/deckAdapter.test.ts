import { describe, it, expect, vi } from 'vitest';
import { readDeckState, goToDeckSlide, clearEditorSelection } from '../deckAdapter';

// Minimal grapesjs Editor mock.
function createMockEditor(html: string): any {
  const doc = document.implementation.createHTMLDocument();
  doc.body.innerHTML = html;

  const select = vi.fn();
  const refresh = vi.fn();

  return {
    Canvas: {
      getDocument: () => doc,
    },
    select,
    refresh,
  };
}

describe('readDeckState', () => {
  it('returns null when there are fewer than 2 slides', () => {
    const editor = createMockEditor('<div class="slide active">1</div>');
    expect(readDeckState(editor)).toBeNull();
  });

  it('returns total and current for multiple slides', () => {
    const editor = createMockEditor(`
      <div class="slide active">1</div>
      <div class="slide">2</div>
      <div class="slide">3</div>
    `);
    const state = readDeckState(editor);
    expect(state).toEqual({ total: 3, current: 0 });
  });

  it('finds active slide index', () => {
    const editor = createMockEditor(`
      <div class="slide">1</div>
      <div class="slide active">2</div>
      <div class="slide">3</div>
    `);
    const state = readDeckState(editor);
    expect(state).toEqual({ total: 3, current: 1 });
  });

  it('defaults current to 0 when no active slide', () => {
    const editor = createMockEditor(`
      <div class="slide">1</div>
      <div class="slide">2</div>
    `);
    const state = readDeckState(editor);
    expect(state).toEqual({ total: 2, current: 0 });
  });

  it('falls back to <section> elements when no .slide', () => {
    const editor = createMockEditor(`
      <section>1</section>
      <section>2</section>
      <section>3</section>
    `);
    const state = readDeckState(editor);
    expect(state).toEqual({ total: 3, current: 0 });
  });

  it('falls back to [class*="slide"] elements', () => {
    const editor = createMockEditor(`
      <div class="myslide">1</div>
      <div class="myslide">2</div>
    `);
    const state = readDeckState(editor);
    expect(state).toEqual({ total: 2, current: 0 });
  });

  it('returns null for empty document', () => {
    const editor = createMockEditor('');
    expect(readDeckState(editor)).toBeNull();
  });

  it('returns null when Canvas.getDocument returns null', () => {
    const editor = {
      Canvas: { getDocument: () => null },
      select: vi.fn(),
      refresh: vi.fn(),
    };
    expect(readDeckState(editor)).toBeNull();
  });

  it('returns null when Canvas.getDocument throws', () => {
    const editor = {
      Canvas: { getDocument: () => { throw new Error('no document'); } },
      select: vi.fn(),
      refresh: vi.fn(),
    };
    expect(readDeckState(editor)).toBeNull();
  });
});

describe('goToDeckSlide', () => {
  it('returns null for single slide', () => {
    const editor = createMockEditor('<div class="slide active">1</div>');
    expect(goToDeckSlide(editor, 0)).toBeNull();
  });

  it('sets active class on target slide', () => {
    const editor = createMockEditor(`
      <div class="slide active">1</div>
      <div class="slide">2</div>
      <div class="slide">3</div>
    `);
    const state = goToDeckSlide(editor, 1);
    expect(state).toEqual({ total: 3, current: 1 });

    const doc = editor.Canvas.getDocument();
    const slides = doc.querySelectorAll('.slide');
    expect(slides[0].classList.contains('active')).toBe(false);
    expect(slides[1].classList.contains('active')).toBe(true);
    expect(slides[2].classList.contains('active')).toBe(false);
  });

  it('clamps index to valid range', () => {
    const editor = createMockEditor(`
      <div class="slide active">1</div>
      <div class="slide">2</div>
    `);
    // Negative → clamped to 0.
    const state = goToDeckSlide(editor, -5);
    expect(state!.current).toBe(0);

    // Too large → clamped to last.
    const state2 = goToDeckSlide(editor, 99);
    expect(state2!.current).toBe(1);
  });

  it('updates slide counter if present', () => {
    const editor = createMockEditor(`
      <div class="slide active">1</div>
      <div class="slide">2</div>
      <div class="slide">3</div>
      <span id="slide-counter">01 / 03</span>
    `);
    goToDeckSlide(editor, 2);
    const doc = editor.Canvas.getDocument();
    const counter = doc.getElementById('slide-counter');
    expect(counter!.textContent).toBe('03 / 03');
  });

  it('updates progress bar if present', () => {
    const editor = createMockEditor(`
      <div class="slide active">1</div>
      <div class="slide">2</div>
      <div class="progress-fill" style="width:50%"></div>
    `);
    goToDeckSlide(editor, 1);
    const doc = editor.Canvas.getDocument();
    const progress = doc.querySelector<HTMLElement>('.progress-fill');
    expect(progress!.style.width).toBe('100%');
  });

  it('updates dot indicators if present', () => {
    const editor = createMockEditor(`
      <div class="slide active">1</div>
      <div class="slide">2</div>
      <div class="slide">3</div>
      <span class="dot active"></span>
      <span class="dot"></span>
      <span class="dot"></span>
    `);
    goToDeckSlide(editor, 2);
    const doc = editor.Canvas.getDocument();
    const dots = doc.querySelectorAll<HTMLElement>('.dot');
    expect(dots[0].classList.contains('active')).toBe(false);
    expect(dots[1].classList.contains('active')).toBe(false);
    expect(dots[2].classList.contains('active')).toBe(true);
  });

  it('calls editor.refresh after navigation', () => {
    const editor = createMockEditor(`
      <div class="slide active">1</div>
      <div class="slide">2</div>
    `);
    goToDeckSlide(editor, 1);
    expect(editor.refresh).toHaveBeenCalled();
  });

  it('survives refresh failure gracefully', () => {
    const editor = createMockEditor(`
      <div class="slide active">1</div>
      <div class="slide">2</div>
    `);
    editor.refresh.mockImplementation(() => { throw new Error('refresh fail'); });
    const state = goToDeckSlide(editor, 1);
    expect(state).toEqual({ total: 2, current: 1 });
  });
});

describe('clearEditorSelection', () => {
  it('calls editor.select(null) and refresh', () => {
    const editor = createMockEditor('');
    clearEditorSelection(editor);
    expect(editor.select).toHaveBeenCalledWith(null);
    expect(editor.refresh).toHaveBeenCalledWith({ tools: true });
  });

  it('survives errors gracefully', () => {
    const editor = createMockEditor('');
    editor.select.mockImplementation(() => { throw new Error('no select'); });
    // Should not throw.
    expect(() => clearEditorSelection(editor)).not.toThrow();
  });
});
