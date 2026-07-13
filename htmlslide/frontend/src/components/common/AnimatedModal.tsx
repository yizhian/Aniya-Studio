import { useEffect, useRef } from "react";
import { motion, AnimatePresence } from "motion/react";
import { X } from "lucide-react";
import { Portal } from "./Portal";
import { Z_INDEX } from "../../constants/zIndex";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  className?: string;
  /** Escape key closes by default. Set false to disable. */
  closeOnEscape?: boolean;
}

export function AnimatedModal({
  isOpen,
  onClose,
  title,
  children,
  className,
  closeOnEscape = true,
}: Props) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen || !closeOnEscape) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [isOpen, onClose, closeOnEscape]);

  return (
    <Portal>
      <AnimatePresence>
        {isOpen && (
          <div
            ref={containerRef}
            className="fixed inset-0 flex items-center justify-center"
            style={{ zIndex: Z_INDEX.OVERLAY }}
          >
            <motion.div
              key="modal-backdrop"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2 }}
              onClick={onClose}
              className="absolute inset-0 bg-black/30 backdrop-blur-sm"
            />
            <motion.div
              key="modal-content"
              initial={{ opacity: 0, scale: 0.95, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 10 }}
              transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
              className={`relative z-10 bg-[var(--editor-surface)] border border-[var(--editor-border)] rounded-2xl shadow-2xl overflow-hidden ${className ?? ""}`}
            >
              {title && (
                <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--editor-border)]">
                  <h2 className="text-sm font-bold tracking-wide text-[var(--editor-text)]">
                    {title}
                  </h2>
                  <button
                    type="button"
                    onClick={onClose}
                    className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
                  >
                    <X size={16} />
                  </button>
                </div>
              )}
              {children}
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </Portal>
  );
}
