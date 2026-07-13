import React from "react";
import { motion } from "motion/react";
import { History, Loader2, Save } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { AniyaLogo } from "../AniyaLogo";

interface Props {
  isTerminalOpen: boolean;
  isVersionOpen: boolean;
  saveState: "idle" | "saving" | "saved" | "error";
  isDirty: boolean;
  hasProject: boolean;
  hasHtml: boolean;
  effectiveLogoState: "thinking" | "done" | "idle";
  onToggleTerminal: () => void;
  onToggleVersion: () => void;
  onSave: () => void;
}

export function EditorSideActions({
  isTerminalOpen,
  isVersionOpen,
  saveState,
  isDirty,
  hasProject,
  hasHtml,
  effectiveLogoState,
  onToggleTerminal,
  onToggleVersion,
  onSave,
}: Props) {
  const { t } = useLocale();

  return (
    <>
      {/* Aniya Logo button (top) */}
      <div className="absolute left-6 top-1/2 -translate-y-1/2 flex flex-col gap-4 z-40">
        <button
          onClick={onToggleTerminal}
          className={`relative group transition-all hover:scale-110 active:scale-95 ${isTerminalOpen ? "scale-110" : ""}`}
        >
          <AniyaLogo
            size={36}
            state={effectiveLogoState}
            className={
              isTerminalOpen
                ? "text-[var(--editor-text)]"
                : "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors"
            }
          />
        </button>
      </div>

      {/* Save + History buttons (bottom) */}
      <div className="absolute left-6 bottom-6 z-40 flex flex-col gap-2">
        <motion.button
          type="button"
          onClick={onSave}
          disabled={!isDirty || !hasProject || !hasHtml || saveState === "saving"}
          whileTap={!isDirty ? undefined : { scale: 0.92 }}
          className={`relative p-3 rounded-xl border shadow-sm flex items-center gap-2 transition-all duration-200 ${
            isDirty
              ? "bg-[var(--editor-surface)] border-orange-400/50 text-orange-400 hover:border-orange-400 hover:bg-[var(--editor-control-hover)] hover:shadow-md cursor-pointer"
              : "bg-[var(--editor-surface)] border-[var(--editor-border)] text-green-400/80 cursor-default"
          } ${(!isDirty || !hasProject || !hasHtml) ? "opacity-50" : ""}`}
          title={isDirty ? t.save.unsavedChanges : t.save.saved}
        >
          {saveState === "saving" ? (
            <Loader2 size={18} className="animate-spin" />
          ) : (
            <Save size={18} strokeWidth={2} />
          )}
          {saveState === "saved" && (
            <span className="absolute inset-0 rounded-xl ring-2 ring-green-400/30 animate-ping pointer-events-none" />
          )}
        </motion.button>

        <button
          onClick={onToggleVersion}
          className={`p-3 bg-[var(--editor-surface)] border border-[var(--editor-border)] rounded-xl text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-all shadow-sm flex items-center gap-2 ${isVersionOpen ? "ring-2 ring-[var(--editor-border)]" : ""}`}
        >
          <History size={18} strokeWidth={2} />
        </button>
      </div>
    </>
  );
}
