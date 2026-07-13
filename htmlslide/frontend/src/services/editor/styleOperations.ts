import type { Editor } from 'grapesjs';
import { normalizeColorToHex, colorFromCssPaint, parseNumberish } from '../../utils/color';

// ─── Types ──────────────────────────────────────────────────────────────────────

type StyleConstraintKind = 'color' | 'px' | 'enum';
export type ControlledStyleKey =
  | 'color'
  | 'font-family'
  | 'font-size'
  | 'text-align'
  | 'font-weight'
  | 'font-style'
  | 'line-height'
  | 'letter-spacing'
  | 'background-color';

interface StyleConstraintBase {
  kind: StyleConstraintKind;
  fallback: string;
}

interface EnumConstraint extends StyleConstraintBase {
  kind: 'enum';
  allowed: readonly string[];
}

interface PxConstraint extends StyleConstraintBase {
  kind: 'px';
  min: number;
  max: number;
}

interface ColorConstraint extends StyleConstraintBase {
  kind: 'color';
}

type StyleConstraint = EnumConstraint | PxConstraint | ColorConstraint;
export type StylePatch = Partial<Record<ControlledStyleKey, string | number>>;

const _BUILTIN_FONT_OPTIONS = [
  { label: 'Inter', value: 'Inter, Arial, sans-serif' },
  { label: 'Arial', value: 'Arial, sans-serif' },
  { label: 'Helvetica', value: 'Helvetica, Arial, sans-serif' },
  { label: 'Georgia', value: 'Georgia, serif' },
  { label: 'Times New Roman', value: '"Times New Roman", Times, serif' },
  { label: 'Monospace', value: '"SFMono-Regular", Consolas, "Liberation Mono", monospace' },
] as const;

/** 返回当前可用字体列表，优先来自 aniyaFonts 插件注册的动态字体。 */
export function getFontFamilyOptions(editor?: Editor | null): ReadonlyArray<{ label: string; value: string }> {
  const af = (editor as any)?.aniyaFonts;
  if (af?.getRegisteredFamilies) {
    const families: string[] = af.getRegisteredFamilies();
    if (families.length > 0) {
      return families.map((f: string) => ({
        label: f,
        value: `'${f}', ${/Mono/i.test(f) ? 'monospace' : 'sans-serif'}`,
      }));
    }
  }
  return _BUILTIN_FONT_OPTIONS;
}

// Add any new toolbar style key here first, then route UI actions through updateSelectedStyles.
export const STYLE_CONSTRAINTS: Readonly<Record<ControlledStyleKey, StyleConstraint>> =
  {
    color: { kind: 'color', fallback: '#ffffff' },
    'font-family': {
      kind: 'enum',
      allowed: _BUILTIN_FONT_OPTIONS.map((item) => item.value.toLowerCase()),
      fallback: _BUILTIN_FONT_OPTIONS[0].value,
    },
    'font-size': { kind: 'px', min: 8, max: 200, fallback: '16px' },
    'text-align': {
      kind: 'enum',
      allowed: ['left', 'center', 'right', 'justify'],
      fallback: 'left',
    },
    'font-weight': {
      kind: 'enum',
      allowed: [
        '100',
        '200',
        '300',
        '400',
        '500',
        '600',
        '700',
        '800',
        '900',
        'normal',
        'bold',
      ],
      fallback: '400',
    },
    'font-style': {
      kind: 'enum',
      allowed: ['normal', 'italic', 'oblique'],
      fallback: 'normal',
    },
    'line-height': { kind: 'px', min: 8, max: 400, fallback: '24px' },
    'letter-spacing': { kind: 'px', min: -20, max: 100, fallback: '0px' },
    'background-color': { kind: 'color', fallback: '#000000' },
  } as const;

function isControlledStyleKey(key: string): key is ControlledStyleKey {
  return key in STYLE_CONSTRAINTS;
}

