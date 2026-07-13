import { useRef, useEffect } from "react";
import { motion, AnimatePresence } from "motion/react";
import { Portal } from "./Portal";
import { Z_INDEX } from "../../constants/zIndex";

type PanelPosition = "left" | "right";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  position?: PanelPosition;
  width?: number;
  children: React.ReactNode;
  backdropClassName?: string;
  panelClassName?: string;
  /** Escape key closes by default. Set false to disable. */
  closeOnEscape?: boolean;
}

const POSITION_STYLES: Record<PanelPosition, { initial: object; exit: object }> = {
  left: {
    initial: { x: -320, opacity: 0 },
    exit: { x: -320, opacity: 0 },
  },
  right: {
    initial: { x: "100%", opacity: 0 },
    exit: { x: "100%", opacity: 0 },
  },
};

export function AnimatedPanel({
  isOpen,
  onClose,
  position = "left",
  width = 320,
  children,
  backdropClassName,
  panelClassName,
  closeOnEscape = true,
}: Props) {
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen || !closeOnEscape) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [isOpen, onClose, closeOnEscape]);

  const variants = POSITION_STYLES[position];
  const isRight = position === "right";

  return (
    <Portal>
      <AnimatePresence>
        {isOpen && (
          <>
            <motion.div
              key="panel-backdrop"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2 }}
              onClick={onClose}
              className={`fixed inset-0 bg-black/20 backdrop-blur-sm ${backdropClassName ?? ""}`}
              style={{ zIndex: Z_INDEX.OVERLAY_BACKDROP }}
            />
            <motion.div
              key="panel-content"
              ref={panelRef}
              initial={variants.initial}
              animate={{ x: 0, opacity: 1 }}
              exit={variants.exit}
              transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
              style={{
                ...(isRight ? { width, minWidth: 300, maxWidth: 360 } : {}),
                zIndex: Z_INDEX.OVERLAY,
              }}
              className={
                panelClassName ??
                (isRight
                  ? "fixed inset-y-0 right-0 flex flex-col bg-[var(--editor-surface)] backdrop-blur-xl border-l border-[var(--editor-border)] shadow-xl"
                  : "fixed inset-y-0 left-0 w-80 flex flex-col bg-[var(--editor-surface)] backdrop-blur-xl border-r border-[var(--editor-border)] shadow-xl")
              }
            >
              {children}
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </Portal>
  );
}
