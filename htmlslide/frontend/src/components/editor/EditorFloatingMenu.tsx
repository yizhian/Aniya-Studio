import React, { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Bookmark, ChevronLeft, Download, FileText, Menu, Play, Presentation, Share2, X } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";

type Props = {
  onPreview: () => void;
  onExport: () => void;
  onExportPdf: () => void;
  onExportPptx: () => void;
  onPrecipitateSkill: () => void;
  onNavigateHome: () => void;
  projectName?: string;
};

export function EditorFloatingMenu({ onPreview, onExport, onExportPdf, onExportPptx, onPrecipitateSkill, onNavigateHome, projectName }: Props) {
  const { t } = useLocale();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  return (
    <div ref={ref} className="absolute left-6 top-6 z-50 flex items-center gap-1">
      {/* Back to home — frameless, just the arrow */}
      <button
        type="button"
        onClick={onNavigateHome}
        className="flex items-center justify-center w-8 h-8 text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors"
        title={t.editor.back}
      >
        <ChevronLeft size={20} strokeWidth={1.5} />
      </button>

      {/* Hamburger menu */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-10 h-10 rounded-lg bg-[var(--editor-surface)] border border-[var(--editor-border)] flex items-center justify-center text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:border-[var(--editor-text-muted)] transition-all shadow-sm"
      >
        {open ? <X size={18} /> : <Menu size={18} />}
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -8, scale: 0.96 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -8, scale: 0.96 }}
            transition={{ duration: 0.18, ease: "easeOut" }}
            className="absolute left-0 top-12 w-56 bg-[var(--editor-surface)] border border-[var(--editor-border)] rounded-xl shadow-lg overflow-hidden"
          >
            {/* Header */}
            <div className="px-4 pt-4 pb-2">
              <div className="flex items-center gap-2 mb-1">
                <div className="w-5 h-5 rounded-md bg-[var(--editor-accent)] flex items-center justify-center">
                  <div className="w-2.5 h-2.5 rounded-sm bg-[var(--editor-accent-text)]" />
                </div>
                <span className="text-[13px] font-medium text-[var(--editor-text)] truncate">
                  {projectName || t.editor.projectName}
                </span>
              </div>
            </div>

            {/* Actions */}
            <div className="py-1">
              <button
                type="button"
                onClick={() => { onPrecipitateSkill(); setOpen(false); }}
                className="w-full flex items-center gap-3 px-4 py-2 text-[13px] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
              >
                <Bookmark size={16} />
                {t.editor.saveAsSkill}
              </button>
              <button
                type="button"
                onClick={() => { onPreview(); setOpen(false); }}
                className="w-full flex items-center gap-3 px-4 py-2 text-[13px] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
              >
                <Play size={16} />
                {t.editor.preview}
              </button>
              <button
                type="button"
                onClick={() => { onExport(); setOpen(false); }}
                className="w-full flex items-center gap-3 px-4 py-2 text-[13px] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
              >
                <Download size={16} />
                {t.editor.export}
              </button>
              <button
                type="button"
                onClick={() => { onExportPdf(); setOpen(false); }}
                className="w-full flex items-center gap-3 px-4 py-2 text-[13px] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
              >
                <FileText size={16} />
                {t.editor.exportPdf}
              </button>
              <button
                type="button"
                onClick={() => { onExportPptx(); setOpen(false); }}
                className="w-full flex items-center gap-3 px-4 py-2 text-[13px] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
              >
                <Presentation size={16} />
                {t.editor.exportPptx}
              </button>
              <button
                type="button"
                className="w-full flex items-center gap-3 px-4 py-2 text-[13px] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
              >
                <Share2 size={16} />
                {t.editor.share}
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