function sanitizeStyleValue(
  key: ControlledStyleKey,
  input: string | number | undefined,
) {
  if (input === undefined || input === null) return undefined;
  const constraint = STYLE_CONSTRAINTS[key];

  if (constraint.kind === 'color') {
    const raw = String(input).trim().toLowerCase();
    if (
      key === 'background-color' &&
      (raw === 'transparent' ||
        raw === 'rgba(0, 0, 0, 0)' ||
        raw === 'rgba(0,0,0,0)')
    ) {
      return normalizeColorToHex(STYLE_CONSTRAINTS['background-color'].fallback);
    }
    return normalizeColorToHex(String(input));
  }

  if (constraint.kind === 'enum') {
    const value = String(input).trim().toLowerCase();
    if (key !== 'font-family' && !constraint.allowed.includes(value)) return constraint.fallback;
    const fontOption = key === 'font-family'
      ? _BUILTIN_FONT_OPTIONS.find((item) => item.value.toLowerCase() === value)
      : null;
    return fontOption?.value ?? value;
  }

  const parsed = parseNumberish(input);
  if (parsed === null) return constraint.fallback;
  const clamped = Math.min(constraint.max, Math.max(constraint.min, parsed));
  return `${Math.round(clamped)}px`;
}

function patchSelectedStyle(editor: Editor, nextPatch: StylePatch) {
  const selected = editor.getSelected();
  if (!selected) return;
  const prevStyle = (selected.getStyle() || {}) as Record<string, unknown>;
  const sanitizedPatch: Record<string, string> = {};

  Object.entries(nextPatch).forEach(([key, value]) => {
    if (!isControlledStyleKey(key)) return;
    const sanitized = sanitizeStyleValue(key, value);
    if (sanitized !== undefined) sanitizedPatch[key] = sanitized;
  });

  if (sanitizedPatch['background-color']) {
    sanitizedPatch['background-image'] = 'none';
  }
  if (sanitizedPatch.color) {
    sanitizedPatch['-webkit-text-fill-color'] = sanitizedPatch.color;
    const selectedEl = selected.getEl();
    if (selectedEl instanceof SVGElement) {
      sanitizedPatch.fill = sanitizedPatch.color;
    }
  }

  const merged = { ...prevStyle, ...sanitizedPatch } as Record<string, string>;
  selected.setStyle(merged);
}

export function updateSelectedStyles(editor: Editor, patch: StylePatch) {
  patchSelectedStyle(editor, patch);
}

/** Direct style setter for Inspector panel — bypasses STYLE_CONSTRAINTS. */
export function setSelectedStyle(editor: Editor, key: string, value: string) {
  const selected = editor.getSelected();
  if (!selected) return;
  const prevStyle = (selected.getStyle() || {}) as Record<string, unknown>;
  const merged = { ...prevStyle, [key]: value } as Record<string, string>;
  selected.setStyle(merged);
}

/** 供工具栏等 UI 读取 px 类样式的合法范围（与 STYLE_CONSTRAINTS 一致）。 */
export function getStylePxBounds(styleKey: ControlledStyleKey): {
  min: number;
  max: number;
} {
  const c = STYLE_CONSTRAINTS[styleKey];
  if (c.kind !== 'px') return { min: 0, max: 9999 };
  return { min: c.min, max: c.max };
}

export function setFontFamily(editor: Editor, family: string) {
  updateSelectedStyles(editor, { 'font-family': family });
}

/** 字重是否在视觉上为「粗体」 */
export function isBoldWeight(weight: string) {
  const w = String(weight).trim().toLowerCase();
  if (w === 'bold' || w === 'bolder') return true;
  const n = parseInt(w, 10);
  return Number.isFinite(n) && n >= 600;
}

export function toggleBold(editor: Editor) {
  const selected = editor.getSelected();
  if (!selected) return;
  const style = (selected.getStyle() || {}) as Record<string, unknown>;
  const raw = style['font-weight'];
  const el = selected.getEl();
  const view = el?.ownerDocument.defaultView;
  const current =
    typeof raw === 'string' && raw.trim()
      ? raw.trim()
      : (el && view
          ? view.getComputedStyle(el).fontWeight
          : '400');
  const next = isBoldWeight(current) ? '400' : '700';
  updateSelectedStyles(editor, { 'font-weight': next });
}

export function toggleItalic(editor: Editor) {
  const selected = editor.getSelected();
  if (!selected) return;
  const style = (selected.getStyle() || {}) as Record<string, unknown>;
  const raw = style['font-style'];
  const el = selected.getEl();
  const view = el?.ownerDocument.defaultView;
  const current =
    typeof raw === 'string' && raw.trim()
      ? raw.trim().toLowerCase()
      : (el && view
          ? view.getComputedStyle(el).fontStyle
          : 'normal');
  const next = current === 'italic' || current === 'oblique' ? 'normal' : 'italic';
  updateSelectedStyles(editor, { 'font-style': next });
}

