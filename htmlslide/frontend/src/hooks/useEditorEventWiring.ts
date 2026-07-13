import { useEffect, useRef } from "react";
import type { Component, Editor } from "grapesjs";
import * as api from "../services/editorApi";
import { useLocale } from "../context/LocaleContext";
import { EditorMode, type EditorModeValue, type SelectedComponentContext } from "../models/editor";
import { clearEditorSelection, readDeckState, type DeckState } from "../services/deckAdapter";

function getComponentPath(el: HTMLElement | null) {
  if (!el) return "unknown";
  const parts: string[] = [];
  let node: HTMLElement | null = el;
  while (node && node.tagName.toLowerCase() !== "body") {
    const tag = node.tagName.toLowerCase();
    const parent = node.parentElement;
    if (!parent) {
      parts.unshift(tag);
      break;
    }
    const siblings = Array.from(parent.children).filter(
      (item) => item.tagName === node?.tagName,
    );
    const index = siblings.indexOf(node) + 1;
    parts.unshift(`${tag}:nth-of-type(${Math.max(index, 1)})`);
    node = parent;
  }
  return parts.join(" > ");
}

function getComponentLabel(el: HTMLElement | null, fallback = "") {
  if (!el) return fallback;
  const tag = el.tagName.toLowerCase();
  const id = el.id ? `#${el.id}` : "";
  const className = Array.from(el.classList)
    .slice(0, 2)
    .map((item) => `.${item}`)
    .join("");
  const text = (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 24);
  return `${tag}${id}${className}${text ? ` · ${text}` : ""}`;
}

function setComponentDirectEditEnabled(component: Component | null, enabled: boolean) {
  if (!component) return;
  component.set({
    editable: enabled,
    draggable: enabled,
    droppable: enabled,
    resizable: enabled,
    copyable: enabled,
    removable: enabled,
  });
}

interface UseEditorEventWiringOptions {
  editor: Editor | null;
  mode: EditorModeValue;
  markDirty: () => void;
  checkAndUpdateDirty: (editor: Editor | null) => void;
  setDeckState: (ds: DeckState | null) => void;
  setSelectedContext: (ctx: SelectedComponentContext | null) => void;
}

