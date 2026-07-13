import type { Editor } from 'grapesjs';
import { buildGoogleFontsURL, extractFontFamilies } from '../../plugins/aniyaFonts';
import type { translations } from '../../i18n/translations';

type T = typeof translations["zh-CN"];

// ─── History ────────────────────────────────────────────────────────────────────

export function undo(editor: Editor) {
  editor.UndoManager?.undo();
}

export function redo(editor: Editor) {
  editor.UndoManager?.redo();
}

export function canUndo(editor: Editor) {
  return editor.UndoManager?.hasUndo() || false;
}

export function canRedo(editor: Editor) {
  return editor.UndoManager?.hasRedo() || false;
}

// ─── State queries ──────────────────────────────────────────────────────────────

export function setHtmlAndCss(editor: Editor, html: string, css = '') {
  editor.setComponents(html || '');
  editor.setStyle(css || '');
}

// ─── Shell CSS constants ────────────────────────────────────────────────────────

// Platform deck shell CSS — single source of truth for slide visibility.
const ANIYA_DECK_SHELL_CSS = 'html[data-aniya-editor] .slide:not(.active) { display: none !important; }';

// Regex for stripping shell CSS from serialized output.
const SCOPED_SHELL_RE =
  /html\[data-aniya-editor\]\s*\.slide:not\(\.active\)\s*\{[^}]*!important[^}]*\}/g;
const LEGACY_SHELL_RE =
  /\.slide:not\(\.active\)\s*\{[^}]*display\s*:\s*none[^}]*!important[^}]*\}/gi;

// Module-level baseline: reset on every importHtmlDocument and save success.
let cssBaseline = '';

// ─── Shell & CSS helpers ────────────────────────────────────────────────────────

function stripShells(css: string): string {
  return css.replace(SCOPED_SHELL_RE, '').replace(LEGACY_SHELL_RE, '').trim();
}

function normalizeCss(css: string): string {
  return css.replace(/\s+/g, ' ').replace(/\s*([{}:;,])\s*/g, '$1').trim();
}

function stripAllLegacyShell(doc: Document): void {
  doc.querySelectorAll('style').forEach(style => {
    const cleaned = (style.textContent || '').replace(LEGACY_SHELL_RE, '').trim();
    if (cleaned !== style.textContent) style.textContent = cleaned;
  });
}

function preserveBodyScripts(persistedHtml: string, doc: Document): void {
  const persisted = new DOMParser().parseFromString(persistedHtml, 'text/html');
  const scripts = Array.from(persisted.body.querySelectorAll('script'));
  if (scripts.length === 0) return;

  const fingerprint = (s: string) => s.slice(0, 200);
  const existingKeys = new Set(
    Array.from(doc.body.querySelectorAll('script'))
      .map(s => fingerprint(s.textContent || ''))
  );

  scripts.forEach(s => {
    const key = fingerprint(s.textContent || '');
    if (!existingKeys.has(key)) {
      doc.body.appendChild(s.cloneNode(true));
      existingKeys.add(key);
    }
  });
}

function injectFontLinks(html: string): string {
  const families = extractFontFamilies(html);
  if (families.length === 0) return html;

  const doc = new DOMParser().parseFromString(html, 'text/html');

  families.forEach(family => {
    const escaped = CSS.escape(family);
    if (doc.head.querySelector(`link[data-aniya-font="${escaped}"]`)) return;
    const familySlug = family.replace(/ /g, '+');
    const existing = doc.head.querySelectorAll('link[rel="stylesheet"]');
    if (Array.from(existing).some(
      l => (l.getAttribute('href') || '').includes('fonts.googleapis.com') &&
           (l.getAttribute('href') || '').includes(familySlug)
    )) return;

    const link = doc.createElement('link');
    link.rel = 'stylesheet';
    link.href = buildGoogleFontsURL(family);
    link.setAttribute('data-aniya-font', family);
    doc.head.insertBefore(link, doc.head.firstChild);
  });

  return '<!DOCTYPE html>\n' + doc.documentElement.outerHTML;
}

// ─── Asset URL resolution ───────────────────────────────────────────────────────

