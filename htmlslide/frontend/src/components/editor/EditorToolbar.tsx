import React, { useEffect, useState } from "react";
import { Image as ImageIcon, Redo2, SlidersHorizontal, Undo2 } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useLocale } from "../../context/LocaleContext";
import { useEditor } from "../../hooks/useEditor";
import * as api from "../../services/editorApi";
import { detectComponentType, type ComponentType } from "../../services/editor/componentDetection";
import { EditorToolbarStyleControls } from "./toolbar/EditorToolbarStyleControls";
import type { EditorModeValue } from "../../models/editor";

type Props = {
  mode: EditorModeValue;
  isAdvancedDesignOpen?: boolean;
  onToggleAdvancedDesign?: () => void;
  onOpenImageReplace?: () => void;
};

export function EditorToolbar({
  mode,
  isAdvancedDesignOpen,
  onToggleAdvancedDesign,
  onOpenImageReplace,
}: Props) {
  const editor = useEditor();
  const { t } = useLocale();
  const [styles, setStyles] = useState<ReturnType<
    typeof api.getSelectedStyles
  > | null>(null);
  const [undoable, setUndoable] = useState(false);
  const [redoable, setRedoable] = useState(false);
  const [componentType, setComponentType] = useState<ComponentType>(null);

  const syncToolbarState = React.useCallback(() => {
    if (!editor) return;
    setStyles(api.getSelectedStyles(editor));
    setUndoable(api.canUndo(editor));
    setRedoable(api.canRedo(editor));
    setComponentType(detectComponentType(editor));
  }, [editor]);

  useEffect(() => {
    if (!editor) return;

    editor.on("component:selected", syncToolbarState);
    editor.on("component:deselected", syncToolbarState);
    editor.on("component:update", syncToolbarState);
    editor.on("component:styleUpdate", syncToolbarState);
    editor.on("undo", syncToolbarState);
    editor.on("redo", syncToolbarState);
    syncToolbarState();

    return () => {
      editor.off("component:selected", syncToolbarState);
      editor.off("component:deselected", syncToolbarState);
      editor.off("component:update", syncToolbarState);
      editor.off("component:styleUpdate", syncToolbarState);
      editor.off("undo", syncToolbarState);
      editor.off("redo", syncToolbarState);
    };
  }, [editor, syncToolbarState]);

  const hasSelection = componentType !== null;
  const visible = mode === "direct" && editor !== null && hasSelection;
  const canOpenAdvanced = Boolean(editor);

  const isImage = componentType === "image";
  const isText = componentType === "text";

  return (
    <AnimatePresence>
      {visible && (
        <motion.div
          initial={{ opacity: 0, y: -8, scale: 0.96 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: -4, scale: 0.97 }}
          transition={{ duration: 0.16, ease: "easeOut" }}
          className="absolute top-6 left-1/2 -translate-x-1/2 z-50 pointer-events-none"
        >
          <div className="bg-[var(--editor-surface)] rounded-lg px-4 py-2 border border-[var(--editor-border)] shadow-sm flex items-center gap-4 pointer-events-auto">
            {/* Undo / Redo — always visible */}
            <div className="flex items-center gap-1 border-r border-[var(--editor-border)] pr-3">
              <button
                type="button"
                disabled={!undoable}
                onClick={() => { if (editor) { api.undo(editor); syncToolbarState(); } }}
                className={`p-1.5 rounded-lg transition-colors ${
                  undoable
                    ? "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)]"
                    : "opacity-20"
                }`}
                title={t.toolbar.undo}
              >
                <Undo2 size={17} />
              </button>
              <button
                type="button"
                disabled={!redoable}
                onClick={() => { if (editor) { api.redo(editor); syncToolbarState(); } }}
                className={`p-1.5 rounded-lg transition-colors ${
                  redoable
                    ? "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)]"
                    : "opacity-20"
                }`}
                title={t.toolbar.redo}
              >
                <Redo2 size={17} />
              </button>
            </div>

            {/* Text controls — only for text selection */}
            {isText && (
              <div className="flex items-center gap-3">
                <EditorToolbarStyleControls
                  editor={editor}
                  styles={styles}
                  onAfterAction={syncToolbarState}
                />
              </div>
            )}

            {/* Image controls — only for image selection */}
            {isImage && onOpenImageReplace && (
              <div className="flex items-center gap-2">
                <div className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-[var(--editor-control)] border border-[var(--editor-border)]">
                  <ImageIcon size={14} className="text-[var(--editor-accent)] shrink-0" />
                  <span className="text-[11px] text-[var(--editor-text-muted)] font-mono">{t.toolbar.image}</span>
                </div>
                <button
                  type="button"
                  onClick={onOpenImageReplace}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-[var(--editor-accent)] bg-[var(--editor-accent-soft)] hover:bg-[var(--editor-control-hover)] border border-[var(--editor-border)] transition-colors"
                >
                  {t.toolbar.replaceImage}
                </button>
              </div>
            )}

            {/* Advanced Design — always visible */}
            {onToggleAdvancedDesign && (
              <div className="border-l border-[var(--editor-border)] pl-3 shrink-0">
                <button
                  type="button"
                  disabled={!canOpenAdvanced}
                  title={t.editor.advancedDesign}
                  aria-label={t.editor.advancedDesign}
                  onClick={() => onToggleAdvancedDesign()}
                  className={`p-2 rounded-xl transition-colors ${
                    !canOpenAdvanced
                      ? "opacity-30 text-[var(--editor-text-muted)] cursor-not-allowed"
                      : isAdvancedDesignOpen
                        ? "text-[var(--editor-text)] bg-[var(--editor-control-hover)] ring-1 ring-[var(--editor-border)]"
                        : "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] border border-[var(--editor-border)]"
                  }`}
                >
                  <SlidersHorizontal size={20} strokeWidth={2} />
                </button>
              </div>
            )}
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