/** Wires up GrapesJS event listeners for deck sync, drag-drop, dirty tracking, selection. */
export function useEditorEventWiring({
  editor,
  mode,
  markDirty,
  checkAndUpdateDirty,
  setDeckState,
  setSelectedContext,
}: UseEditorEventWiringOptions) {
  const { t } = useLocale();
  const modeRef = useRef(mode);
  modeRef.current = mode;

  // Deck sync
  useEffect(() => {
    if (!editor) return;
    let syncTimer: number | null = null;
    const syncDeck = () => {
      if (syncTimer !== null) window.clearTimeout(syncTimer);
      syncTimer = window.setTimeout(() => {
        setDeckState(readDeckState(editor));
        syncTimer = null;
      }, 0);
    };
    editor.on("load", syncDeck);
    editor.on("component:add", syncDeck);
    editor.on("component:remove", syncDeck);
    syncDeck();
    return () => {
      if (syncTimer !== null) window.clearTimeout(syncTimer);
      editor.off("load", syncDeck);
      editor.off("component:add", syncDeck);
      editor.off("component:remove", syncDeck);
    };
  }, [editor, setDeckState]);

  // Block drag placement
  useEffect(() => {
    if (!editor) return;
    const onBlockDragStop = (component: Component | undefined) => {
      api.finalizeBlockDrop(editor, component);
    };
    const onCanvasDrop = (_component: Component | undefined, _block: unknown, dropped: Component | undefined) => {
      api.finalizeBlockDrop(editor, dropped);
    };
    editor.on("block:drag:stop", onBlockDragStop);
    editor.on("canvas:drop", onCanvasDrop);
    return () => {
      editor.off("block:drag:stop", onBlockDragStop);
      editor.off("canvas:drop", onCanvasDrop);
    };
  }, [editor]);

  // Dirty tracking
  useEffect(() => {
    if (!editor) return;
    const onEdit = () => {
      if (modeRef.current === "direct") markDirty();
    };
    editor.on("component:add", onEdit);
    editor.on("component:remove", onEdit);
    editor.on("component:update", onEdit);
    editor.on("component:styleUpdate", onEdit);

    let debounceTimer: ReturnType<typeof setTimeout>;
    const onUndoRedo = () => {
      if (modeRef.current === "direct") markDirty();
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => checkAndUpdateDirty(editor), 100);
    };
    editor.on("undo", onUndoRedo);
    editor.on("redo", onUndoRedo);

    return () => {
      editor.off("component:add", onEdit);
      editor.off("component:remove", onEdit);
      editor.off("component:update", onEdit);
      editor.off("component:styleUpdate", onEdit);
      editor.off("undo", onUndoRedo);
      editor.off("redo", onUndoRedo);
      clearTimeout(debounceTimer);
    };
  }, [editor, markDirty, checkAndUpdateDirty]);

  // Design mode: disable RTE
  useEffect(() => {
    if (!editor) return;
    clearEditorSelection(editor);
    setSelectedContext(null);
    const enabled = mode === EditorMode.Design;
    const stopRte = () => {
      if (!enabled) return;
      const rte = editor.RichTextEditor as unknown as { disable?: () => void };
      rte.disable?.();
    };
    editor.on("rte:enable", stopRte);
    return () => {
      editor.off("rte:enable", stopRte);
    };
  }, [editor, mode, setSelectedContext]);

  // Blank area click → deselect
  useEffect(() => {
    if (!editor) return;
    const bindBlankDeselect = () => {
      const doc = editor.Canvas.getDocument();
      if (!doc) return undefined;
      const onPointerDown = (event: PointerEvent) => {
        const target = event.target as HTMLElement | null;
        if (!target) return;
        const isBlank =
          target === doc.body ||
          target === doc.documentElement ||
          target.id === "wrapper" ||
          target.id === "stage" ||
          target.classList.contains("slide");
        if (isBlank) {
          clearEditorSelection(editor);
          setSelectedContext(null);
        }
      };
      doc.addEventListener("pointerdown", onPointerDown, true);
      return () => doc.removeEventListener("pointerdown", onPointerDown, true);
    };

    let cleanup = bindBlankDeselect();
    const rebind = () => {
      cleanup?.();
      cleanup = bindBlankDeselect();
    };
    editor.on("load", rebind);
    return () => {
      cleanup?.();
      editor.off("load", rebind);
    };
  }, [editor, setSelectedContext]);

  // Component selection → sync selectedContext
  useEffect(() => {
    if (!editor) return;
    const syncSelected = () => {
      const selected = editor.getSelected();
      const el = selected?.getEl();
      if (!selected || !el) {
        setSelectedContext(null);
        return;
      }
      setComponentDirectEditEnabled(selected, mode === EditorMode.Direct);
      if (mode === EditorMode.Design) {
        const rte = editor.RichTextEditor as unknown as { disable?: () => void };
        rte.disable?.();
      }
      setSelectedContext({
        id: selected.getId?.() || selected.cid || "unknown",
        tagName: el.tagName.toLowerCase(),
        label: getComponentLabel(el, t.errors.deselectDom),
        html: selected.toHTML?.() || el.outerHTML,
        styles: selected.getStyle() || {},
        path: getComponentPath(el),
      });
    };

    editor.on("component:selected", syncSelected);
    editor.on("component:deselected", syncSelected);
    editor.on("component:update", syncSelected);
    syncSelected();
    return () => {
      editor.off("component:selected", syncSelected);
      editor.off("component:deselected", syncSelected);
      editor.off("component:update", syncSelected);
    };
  }, [editor, mode, setSelectedContext, t.errors.deselectDom]);
}

export { getComponentPath, getComponentLabel, setComponentDirectEditEnabled };
