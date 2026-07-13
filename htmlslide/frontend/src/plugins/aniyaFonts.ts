import type { Editor } from 'grapesjs';

const FONT_LIST_URL = '/.slidecraft/available_fonts.json';

const SYSTEM_FONTS = new Set([
  'Arial', 'Helvetica', 'sans-serif', 'serif', 'monospace',
  'cursive', 'fantasy', 'system-ui', 'inherit', 'initial',
]);

interface FontEntry {
  family: string;
  category: string;
  weights: string[];
}

type FontRegistry = Map<string, FontEntry>;

export function buildGoogleFontsURL(family: string): string {
  return `https://fonts.googleapis.com/css2?family=${family.replace(/ /g, '+')}&display=swap`;
}

/** 纯函数：从 HTML 字符串提取所有 Google Font 家族名。
 *  扫描三个来源：<style> 块 font-family 声明、inline style 属性、Google Fonts <link> 标签。 */
export function extractFontFamilies(html: string): string[] {
  const families = new Set<string>();
  const cssRe = /font-family\s*:\s*['"]?([^'";}]+)['"]?/g;

  // 1. <style> 块
  const styleRe = /<style[^>]*>([\s\S]*?)<\/style>/gi;
  let sm: RegExpExecArray | null;
  while ((sm = styleRe.exec(html)) !== null) {
    let m: RegExpExecArray | null;
    while ((m = cssRe.exec(sm[1])) !== null) {
      const family = m[1].trim().split(',')[0].trim().replace(/^['"]|['"]$/g, '');
      if (family && !SYSTEM_FONTS.has(family)) families.add(family);
    }
  }

  // 2. inline style 属性（双引号 / 单引号分别匹配，避免值内引号截断）
  const processInline = (val: string) => {
    let m: RegExpExecArray | null;
    while ((m = cssRe.exec(val)) !== null) {
      const family = m[1].trim().split(',')[0].trim().replace(/^['"]|['"]$/g, '');
      if (family && !SYSTEM_FONTS.has(family)) families.add(family);
    }
  };
  let im: RegExpExecArray | null;
  const inlineReDbl = /style\s*=\s*"([^"]*)"/gi;
  while ((im = inlineReDbl.exec(html)) !== null) processInline(im[1]);
  const inlineReSgl = /style\s*=\s*'([^']*)'/gi;
  while ((im = inlineReSgl.exec(html)) !== null) processInline(im[1]);

  // 3. Google Fonts <link> 标签
  const linkRe = /<link[^>]+href=["']https:\/\/fonts\.googleapis\.com\/css2\?family=([^&"']+)/gi;
  let lm: RegExpExecArray | null;
  while ((lm = linkRe.exec(html)) !== null) {
    families.add(lm[1].replace(/\+/g, ' '));
  }

  return [...families];
}

function getFallbackCategory(category: string): string {
  switch (category) {
    case 'serif': return 'serif';
    case 'monospace': return 'monospace';
    default: return 'sans-serif';
  }
}

export default function aniyaFonts(editor: Editor, _opts = {}) {
  const registry: FontRegistry = new Map();

  // ── load a single font into the canvas iframe ──
  function loadFont(family: string): void {
    const doc = editor.Canvas.getDocument();
    if (!doc) return;
    // Avoid duplicates.
    const existing = doc.querySelector(`link[data-aniya-font="${CSS.escape(family)}"]`);
    if (existing) return;
    const link = doc.createElement('link');
    link.href = buildGoogleFontsURL(family);
    link.rel = 'stylesheet';
    link.dataset.aniyaFont = family;
    doc.head.appendChild(link);
  }

  // ── register a font: add to registry, load into canvas, update UI ──
  function registerFont(family: string): void {
    const key = family.toLowerCase();
    if (SYSTEM_FONTS.has(family) || registry.has(key)) return;
    const entry: FontEntry = { family, category: 'sans-serif', weights: ['400', '700'] };
    registry.set(key, entry);
    loadFont(family);
    refreshStyleManager();
  }

  // ── update StyleManager font-family sector options ──
  function refreshStyleManager(): void {
    const sm = editor.StyleManager;
    const sector = sm.getSector('typography');
    if (!sector) return;
    const prop = sector.getProperty('font-family');
    if (!prop) return;

    const options: { id: string; label: string }[] = [];
    registry.forEach((entry) => {
      const fallback = getFallbackCategory(entry.category);
      options.push({
        id: `'${entry.family}', ${fallback}`,
        label: entry.family,
      });
    });
    // Keep Inter at top if registered, otherwise add system fallbacks.
    if (!registry.has('inter')) {
      options.unshift({ id: 'Inter, Arial, sans-serif', label: 'Inter' });
    }
    options.push(
      { id: 'Arial, sans-serif', label: 'Arial' },
      { id: 'Georgia, serif', label: 'Georgia' },
      { id: '"SFMono-Regular", Consolas, monospace', label: 'Monospace' },
    );

    prop.set('options', options);
  }

  // ── scan CSS text for font-family declarations ──
  function scanCSSForFonts(css: string): void {
    const re = /font-family\s*:\s*['"]?([^'";}]+)['"]?/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(css)) !== null) {
      const family = m[1].trim().split(',')[0].trim().replace(/^['"]|['"]$/g, '');
      if (family && !SYSTEM_FONTS.has(family)) {
        registerFont(family);
      }
    }
  }

  // ── scan an HTML string for font-family declarations ──
  function scanHTMLForFonts(html: string): void {
    for (const family of extractFontFamilies(html)) {
      registerFont(family);
    }
  }

  // ── load pre-configured font list on startup ──
  async function loadFontList(): Promise<void> {
    try {
      const res = await fetch(FONT_LIST_URL);
      if (!res.ok) return;
      const data = await res.json();
      for (const font of (data.fonts || []) as FontEntry[]) {
        const key = font.family.toLowerCase();
        if (!registry.has(key)) {
          registry.set(key, font);
          loadFont(font.family);
        }
      }
      refreshStyleManager();
    } catch {
      // Font list fetch is best-effort; editor works without it.
    }
  }

  // ── listen for new components to auto-register fonts ──
  editor.on('component:add', (model) => {
    const style = (model as any).getStyle?.() || {};
    const ff = style['font-family'];
    if (typeof ff === 'string') {
      const family = ff.split(',')[0].trim().replace(/^['"]|['"]$/g, '');
      if (family && !SYSTEM_FONTS.has(family)) {
        registerFont(family);
      }
    }
  });

  // ── register commands ──
  editor.Commands.add('aniya:scan-fonts', {
    run: (_: any, __: any, opts: { html?: string; css?: string }) => {
      if (opts?.html) scanHTMLForFonts(opts.html);
      if (opts?.css) scanCSSForFonts(opts.css);
    },
  });

  editor.Commands.add('aniya:register-font', {
    run: (_: any, __: any, opts: { family: string }) => {
      if (opts?.family) registerFont(opts.family);
    },
  });

  // Expose registry and scanner on editor for external use.
  (editor as any).aniyaFonts = {
    registry,
    registerFont,
    scanHTMLForFonts,
    scanCSSForFonts,
    refreshStyleManager,
    getRegisteredFamilies: () => [...registry.values()].map((e) => e.family),
  };

  loadFontList();
}
