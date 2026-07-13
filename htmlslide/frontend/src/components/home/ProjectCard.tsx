import { FileCode2, Loader2, Sparkles, Trash2 } from "lucide-react";
import { motion, AnimatePresence } from "motion/react";
import type { ProjectResponse } from "../../models/chat";

function formatDate(iso: string, locale: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleString(locale, {
      month: "numeric",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso.slice(0, 10);
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}

interface Props {
  project: ProjectResponse;
  index: number;
  onClick: () => void;
  onDelete: () => void;
  isConfirming: boolean;
  onRequestConfirm: () => void;
  onCancelConfirm: () => void;
  isDeleting: boolean;
  deleteError: string | null;
  locale: string;
  deleteLabel: string;
  confirmLabel: string;
  cancelLabel: string;
}

export function ProjectCard({
  project: p,
  index: i,
  onClick,
  onDelete,
  isConfirming,
  onRequestConfirm,
  onCancelConfirm,
  isDeleting,
  deleteError,
  locale,
  deleteLabel,
  confirmLabel,
  cancelLabel,
}: Props) {
  return (
    <motion.div
      key={p.id}
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{
        duration: 0.35,
        ease: [0.16, 1, 0.3, 1],
        delay: i * 0.04,
      }}
      className="mb-2"
    >
      <div className="group flex items-center gap-3 px-3 py-2.5 rounded-xl border border-[var(--editor-border)] bg-[var(--editor-control)]/30 hover:bg-[var(--editor-control-hover)] transition-colors">
        {/* Thumbnail */}
        <div
          role="button"
          tabIndex={0}
          onClick={onClick}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              onClick();
            }
          }}
          className="shrink-0 w-[100px] h-[67px] rounded-lg overflow-hidden border border-[var(--editor-border)] bg-[var(--editor-canvas-bg)] cursor-pointer relative"
        >
          {p.has_html ? (
            <iframe
              src={`/api/v1/projects/${p.id}/preview`}
              sandbox="allow-scripts"
              title={p.name}
              loading="lazy"
              className="absolute top-0 left-0 border-0 pointer-events-none"
              style={{
                width: "480px",
                height: "320px",
                transform: "scale(0.208)",
                transformOrigin: "top left",
              }}
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center">
              <FileCode2 size={20} className="text-[var(--editor-text-muted)] opacity-25" />
            </div>
          )}
        </div>

        {/* Info */}
        <div
          role="button"
          tabIndex={0}
          onClick={onClick}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              onClick();
            }
          }}
          className="flex-1 min-w-0 cursor-pointer"
        >
          <h3 className="font-semibold text-[var(--editor-text)] truncate text-[13px] leading-snug">
            {p.name}
          </h3>
          <div className="flex flex-wrap items-center gap-x-2.5 gap-y-0.5 mt-1 text-[11px] text-[var(--editor-text-muted)]">
            <span className="font-mono tabular-nums">{formatDate(p.created_at, locale)}</span>
            {p.slide_count > 0 && (
              <span className="flex items-center gap-0.5">
                <Sparkles size={9} className="opacity-40" />
                {p.slide_count}
              </span>
            )}
            {p.file_size_bytes > 0 && <span>{formatSize(p.file_size_bytes)}</span>}
          </div>
          {p.current_version && (
            <span className="inline-block mt-1 px-1.5 py-0.5 rounded-md bg-[var(--editor-accent-soft)] text-[var(--editor-accent)] text-[10px] font-medium tracking-tight">
              {p.current_version}
            </span>
          )}

          {deleteError && isConfirming && (
            <p className="text-[10px] text-[var(--editor-danger)] mt-1">{deleteError}</p>
          )}
        </div>

        {/* Delete */}
        <div className="shrink-0 flex items-center">
          <AnimatePresence mode="wait">
            {isConfirming ? (
              <motion.div
                key="confirm"
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.9 }}
                transition={{ duration: 0.12 }}
                className="flex items-center gap-1"
              >
                <button
                  type="button"
                  disabled={isDeleting}
                  onClick={(e) => {
                    e.stopPropagation();
                    onDelete();
                  }}
                  className="px-2 py-1 rounded-md text-[10px] font-semibold bg-red-500/15 text-red-400 hover:bg-red-500/25 transition-colors disabled:opacity-40"
                >
                  {isDeleting ? <Loader2 size={11} className="animate-spin" /> : confirmLabel}
                </button>
                <button
                  type="button"
                  disabled={isDeleting}
                  onClick={(e) => {
                    e.stopPropagation();
                    onCancelConfirm();
                  }}
                  className="px-2 py-1 rounded-md text-[10px] font-medium text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)] transition-colors disabled:opacity-40"
                >
                  {cancelLabel}
                </button>
              </motion.div>
            ) : (
              <motion.button
                key="delete"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.12 }}
                type="button"
                disabled={isDeleting}
                onClick={(e) => {
                  e.stopPropagation();
                  onRequestConfirm();
                }}
                className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-red-400 hover:bg-red-500/10 transition-colors disabled:opacity-40"
                title={deleteLabel}
              >
                {isDeleting ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
              </motion.button>
            )}
          </AnimatePresence>
        </div>
      </div>
    </motion.div>
  );
}