function normalizeFontWeightForUi(input: string | undefined) {
  if (!input) return STYLE_CONSTRAINTS['font-weight'].fallback;
  const v = String(input).trim().toLowerCase();
  if (v === 'normal') return '400';
  if (v === 'bold' || v === 'bolder') return '700';
  const n = parseInt(v, 10);
  if (Number.isFinite(n)) return String(Math.min(900, Math.max(100, n)));
  return sanitizeStyleValue('font-weight', v) || STYLE_CONSTRAINTS['font-weight'].fallback;
}

export function getSelectedStyles(editor: Editor) {
  const selected = editor.getSelected();
  if (!selected) return null;

  const style = (selected.getStyle() || {}) as Record<string, unknown>;
  const el = selected.getEl();
  const view = el?.ownerDocument.defaultView;
  const computed = el && view ? view.getComputedStyle(el) : null;
  const fontFamilyFromInline = style['font-family'];
  const fontSizeFromInline = style['font-size'];
  const colorFromInline = style.color;
  const textAlignFromInline = style['text-align'];
  const fontWeightFromInline = style['font-weight'];
  const fontStyleFromInline = style['font-style'];
  const bgFromInline = style['background-color'];

  const color = sanitizeStyleValue(
    'color',
    (typeof colorFromInline === 'string' && colorFromInline) ||
      computed?.color ||
      STYLE_CONSTRAINTS.color.fallback,
  );

  const fontFamily = sanitizeStyleValue(
    'font-family',
    (typeof fontFamilyFromInline === 'string' && fontFamilyFromInline) ||
      computed?.fontFamily ||
      STYLE_CONSTRAINTS['font-family'].fallback,
  );

  const fontSize = sanitizeStyleValue(
    'font-size',
    (typeof fontSizeFromInline === 'string' && fontSizeFromInline) ||
      (computed?.fontSize ?? STYLE_CONSTRAINTS['font-size'].fallback),
  );

  const textAlign = sanitizeStyleValue(
    'text-align',
    (typeof textAlignFromInline === 'string' && textAlignFromInline) ||
      computed?.textAlign ||
      STYLE_CONSTRAINTS['text-align'].fallback,
  );

  const fontWeightRaw =
    (typeof fontWeightFromInline === 'string' && fontWeightFromInline) ||
    computed?.fontWeight ||
    STYLE_CONSTRAINTS['font-weight'].fallback;

  const fontWeight =
    sanitizeStyleValue('font-weight', normalizeFontWeightForUi(fontWeightRaw)) ||
    STYLE_CONSTRAINTS['font-weight'].fallback;

  const fontStyle = sanitizeStyleValue(
    'font-style',
    (typeof fontStyleFromInline === 'string' && fontStyleFromInline) ||
      computed?.fontStyle ||
      STYLE_CONSTRAINTS['font-style'].fallback,
  );

  const computedBackgroundColor = computed?.backgroundColor;
  const backgroundColor = sanitizeStyleValue(
    'background-color',
    (typeof bgFromInline === 'string' && bgFromInline) ||
      (computedBackgroundColor &&
      computedBackgroundColor !== 'rgba(0, 0, 0, 0)' &&
      computedBackgroundColor !== 'rgba(0,0,0,0)'
        ? computedBackgroundColor
        : colorFromCssPaint(computed?.backgroundImage)) ||
      'transparent',
  );

  return {
    color: color || STYLE_CONSTRAINTS.color.fallback,
    fontFamily: fontFamily || STYLE_CONSTRAINTS['font-family'].fallback,
    fontSize:
      parseInt(fontSize || STYLE_CONSTRAINTS['font-size'].fallback, 10) || 16,
    fontWeight,
    fontStyle: fontStyle || STYLE_CONSTRAINTS['font-style'].fallback,
    backgroundColor:
      backgroundColor ||
      normalizeColorToHex(
        computed?.backgroundColor || STYLE_CONSTRAINTS['background-color'].fallback,
      ),
    textAlign: textAlign || STYLE_CONSTRAINTS['text-align'].fallback,
  };
}
