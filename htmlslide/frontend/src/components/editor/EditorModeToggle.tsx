import React from "react";
import { EditorMode, type EditorModeValue } from "../../models/editor";
import { useLocale } from "../../context/LocaleContext";

interface Props {
  mode: EditorModeValue;
  onSetMode: (mode: EditorModeValue) => void;
}

export function EditorModeToggle({ mode, onSetMode }: Props) {
  const { t } = useLocale();

  return (
    <div className="absolute left-1/2 -translate-x-1/2 top-6 z-50">
      <div className="flex items-center gap-0.5 p-0.5 rounded-xl bg-[var(--editor-surface)] border border-[var(--editor-border)] shadow-sm">
        <button
          type="button"
          onClick={() => {
            if (mode === "direct") onSetMode(EditorMode.Design);
          }}
          className={`flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-medium rounded-lg transition-all ${
            mode === "design"
              ? "bg-[var(--editor-control-hover)] text-[var(--editor-text)] shadow-sm"
              : "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)]"
          }`}
        >
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round">
            <path d="M12 2L3 7v10l9 5 9-5V7L12 2z" />
            <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="1.5" />
          </svg>
          {t.designMode.aiDesign}
        </button>
        <button
          type="button"
          onClick={() => onSetMode(EditorMode.Direct)}
          className={`flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-medium rounded-lg transition-all ${
            mode === "direct"
              ? "bg-[var(--editor-control-hover)] text-[var(--editor-text)] shadow-sm"
              : "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)]"
          }`}
        >
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M17 3a2.83 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
            <path d="m15 5 4 4" />
          </svg>
          {t.designMode.edit}
        </button>
      </div>
    </div>
  );
}
