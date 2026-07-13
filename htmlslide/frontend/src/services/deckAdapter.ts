import type { Editor } from "grapesjs";

export type DeckState = {
  total: number;
  current: number;
};

function getCanvasDocument(editor: Editor): Document | null {
  try {
    return editor.Canvas.getDocument() ?? null;
  } catch {
    return null;
  }
}

function getSlides(editor: Editor): HTMLElement[] {
  const doc = getCanvasDocument(editor);
  if (!doc) return [];
  const slides = Array.from(doc.querySelectorAll<HTMLElement>(".slide"));
  if (slides.length > 0) return slides;

  // Fallback: AI-generated HTML may not use class="slide".
  // Try <section> elements (most common slide container), then any element
  // whose class contains "slide".
  const sections = Array.from(doc.querySelectorAll<HTMLElement>("section"));
  if (sections.length >= 2) return sections;

  return Array.from(doc.querySelectorAll<HTMLElement>("[class*=\"slide\"]"));
}

export function readDeckState(editor: Editor): DeckState | null {
  const slides = getSlides(editor);
  if (slides.length <= 1) return null;
  const activeIndex = slides.findIndex((slide) => slide.classList.contains("active"));
  return {
    total: slides.length,
    current: activeIndex >= 0 ? activeIndex : 0,
  };
}

export function goToDeckSlide(editor: Editor, nextIndex: number): DeckState | null {
  const slides = getSlides(editor);
  if (slides.length <= 1) return null;

  const current = Math.max(0, Math.min(slides.length - 1, nextIndex));
  slides.forEach((slide, index) => slide.classList.toggle("active", index === current));

  const currentText = String(current + 1).padStart(2, "0");
  const totalText = String(slides.length).padStart(2, "0");

  const doc = getCanvasDocument(editor);
  if (doc) {
    const counter = doc.getElementById("slide-counter");
    if (counter) counter.textContent = `${currentText} / ${totalText}`;

    const progress = doc.querySelector<HTMLElement>(".progress-fill");
    if (progress) progress.style.width = `${((current + 1) / slides.length) * 100}%`;

    doc.querySelectorAll<HTMLElement>(".dot").forEach((dot, index) => {
      dot.classList.toggle("active", index === current);
    });
  }

  // Trigger GrapesJS re-render so the canvas reflects the class toggle.
  try {
    editor.refresh();
  } catch {
    // refresh may fail during rapid navigation; the DOM is already updated.
  }

  return { total: slides.length, current };
}

export function clearEditorSelection(editor: Editor) {
  try {
    editor.select(null);
    editor.refresh({ tools: true });
  } catch {
    // GrapesJS can be between iframe refresh phases while components are added/removed.
  }
}
