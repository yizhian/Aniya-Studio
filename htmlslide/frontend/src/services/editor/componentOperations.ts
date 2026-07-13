import type { Component, Editor } from 'grapesjs';
import type { translations } from '../../i18n/translations';

type T = typeof translations["zh-CN"];

export function getImageBlockHtml(t: T): string {
  return `<img src="https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?q=80&w=800&auto=format&fit=crop" style="display:block;width:480px;max-width:100%;height:auto;border-radius:8px;border:1px dashed rgba(255,255,255,0.25)" alt="${t.elementLabels.image}" />`;
}

export function getVideoBlockHtml(): string {
  return '<div style="width:640px;max-width:100%;position:relative;padding-bottom:56.25%;height:0;overflow:hidden;border-radius:8px;border:1px dashed rgba(255,255,255,0.25)"><iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ" style="position:absolute;top:0;left:0;width:100%;height:100%;border:none;border-radius:8px" allowfullscreen></iframe></div>';
}

function getActiveSlideComponent(editor: Editor): Component | undefined {
  const slides = editor.getWrapper()?.find('.slide.active');
  if (slides?.length) return slides[0] as Component;

  const doc = editor.Canvas?.getDocument?.();
  const slideEl = doc?.querySelector('.slide.active');
  if (!slideEl) return undefined;

  const getComponent = (editor as Editor & {
    Components?: { getComponent?: (el: Element) => Component | undefined };
  }).Components?.getComponent;
  return getComponent?.(slideEl);
}

function isDescendantOf(ancestor: Component, node: Component): boolean {
  let current: Component | undefined | null = node;
  while (current) {
    if (current === ancestor) return true;
    current = current.parent?.() ?? null;
  }
  return false;
}

function normalizeDroppedMedia(component: Component): void {
  const el = component.getEl();
  const tag = el?.tagName?.toLowerCase() ?? '';
  const type = component.get('type');

  if (tag === 'img' || type === 'image') {
    const src = component.getAttributes?.()?.src || el?.getAttribute('src') || '';
    if (!src || src.startsWith('data:image/svg')) {
      component.addAttributes({
        src: 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?q=80&w=800&auto=format&fit=crop',
        alt: 'Image',
      });
    }
    const style = (component.getStyle?.() || {}) as Record<string, string>;
    const width = style.width ?? el?.style?.width ?? '';
    const height = style.height ?? el?.style?.height ?? '';
    if (!width && !height) {
      component.addStyle?.({
        display: 'block',
        width: '480px',
        'max-width': '100%',
        height: 'auto',
        'min-height': '120px',
        'border-radius': '8px',
        border: '1px dashed rgba(255,255,255,0.25)',
      });
    }
    return;
  }

  const iframe = el?.querySelector?.('iframe') ?? (tag === 'iframe' ? el : null);
  if (iframe) {
    const host = tag === 'iframe' ? component : component.closest?.('div') ?? component.parent?.();
    if (host) {
      const hostStyle = (host.getStyle?.() || {}) as Record<string, string>;
      if (!hostStyle.width && !hostStyle['max-width']) {
        host.addStyle?.({
          width: '640px',
          'max-width': '100%',
          position: 'relative',
          'padding-bottom': '56.25%',
          height: '0',
          overflow: 'hidden',
          'border-radius': '8px',
          border: '1px dashed rgba(255,255,255,0.25)',
        });
      }
    }
  }
}

/** Reparent block-drop results into the visible slide when deck markup is present. */
function ensureComponentInActiveSlide(
  editor: Editor,
  component: Component,
): Component {
  const activeSlide = getActiveSlideComponent(editor);
  if (!activeSlide || isDescendantOf(activeSlide, component)) {
    return component;
  }

  const moved = activeSlide.append(component)?.[0];
  if (moved) {
    editor.select(moved);
    return moved;
  }
  return component;
}

/** Normalize slide placement and visible dimensions after toolbar block drops. */
export function finalizeBlockDrop(editor: Editor, component: Component | undefined): void {
  if (!component) return;
  const placed = ensureComponentInActiveSlide(editor, component);
  normalizeDroppedMedia(placed);
  editor.select(placed);
  editor.refresh({ tools: true });
}

export function getSelectedImageData(editor: Editor) {
  const selected = editor.getSelected();
  const el = selected?.getEl();
  if (!selected || !el || el.tagName.toLowerCase() !== 'img') return null;
  return {
    src: selected.getAttributes().src || el.getAttribute('src') || '',
    alt: selected.getAttributes().alt || el.getAttribute('alt') || '',
  };
}

export function replaceSelectedImage(
  editor: Editor,
  payload: { src: string; alt?: string },
  t: T,
) {
  const selected = editor.getSelected();
  const el = selected?.getEl();
  if (!selected || !el || el.tagName.toLowerCase() !== 'img') {
    throw new Error(t.errors.selectImageBeforeReplace);
  }

  const src = payload.src.trim();
  if (!src) throw new Error(t.errors.imageUrlRequired);
  selected.addAttributes({ src, alt: el.getAttribute('alt') || 'Image' });
}
