import React, { useEffect, useRef, useState, ReactNode } from 'react';
import grapesjs from 'grapesjs';
import type { Editor } from 'grapesjs';
import postcssParser from 'grapesjs-parser-postcss';
import customCodePlugin from 'grapesjs-custom-code';
import aniyaFonts from '../plugins/aniyaFonts';
import 'grapesjs/dist/css/grapes.min.css';
import { getImageBlockHtml, getVideoBlockHtml } from '../services/editorApi';
import { ThemeMode, type ThemeModeValue } from '../models/editor';
import { Locale, translations, type LocaleValue } from '../i18n/translations';

export type EditorRuntimeValue = {
  editor: Editor | null;
};

export const EditorRuntimeContext = React.createContext<EditorRuntimeValue>({
  editor: null,
});

/** @deprecated 请优先用 useEditorRuntime；保留兼容仅返回 editor */
export const EditorContext = React.createContext<Editor | null>(null);

function applyCanvasBaseStyles(instance: Editor, themeMode: ThemeModeValue) {
  const doc = instance.Canvas.getDocument();
  if (!doc) return;
  const prev = doc.getElementById('gjs-base-theme');
  if (prev) prev.remove();
  const style = doc.createElement('style');
  style.id = 'gjs-base-theme';
  const isLight = themeMode === ThemeMode.Light;
  style.textContent = `
    html, body {
      margin: 0;
      padding: 0;
      width: 100%;
      height: 100%;
      overflow: hidden !important;
    }
    html {
      background-color: ${isLight ? '#ffffff' : '#000000'};
      color: ${isLight ? '#000000' : '#ffffff'};
      font-family: Inter, Arial, sans-serif;
    }
  `;
  doc.head.appendChild(style);
  doc.documentElement.setAttribute('data-aniya-editor', '');
}

