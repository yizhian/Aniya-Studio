import React from "react";
import { motion, AnimatePresence } from "motion/react";
import { Loader2 } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { Portal } from "../common/Portal";
import { Z_INDEX } from "../../constants/zIndex";

type Props = {
  open: boolean;
  mode: "navigate" | "rollback";
  onSaveAndProceed: () => void;
  onDiscard: () => void;
  onCancel: () => void;
  saving: boolean;
};

export function UnsavedChangesDialog({
  open,
  mode,
  onSaveAndProceed,
  onDiscard,
  onCancel,
  saving,
}: Props) {
  const { t } = useLocale();
  return (
    <Portal>
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
          className="fixed inset-0 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          style={{ zIndex: Z_INDEX.CRITICAL }}
          onClick={onCancel}
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.92, y: 16 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95 }}
            transition={{ duration: 0.2, ease: "easeOut" }}
            onClick={(e) => e.stopPropagation()}
            className="w-[400px] max-w-[90vw] bg-[var(--editor-surface)] border-2 border-[#0a0a0a] rounded-2xl shadow-2xl p-6"
          >
            {/* SYSTEM ALERT label */}
            <div className="mb-3">
              <span
                className="text-[10px] font-bold tracking-[0.15em] uppercase text-[var(--editor-text-muted)]"
                style={{ fontFamily: "'Space Mono', 'JetBrains Mono', monospace" }}
              >
                <span className="inline-block w-1.5 h-1.5 rounded-sm bg-[var(--editor-text)] mr-1.5 align-middle" />
                SYSTEM ALERT
              </span>
            </div>

            {/* Title + description */}
            <div className="flex items-start gap-3 mb-4">
              <div className="flex-shrink-0 w-10 h-10 rounded-lg bg-[var(--editor-text)] flex items-center justify-center">
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="var(--editor-bg)"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <rect x="3" y="3" width="18" height="18" rx="3" />
                  <path d="M12 8v5" />
                  <circle cx="12" cy="16.5" r="1.2" fill="var(--editor-bg)" stroke="none" />
                </svg>
              </div>
              <div>
                <h3
                  className="text-[15px] font-bold text-[var(--editor-text)] leading-snug"
                  style={{ fontFamily: "'Noto Sans SC', sans-serif" }}
                >
                  {t.save.unsavedChanges}
                </h3>
                <p className="text-[13px] text-[var(--editor-text-muted)] mt-1 leading-relaxed">
                  {t.save.unsavedMessage}
                </p>
              </div>
            </div>

            {/* Buttons */}
            <div className="flex items-center justify-end gap-2.5 mt-5">
              <button
                type="button"
                onClick={onCancel}
                disabled={saving}
                className="px-4 py-2 text-[13px] font-medium text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] rounded-lg transition-colors disabled:opacity-40"
              >
                {t.save.cancel}
              </button>
              <button
                type="button"
                onClick={onDiscard}
                disabled={saving}
                className="px-4 py-2 text-[13px] font-medium text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)] rounded-lg border border-[var(--editor-border)] transition-colors disabled:opacity-40"
              >
                {t.save.discard}
              </button>
              <button
                type="button"
                onClick={onSaveAndProceed}
                disabled={saving}
                className="relative px-5 py-2 text-[13px] font-semibold text-[var(--editor-bg)] bg-[var(--editor-text)] hover:opacity-90 rounded-lg transition-all disabled:opacity-50 flex items-center gap-1.5"
              >
                {saving && <Loader2 size={14} className="animate-spin" />}
                {t.save.saveAndContinue}
              </button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
    </Portal>
  );
}
