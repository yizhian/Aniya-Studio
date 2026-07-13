import { Loader2 } from "lucide-react";
import { motion } from "motion/react";

interface LoadingStateProps {
  className?: string;
}

export function LoadingState({ className }: LoadingStateProps) {
  return (
    <div className={`flex items-center justify-center py-20 ${className ?? ""}`}>
      <Loader2 size={24} className="animate-spin text-[var(--editor-text-muted)]" />
    </div>
  );
}

interface ErrorStateProps {
  message: string;
  onRetry: () => void;
  retryLabel: string;
  className?: string;
}

export function ErrorState({ message, onRetry, retryLabel, className }: ErrorStateProps) {
  return (
    <div className={`text-center py-16 px-4 ${className ?? ""}`}>
      <p className="text-sm text-[var(--editor-text-muted)] mb-3">{message}</p>
      <button
        type="button"
        onClick={onRetry}
        className="px-4 py-1.5 rounded-lg border border-[var(--editor-border)] text-xs font-medium text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
      >
        {retryLabel}
      </button>
    </div>
  );
}

interface EmptyStateProps {
  icon?: React.ComponentType<{ size?: number; className?: string }>;
  title: string;
  subtitle?: string;
  className?: string;
}

export function EmptyState({ icon: Icon, title, subtitle, className }: EmptyStateProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, ease: "easeOut" }}
      className={`text-center py-16 px-4 rounded-xl border border-dashed border-[var(--editor-border)] ${className ?? ""}`}
    >
      {Icon && <Icon size={28} className="mx-auto mb-2 text-[var(--editor-text-muted)] opacity-30" />}
      <p className="text-xs text-[var(--editor-text-muted)]">{title}</p>
      {subtitle && (
        <p className="text-[11px] text-[var(--editor-text-muted)] opacity-50 mt-1">{subtitle}</p>
      )}
    </motion.div>
  );
}