function makeBlockIcon(svgInner: string) {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40" fill="none">
    <rect width="40" height="40" rx="8" fill="rgba(255,255,255,0.08)" />
    ${svgInner}
  </svg>`;
}

function registerEditorBlocks(instance: Editor, t: typeof translations["zh-CN"]) {
  const bm = instance.BlockManager;

  bm.add('div-container', {
    label: t.elementLabels.container,
    category: t.elementLabels.layout,
    media: makeBlockIcon('<rect x="8" y="10" width="24" height="20" rx="2" stroke="rgba(255,255,255,0.6)" stroke-width="1.5" stroke-dasharray="3 2" fill="none"/>'),
    content: `<div style="padding:20px;min-height:100px;border:1px dashed rgba(255,255,255,0.2);border-radius:8px;display:flex;align-items:center;justify-content:center">${t.elementLabels.container}</div>`,
  });

  bm.add('column1', {
    label: t.elementLabels.column1,
    category: t.elementLabels.layout,
    media: makeBlockIcon('<rect x="8" y="8" width="24" height="24" rx="2" stroke="rgba(255,255,255,0.6)" stroke-width="1.5" fill="none"/>'),
    content: `<div style="padding:12px;min-height:80px;border-radius:4px">${t.elementLabels.contentArea}</div>`,
  });

  bm.add('column2', {
    label: t.elementLabels.column2,
    category: t.elementLabels.layout,
    media: makeBlockIcon('<rect x="6" y="8" width="12" height="24" rx="1.5" stroke="rgba(255,255,255,0.6)" stroke-width="1.2" fill="none"/><rect x="22" y="8" width="12" height="24" rx="1.5" stroke="rgba(255,255,255,0.6)" stroke-width="1.2" fill="none"/>'),
    content: `<div style="display:flex;gap:16px"><div style="flex:1;padding:12px;min-height:80px;border:1px dashed rgba(255,255,255,0.15);border-radius:4px">${t.elementLabels.column1}</div><div style="flex:1;padding:12px;min-height:80px;border:1px dashed rgba(255,255,255,0.15);border-radius:4px">${t.elementLabels.column2}</div></div>`,
  });

  bm.add('column3', {
    label: t.elementLabels.column3,
    category: t.elementLabels.layout,
    media: makeBlockIcon('<rect x="5" y="8" width="8" height="24" rx="1" stroke="rgba(255,255,255,0.6)" stroke-width="1.2" fill="none"/><rect x="16" y="8" width="8" height="24" rx="1" stroke="rgba(255,255,255,0.6)" stroke-width="1.2" fill="none"/><rect x="27" y="8" width="8" height="24" rx="1" stroke="rgba(255,255,255,0.6)" stroke-width="1.2" fill="none"/>'),
    content: `<div style="display:flex;gap:12px"><div style="flex:1;padding:12px;min-height:80px;border:1px dashed rgba(255,255,255,0.15);border-radius:4px">${t.elementLabels.column1}</div><div style="flex:1;padding:12px;min-height:80px;border:1px dashed rgba(255,255,255,0.15);border-radius:4px">${t.elementLabels.column2}</div><div style="flex:1;padding:12px;min-height:80px;border:1px dashed rgba(255,255,255,0.15);border-radius:4px">${t.elementLabels.column3}</div></div>`,
  });

  bm.add('heading1', {
    label: t.elementLabels.heading1,
    category: t.elementLabels.text,
    media: makeBlockIcon('<text x="20" y="26" text-anchor="middle" font-size="18" font-weight="700" fill="rgba(255,255,255,0.7)" font-family="Inter,Arial,sans-serif">H1</text>'),
    content: { type: 'text', tagName: 'h1', content: t.elementLabels.heading1, style: { 'font-size': '48px', 'font-weight': '700', color: '#ffffff' } },
  });

  bm.add('heading2', {
    label: t.elementLabels.heading2,
    category: t.elementLabels.text,
    media: makeBlockIcon('<text x="20" y="26" text-anchor="middle" font-size="15" font-weight="600" fill="rgba(255,255,255,0.7)" font-family="Inter,Arial,sans-serif">H2</text>'),
    content: { type: 'text', tagName: 'h2', content: t.elementLabels.heading2, style: { 'font-size': '32px', 'font-weight': '600', color: '#ffffff' } },
  });

  bm.add('paragraph', {
    label: t.elementLabels.paragraph,
    category: t.elementLabels.text,
    media: makeBlockIcon('<line x1="10" y1="14" x2="30" y2="14" stroke="rgba(255,255,255,0.6)" stroke-width="1.5" stroke-linecap="round"/><line x1="10" y1="20" x2="28" y2="20" stroke="rgba(255,255,255,0.4)" stroke-width="1.5" stroke-linecap="round"/><line x1="10" y1="26" x2="24" y2="26" stroke="rgba(255,255,255,0.3)" stroke-width="1.5" stroke-linecap="round"/>'),
    content: { type: 'text', tagName: 'p', content: t.elementLabels.editableText, style: { 'font-size': '16px', color: '#cccccc', 'line-height': '1.6' } },
  });

  bm.add('quote', {
    label: t.elementLabels.quote,
    category: t.elementLabels.text,
    media: makeBlockIcon('<text x="20" y="26" text-anchor="middle" font-size="14" font-style="italic" fill="rgba(255,255,255,0.7)" font-family="Georgia,serif">"</text>'),
    content: `<blockquote style="border-left:3px solid rgba(255,255,255,0.3);padding-left:16px;margin:0;font-style:italic;color:#aaaaaa">${t.elementLabels.quoteText}</blockquote>`,
  });

  bm.add('image', {
    label: t.elementLabels.image,
    category: t.elementLabels.media,
    media: makeBlockIcon('<rect x="8" y="8" width="24" height="18" rx="2" stroke="rgba(255,255,255,0.6)" stroke-width="1.5" fill="none"/><circle cx="14" cy="14" r="3" stroke="rgba(255,255,255,0.5)" stroke-width="1.2" fill="none"/><path d="M8 22l6-6 4 4 4-4 6 6" stroke="rgba(255,255,255,0.4)" stroke-width="1" fill="none"/>'),
    content: getImageBlockHtml(t),
  });

  bm.add('video', {
    label: t.elementLabels.video,
    category: t.elementLabels.media,
    media: makeBlockIcon('<polygon points="16,12 16,28 28,20" fill="rgba(255,255,255,0.6)"/>'),
    content: getVideoBlockHtml(),
  });

  bm.add('button', {
    label: t.elementLabels.button,
    category: t.elementLabels.components,
    media: makeBlockIcon('<rect x="8" y="14" width="24" height="12" rx="6" stroke="rgba(255,255,255,0.6)" stroke-width="1.5" fill="rgba(255,255,255,0.15)"/>'),
    content: `<button style="background-color:#ddb7ff;color:#490080;padding:12px 24px;border-radius:8px;font-weight:600;border:none;cursor:pointer">${t.elementLabels.tryNow}</button>`,
  });

  bm.add('chart', {
    label: t.elementLabels.chart,
    category: t.elementLabels.components,
    media: makeBlockIcon('<rect x="6" y="20" width="6" height="8" rx="1" fill="rgba(255,255,255,0.6)"/><rect x="14" y="14" width="6" height="14" rx="1" fill="rgba(255,255,255,0.5)"/><rect x="22" y="8" width="6" height="20" rx="1" fill="rgba(255,255,255,0.4)"/>'),
    content: `<div style="display:flex;gap:8px;align-items:flex-end;padding:16px;min-height:120px;border:1px dashed rgba(255,255,255,0.15);border-radius:8px"><div style="flex:1;background:rgba(0,212,255,0.3);border-radius:4px 4px 0 0;height:60px"></div><div style="flex:1;background:rgba(0,212,255,0.5);border-radius:4px 4px 0 0;height:90px"></div><div style="flex:1;background:rgba(0,212,255,0.7);border-radius:4px 4px 0 0;height:120px"></div></div>`,
  });

  bm.add('divider', {
    label: t.elementLabels.divider,
    category: t.elementLabels.components,
    media: makeBlockIcon('<line x1="8" y1="20" x2="32" y2="20" stroke="rgba(255,255,255,0.6)" stroke-width="2" stroke-linecap="round"/>'),
    content: '<hr style="border:none;height:1px;background-color:#333333;margin:24px 0" />',
  });
}

function registerEditorComponents(instance: Editor, t: typeof translations["zh-CN"]) {
  instance.DomComponents.addType('image', {
    extend: 'image',
    model: {
      defaults: {
        traits: [
          { type: 'text', name: 'src', label: t.elementLabels.imageUrl },
          { type: 'text', name: 'alt', label: t.elementLabels.altText },
          { type: 'text', name: 'title', label: t.elementLabels.titleAttr },
        ],
      },
    },
  });
}

export function EditorProvider({
  children,
  themeMode = ThemeMode.Dark,
  locale = Locale.ZhCN,
}: {
  children: ReactNode;
  themeMode?: ThemeModeValue;
  locale?: LocaleValue;
}) {
  const [editor, setEditor] = useState<Editor | null>(null);
  const initialized = useRef(false);
  const localeRef = useRef(locale);
  localeRef.current = locale;

  useEffect(() => {
    if (initialized.current) return;
    initialized.current = true;

    const parkingRoot = document.createElement('div');
    parkingRoot.setAttribute('data-gjs-manager-parking', 'true');
    parkingRoot.style.display = 'none';
    document.body.appendChild(parkingRoot);

    const mkSlot = (id: string) => {
      const el = document.createElement('div');
      el.dataset.slot = id;
      parkingRoot.appendChild(el);
      return el;
    };

    const t = translations[localeRef.current];

    const instance = grapesjs.init({
      container: '#gjs-canvas',
      height: '100%',
      width: '100%',
      customUI: true,
      plugins: [postcssParser, customCodePlugin, aniyaFonts],
      pluginsOpts: {
        [customCodePlugin]: {
          modalTitle: t.editor.advancedDesignStyle,
          buttonLabel: t.save.save,
          placeholderScript: '<div style="padding:24px;text-align:center;color:#888;font-family:Inter,Arial,sans-serif;font-size:14px">Custom code with &lt;script&gt; does not render in edit mode.<br/>Scripts will execute normally in preview mode.</div>',
        },
      },
      storageManager: false,
      avoidInlineStyle: false,
      panels: { defaults: [] },
      blockManager: { appendTo: mkSlot('blocks') },
      styleManager: { appendTo: mkSlot('style') },
      selectorManager: { appendTo: mkSlot('selector') },
      traitManager: { appendTo: mkSlot('trait') },
      layerManager: { appendTo: mkSlot('layer') },
      richTextEditor: { custom: true },
      canvas: {
        styles: [
          'https://fonts.googleapis.com/css2?family=Inter:opsz,wght@14..32,100..900&display=swap',
        ],
        scripts: [],
        frameStyle: 'overflow: hidden;',
        allowUnsafeAttr: true,
        allowExternalDrop: true,
      },
      deviceManager: { devices: [] },
    });

    registerEditorComponents(instance, t);
    registerEditorBlocks(instance, t);
    const applyBaseStyles = () => applyCanvasBaseStyles(instance, themeMode);
    applyBaseStyles();
    instance.on('load', applyBaseStyles);

    setEditor(instance);

    return () => {
      instance.destroy();
      document.body.removeChild(parkingRoot);
    };
  }, []);

  // Re-register blocks when locale changes
  useEffect(() => {
    if (!editor) return;
    const t = translations[locale];
    editor.BlockManager.getAll().reset();
    editor.DomComponents.getTypes().forEach((type) => {
      if (type.id === 'image') {
        editor.DomComponents.removeType('image');
      }
    });
    registerEditorComponents(editor, t);
    registerEditorBlocks(editor, t);
  }, [editor, locale]);

  useEffect(() => {
    if (!editor) return;
    applyCanvasBaseStyles(editor, themeMode);
  }, [editor, themeMode]);

  const runtime: EditorRuntimeValue = { editor };

  return (
    <EditorRuntimeContext.Provider value={runtime}>
      <EditorContext.Provider value={editor}>{children}</EditorContext.Provider>
    </EditorRuntimeContext.Provider>
  );
}