function resolveProjectAssetUrls(html: string, projectId: string): string {
  const root = `/api/v1/projects/${encodeURIComponent(projectId)}/`;
  return html
    .replace(/(\bsrc=(["']))uploads\//g, `$1${root}uploads/`)
    .replace(/(\bhref=(["']))uploads\//g, `$1${root}uploads/`)
    .replace(/url\(\s*(["']?)uploads\//g, (_match, quote: string) => `url(${quote || ''}${root}uploads/`);
}

// ─── Publish serialization ──────────────────────────────────────────────────────

export function serializePublishHtml(
  persistedHtml: string,
  editor: Editor,
  opts?: { skipFontInjection?: boolean },
): string {
  const doc = new DOMParser().parseFromString(persistedHtml, 'text/html');

  // 1. Remove editor attribute — scoped shell no longer matches.
  doc.documentElement.removeAttribute('data-aniya-editor');

  // 2. Strip legacy unscoped shell from ALL <style> blocks.
  stripAllLegacyShell(doc);

  // 3. Delta three-branch logic.
  const cleanDelta = stripShells(editor.getCss());
  const cssDirty = normalizeCss(cleanDelta) !== normalizeCss(cssBaseline);
  const hasExistingDelta = !!doc.querySelector('style[data-aniya-editor-delta]');

  if (cssDirty) {
    // Branch A: CSS changed → replace delta.
    doc.querySelectorAll('style[data-aniya-editor-delta]').forEach(s => s.remove());
    if (cleanDelta) {
      const el = doc.createElement('style');
      el.setAttribute('data-aniya-editor-delta', '');
      el.textContent = cleanDelta;
      doc.head.appendChild(el);
    }
  } else if (!hasExistingDelta && cleanDelta) {
    // Branch B: CSS unchanged + no delta on disk → first materialization.
    const el = doc.createElement('style');
    el.setAttribute('data-aniya-editor-delta', '');
    el.textContent = cleanDelta;
    doc.head.appendChild(el);
  }
  // Branch C: !cssDirty && hasExistingDelta → keep existing delta.

  // 4. Body from editor.
  doc.body.innerHTML = editor.getHtml();

  // 5. Restore body scripts from persistedHtml.
  preserveBodyScripts(persistedHtml, doc);

  // 6. Inject font links (skipped for preview to avoid blocking external CSS).
  let html = '<!DOCTYPE html>\n' + doc.documentElement.outerHTML;
  if (!opts?.skipFontInjection) {
    html = injectFontLinks(html);
  }
  return html;
}

/** Reset baseline so subsequent cssDirty comparisons are relative to last save. */
export function resetCssBaseline(editor: Editor): void {
  cssBaseline = stripShells(editor.getCss());
}

// ─── Import helpers ─────────────────────────────────────────────────────────────

function syncImportedHeadAssets(editor: Editor, parsed: Document) {
  const canvasDoc = editor.Canvas.getDocument();
  if (!canvasDoc) return;

  canvasDoc
    .querySelectorAll('[data-imported-html-asset="true"]')
    .forEach((node) => node.remove());

  parsed
    .querySelectorAll<HTMLLinkElement>(
      'link[rel="stylesheet"], link[rel="preconnect"], link[rel="preload"]',
    )
    .forEach((link) => {
      const clone = canvasDoc.createElement('link');
      Array.from(link.attributes).forEach((attr) => {
        clone.setAttribute(attr.name, attr.value);
      });
      clone.setAttribute('data-imported-html-asset', 'true');
      canvasDoc.head.appendChild(clone);
    });
}

const PAGE_BOUNDARY_TAGS = new Set(['section', 'header', 'footer', 'nav', 'article', 'main']);

function isPageBoundary(el: Element): boolean {
  if (PAGE_BOUNDARY_TAGS.has(el.tagName.toLowerCase())) return true;
  if (el.tagName.toLowerCase() === 'div') {
    const cls = el.className?.toString() || '';
    if (/\b(section|chapter|hero|page|container)\b/i.test(cls)) return true;
    if (el.querySelector('h1, h2, h3')) return true;
  }
  return false;
}

function normalizeToSlides(editor: Editor): void {
  const doc = editor.Canvas.getDocument();
  if (!doc) return;

  const existingSlides = doc.querySelectorAll('.slide');
  if (existingSlides.length >= 1) return;

  const body = doc.body;
  if (!body) return;

  const children = Array.from(body.children).filter((el) => {
    const tag = el.tagName.toLowerCase();
    return tag !== 'script' && tag !== 'style' && !el.classList.contains('noise');
  });

  if (children.length === 0) return;

  // Group children by page boundaries.
  const groups: Element[][] = [];
  let currentGroup: Element[] = [];

  for (const child of children) {
    if (isPageBoundary(child) && currentGroup.length > 0) {
      groups.push(currentGroup);
      currentGroup = [];
    }
    currentGroup.push(child);
  }
  if (currentGroup.length > 0) groups.push(currentGroup);

  // Wrap each group in a slide div.
  const slideHtml = groups
    .map(
      (group, i) =>
        `<div class="slide${i === 0 ? ' active' : ''}" style="position:absolute;top:0;left:0;width:1920px;height:1080px;overflow:hidden;box-sizing:border-box">${group
          .map((el) => el.outerHTML)
          .join('\n')}</div>`,
    )
    .join('\n');

  const wrapper = `<div id="slide-wrapper" style="position:relative;width:1920px;height:1080px;overflow:hidden">${slideHtml}</div>`;

  editor.setComponents(wrapper);
  editor.refresh();
}

function isInsideCustomCode(el: Element): boolean {
  let parent = el.parentElement;
  while (parent) {
    if (parent.getAttribute('data-gjs-type') === 'custom-code') return true;
    parent = parent.parentElement;
  }
  return false;
}

const VIEWPORT_CONTAINER_SELECTORS = ['#presentation', '#stage', '#scale-wrap'];

function fixViewportContainers(editor: Editor): void {
  const doc = editor.Canvas.getDocument();
  if (!doc) return;

  let changed = false;
  for (const sel of VIEWPORT_CONTAINER_SELECTORS) {
    const el = doc.querySelector<HTMLElement>(sel);
    if (!el) continue;
    const view = doc.defaultView;
    if (!view) continue;
    const cs = view.getComputedStyle(el);
    if (
      cs.position === 'absolute' &&
      cs.top === '50%' &&
      cs.left === '50%'
    ) {
      el.style.position = 'relative';
      el.style.top = '';
      el.style.left = '';
      el.style.right = '';
      el.style.bottom = '';
      el.style.margin = '0 auto';
      el.style.transform = '';
      changed = true;
    }
  }

  if (changed) {
    editor.refresh();
  }
}

export function importHtmlDocument(
  editor: Editor,
  html: string,
  t: T,
  projectId?: string,
) {
  let source = html.trim();
  if (!source) {
    throw new Error(t.errors.importHtmlEmpty);
  }
  if (projectId) {
    source = resolveProjectAssetUrls(source, projectId);
  }

  const parsed = new DOMParser().parseFromString(source, 'text/html');

  // ── Extract and merge all CSS into a single CssComposer input ──

  // 1. Styles from <head> (not inside custom-code).
  const headCss = Array.from(parsed.querySelectorAll('style'))
    .filter((s) => !isInsideCustomCode(s))
    .map((style) => style.textContent || '')
    .filter((text) => text.trim())
    .join('\n\n');

  // 2. Normalize: extract <style> from inside custom-code blocks so they
  //    reach CssComposer instead of being trapped in a placeholder view.
  //    Merge order (head → custom-code → shell) is load-bearing:
  //    custom-code styles come after head styles so they can override when
  //    the author intended them as late-binding; shell is always last.
  const customCodeCss = Array.from(parsed.querySelectorAll('style'))
    .filter((s) => isInsideCustomCode(s))
    .map((style) => style.textContent || '')
    .filter((text) => text.trim())
    .join('\n\n');

  const mergedCss = [headCss, customCodeCss, ANIYA_DECK_SHELL_CSS]
    .filter(Boolean)
    .join('\n\n');

  // Remove <style> elements from the parsed DOM — both outside and inside
  // custom-code blocks — since they are now merged into CssComposer.
  parsed.querySelectorAll('style').forEach((style) => style.remove());

  // Remove scripts outside custom-code blocks. Scripts inside custom-code
  // blocks are preserved so the custom-code plugin stores them as raw code.
  parsed.querySelectorAll('script').forEach((script) => {
    if (!isInsideCustomCode(script)) script.remove();
  });

  const bodyHtml = parsed.body?.innerHTML?.trim() || source;

  editor.setStyle(mergedCss);
  editor.setComponents(bodyHtml);
  syncImportedHeadAssets(editor, parsed);
  fixViewportContainers(editor);
  normalizeToSlides(editor);

  // Ensure the first .slide has class="active" so it's visible under
  // ANIYA_DECK_SHELL_CSS (.slide:not(.active) { display: none !important; }).
  const wrapper = editor.getWrapper();
  if (wrapper) {
    const slides = wrapper.find('.slide');
    slides.forEach((s: any) => s.removeClass('active'));
    if (slides.length > 0) {
      slides[0].addClass('active');
    }
  }

  editor.refresh();

  // Inject editor context attribute for scoped shell CSS matching.
  const canvasDoc = editor.Canvas.getDocument();
  if (canvasDoc?.documentElement) {
    canvasDoc.documentElement.setAttribute('data-aniya-editor', '');
  }

  // Store CSS baseline for delta dirty detection.
  cssBaseline = stripShells(editor.getCss());

  // Scan and register fonts used in the imported HTML.
  const af = (editor as any).aniyaFonts;
  if (af?.scanHTMLForFonts) {
    af.scanHTMLForFonts(source);
  }
}
