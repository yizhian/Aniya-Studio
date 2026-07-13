import React, { useCallback, useEffect, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { AlertTriangle, CheckCircle2, History, Loader2, X, RotateCcw } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { AIChatPanel } from "../chat/AIChatPanel";
import { getVersions } from "../../api/versions";
import { extractErrorMessage } from "../../models/apiResponses";

export { AIChatPanel };

interface VersionPanelProps {
  isOpen: boolean;
  onClose: () => void;
  projectId?: string | null;
  onRequestRestore?: (versionId: string) => void;
  refreshTrigger?: number;
}

function formatRelativeTime(isoString: string, t: ReturnType<typeof useLocale>["t"]): string {
  try {
    const date = new Date(isoString);
    if (isNaN(date.getTime())) return isoString;
    const now = Date.now();
    const diffMs = now - date.getTime();
    const diffSec = Math.floor(diffMs / 1000);
    if (diffSec < 60) return t.version.justNow;
    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) return `${diffMin}${t.version.minutesAgo}`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}${t.version.hoursAgo}`;
    const diffDay = Math.floor(diffHr / 24);
    if (diffDay < 30) return `${diffDay}${t.version.daysAgo}`;
    return date.toLocaleDateString("zh-CN");
  } catch {
    return isoString;
  }
}

export function VersionPanel({ isOpen, onClose, projectId, onRequestRestore, refreshTrigger }: VersionPanelProps) {
  const { t } = useLocale();

  type ApiVersion = { id: string; tag: string; title: string; slide_count: number; file_size_bytes: number; created_at: string; current: boolean };
  const [versions, setVersions] = useState<ApiVersion[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmVersion, setConfirmVersion] = useState<ApiVersion | null>(null);

  const fetchVersions = useCallback(async () => {
    if (!projectId) return;
    setLoading(true);
    setError(null);
    try {
      const res = await getVersions(projectId);
      if (!res.ok) {
        throw new Error(await extractErrorMessage(res));
      }
      const data = await res.json();
      setVersions(data.versions || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : t.panels.fetchVersionsFailed);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    if (isOpen) fetchVersions();
  }, [isOpen, fetchVersions, refreshTrigger]);

  const handleRestore = (versionId: string) => {
    setConfirmVersion(null);
    onRequestRestore?.(versionId);
  };

  if (!isOpen) return null;

  return (
    <motion.div
      initial={{ opacity: 0, y: 12, scale: 0.96 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 12, scale: 0.96 }}
      transition={{ duration: APP_CONFIG.editor.animation.panelDuration, ease: "easeOut" }}
      className="absolute left-6 bottom-24 z-50 w-80 max-h-[420px] bg-[var(--editor-surface)] backdrop-blur-xl rounded-2xl border border-[var(--editor-border)] shadow-lg flex flex-col overflow-hidden"
    >
      {/* Header */}
      <div className="h-12 flex items-center justify-between px-4 shrink-0 border-b border-[var(--editor-border)]">
        <div className="flex items-center gap-2">
          <History size={14} className="text-[var(--editor-text-muted)]" />
          <span className="text-[11px] font-semibold tracking-wider uppercase text-[var(--editor-text-muted)]">
            {t.editor.versionHistory}
          </span>
          {versions.length > 0 && (
            <span className="text-[10px] text-[var(--editor-text-muted)] font-mono tabular-nums">
              {versions.length}
            </span>
          )}
        </div>
        <button
          onClick={onClose}
          className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
        >
          <X size={14} />
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto px-4 py-4 thin-scrollbar">
        {loading && (
          <div className="flex items-center justify-center py-12">
            <Loader2 size={22} className="animate-spin text-[var(--editor-accent)]" />
          </div>
        )}

        {error && (
          <div className="flex items-start gap-2 rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-xs text-red-400">
            <AlertTriangle size={13} className="shrink-0 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        {!loading && !error && versions.length === 0 && (
          <div className="flex items-center justify-center py-12 text-xs text-[var(--editor-text-muted)]">
            {t.panels.noVersions}
          </div>
        )}

        {!loading && versions.length > 0 && (
          <div className="relative pl-6">
            {/* Timeline line */}
            <div className="absolute left-[7px] top-1 bottom-1 w-px bg-[var(--editor-border)]" />

            <div className="flex flex-col gap-6">
              {versions.map((version, i) => (
                <motion.div
                  key={version.id}
                  initial={{ opacity: 0, x: -4 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.3, ease: "easeOut", delay: i * 0.05 }}
                  className="relative"
                >
                  {/* Timeline dot — centered on the line at left-[7px] */}
                  <div
                    className={`absolute -left-[20px] top-[7px] w-[7px] h-[7px] rounded-full border-2 ${
                      version.current
                        ? "bg-[var(--editor-accent)] border-[var(--editor-accent)]"
                        : "bg-[var(--editor-surface)] border-[var(--editor-border)]"
                    }`}
                  />

                  {/* Card */}
                  <div
                    className={`rounded-xl border transition-colors ${
                      version.current
                        ? "border-[var(--editor-accent)]/20 bg-[var(--editor-accent-soft)]/50"
                        : "border-[var(--editor-border)] bg-[var(--editor-control)]/40 hover:border-[var(--editor-text-muted)]"
                    }`}
                  >
                    <div className="px-3 py-2.5">
                      {/* Top row: tag + time */}
                      <div className="flex items-center justify-between mb-1.5">
                        <span
                          className={`text-[10px] font-bold tracking-wider uppercase font-mono ${
                            version.current
                              ? "text-[var(--editor-accent)]"
                              : "text-[var(--editor-text-muted)]"
                          }`}
                        >
                          {version.tag}
                        </span>
                        <span className="text-[10px] text-[var(--editor-text-muted)] font-mono tabular-nums">
                          {formatRelativeTime(version.created_at, t)}
                        </span>
                      </div>

                      {/* Title */}
                      <p
                        className={`text-[13px] leading-snug font-medium mb-2 ${
                          version.current
                            ? "text-[var(--editor-text)]"
                            : "text-[var(--editor-text-muted)]"
                        }`}
                      >
                        {version.title}
                      </p>

                      {/* Bottom row: status or actions */}
                      {version.current ? (
                        <div className="flex items-center gap-1.5">
                          <CheckCircle2 size={11} className="text-green-400" />
                          <span className="text-[10px] text-green-400/80 font-medium uppercase tracking-wider">
                            {t.editor.currentVersion}
                          </span>
                        </div>
                      ) : confirmVersion?.id !== version.id ? (
                        <button
                          type="button"
                          onClick={() => setConfirmVersion(version)}
                          className="flex items-center gap-1.5 text-[10px] font-medium text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors"
                        >
                          <RotateCcw size={11} />
                          {t.editor.restoreVersion}
                        </button>
                      ) : (
                        <AnimatePresence mode="wait">
                          <motion.div
                            key="confirm"
                            initial={{ opacity: 0, height: 0 }}
                            animate={{ opacity: 1, height: "auto" }}
                            exit={{ opacity: 0, height: 0 }}
                            className="flex items-center gap-1.5 overflow-hidden"
                          >
                            <button
                              type="button"
                              onClick={() => handleRestore(version.id)}
                              className="text-[10px] font-medium px-2.5 py-1 rounded-md bg-[var(--editor-accent)] text-[var(--editor-bg)] hover:opacity-90 transition-opacity"
                            >
                              {t.version.confirmRestore}
                            </button>
                            <button
                              type="button"
                              onClick={() => setConfirmVersion(null)}
                              className="text-[10px] font-medium text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] px-2 py-1 rounded-md transition-colors"
                            >
                              {t.version.cancel}
                            </button>
                          </motion.div>
                        </AnimatePresence>
                      )}
                    </div>
                  </div>
                </motion.div>
              ))}
            </div>
          </div>
        )}
      </div>
    </motion.div>
  );
}
