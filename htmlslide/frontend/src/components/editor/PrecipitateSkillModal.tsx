import React, { useCallback, useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Bookmark, Loader2, RefreshCw, X } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { precipitateStream, precipitateConfirm } from "../../api/skills";
import { safeJson, extractErrorMessage, type ApiErrorResponse } from "../../models/apiResponses";
import { logError } from "../../utils/errorLogger";
import { Portal } from "../common/Portal";
import { Z_INDEX } from "../../constants/zIndex";

type Phase = "loading" | "streaming" | "preview" | "saving" | "done";

interface PreviewData {
  suggested_name: string;
  description: string;
  scenario: string;
  skill_md: string;
  example_html: string;
}

type Props = {
  projectId: string;
  htmlContent: string;
  onClose: () => void;
};

const SCENARIOS = ["marketing", "pitch-deck", "tech-sharing", "internal", "product-launch"];

function scenarioOptions(current: string) {
  const options = [...SCENARIOS];
  if (current && !options.includes(current)) {
    options.unshift(current);
  }
  return options;
}

export function PrecipitateSkillModal({ projectId, htmlContent, onClose }: Props) {
  const { t } = useLocale();
  const [phase, setPhase] = useState<Phase>("loading");
  const [editedName, setEditedName] = useState("");
  const [editedDescription, setEditedDescription] = useState("");
  const [editedScenario, setEditedScenario] = useState("");
  const [preview, setPreview] = useState<PreviewData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [streamingText, setStreamingText] = useState("");
  const [savedName, setSavedName] = useState("");
  const [savedPath, setSavedPath] = useState("");
  const resultHandledRef = useRef(false);
  const overlayRef = useRef<HTMLDivElement>(null);

  const callPreview = useCallback(async (html: string) => {
    setPhase("streaming");
    setError(null);
    setStreamingText(t.precipitate.analyzing);
    resultHandledRef.current = false;
    try {
      const res = await precipitateStream(projectId, html);
      if (!res.ok) {
        const body = await safeJson<ApiErrorResponse>(res);
        throw new Error(body.detail || `Server error (${res.status})`);
      }

      const reader = res.body?.getReader();
      if (!reader) throw new Error("No response body");
      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          try {
            const event = JSON.parse(line.slice(6));
            switch (event.type) {
              case "thinking":
              case "text":
                setStreamingText(event.data?.text || "");
                break;
              case "tool_call_start":
                setStreamingText(event.data?.name ? `Running ${event.data.name}...` : t.precipitate.analyzing);
                break;
              case "precipitate_result": {
                const d = event.data as PreviewData;
                setPreview(d);
                setEditedName(d.suggested_name);
                setEditedDescription(d.description);
                setEditedScenario(d.scenario);
                setPhase("preview");
                resultHandledRef.current = true;
                return;
              }
              case "error":
                setError(event.data?.message || "Agent error");
                setPhase("preview");
                resultHandledRef.current = true;
                return;
            }
          } catch {
            // Skip malformed JSON lines
          }
        }
      }
      throw new Error("Stream ended without result");
    } catch (e) {
      if (resultHandledRef.current) return;
      setError(e instanceof Error ? e.message : "Failed to generate skill preview");
      setPhase("preview");
    }
  }, [projectId, t]);

  useEffect(() => {
    callPreview(htmlContent);
  }, [htmlContent, callPreview]);

  const handleConfirm = useCallback(async () => {
    if (!preview) return;
    setPhase("saving");
    setError(null);
    const patchedSkillMd = preview.skill_md.replace(
      /^description:.*$/m,
      `description: ${editedDescription}`,
    );
    try {
      const res = await precipitateConfirm({
        project_id: projectId,
        skill_name: editedName,
        scenario: editedScenario,
        skill_md: patchedSkillMd,
        example_html: preview.example_html,
      });
      if (res.status === 409) {
        const body = await safeJson<ApiErrorResponse>(res);
        setError(body.error || t.precipitate.nameConflict);
        setPhase("preview");
        return;
      }
      if (!res.ok) {
        const body = await safeJson<ApiErrorResponse>(res);
        throw new Error(body.detail || `Server error (${res.status})`);
      }
      const data = await safeJson<{ skill_name?: string; dir_path?: string }>(res);
      setSavedName(data.skill_name || editedName);
      setSavedPath(data.dir_path || "");
      setPhase("done");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save skill");
      setPhase("preview");
    }
  }, [projectId, preview, editedName, editedDescription, editedScenario, t]);

  const handleRegenerate = useCallback(() => {
    callPreview(htmlContent);
  }, [htmlContent, callPreview]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  useEffect(() => {
    if (!overlayRef.current) return;
    const onPointerDown = (e: MouseEvent) => {
      if (e.target === overlayRef.current) onClose();
    };
    const el = overlayRef.current;
    el.addEventListener("pointerdown", onPointerDown);
    return () => el.removeEventListener("pointerdown", onPointerDown);
  }, [onClose]);

  return (
    <Portal>
    <AnimatePresence>
      <motion.div
        ref={overlayRef}
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.15 }}
        className="fixed inset-0 flex items-center justify-center bg-black/40"
        style={{ zIndex: Z_INDEX.OVERLAY }}
      >
        <motion.div
          initial={{ opacity: 0, y: 12, scale: 0.97 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 12, scale: 0.97 }}
          transition={{ duration: 0.2, ease: "easeOut" }}
          className="relative w-[960px] max-h-[88vh] bg-[var(--editor-surface)] border border-[var(--editor-border)] rounded-2xl shadow-xl flex flex-col overflow-hidden"
        >
          {/* Header */}
          <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--editor-border)] shrink-0">
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 rounded-lg bg-[var(--editor-accent)] flex items-center justify-center">
                <Bookmark size={14} className="text-[var(--editor-accent-text)]" />
              </div>
              <span className="text-sm font-semibold text-[var(--editor-text)]">
                {t.precipitate.title}
              </span>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="w-7 h-7 rounded-lg flex items-center justify-center text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
            >
              <X size={16} />
            </button>
          </div>

          {/* Body */}
          <div className="flex-1 overflow-y-auto p-5">
            {(phase === "loading" || phase === "streaming") && (
              <div className="flex flex-col items-center justify-center py-16 gap-4">
                <Loader2 size={28} className="animate-spin text-[var(--editor-accent)]" />
                <div className="flex flex-col items-center gap-1.5">
                  <span className="text-sm font-medium text-[var(--editor-text)]">
                    {t.precipitate.extracting}
                  </span>
                  {streamingText && (
                    <span className="text-xs text-[var(--editor-text-muted)] max-w-md text-center truncate">
                      {streamingText}
                    </span>
                  )}
                </div>
                <div className="w-64 h-1 rounded-full bg-[var(--editor-border)] overflow-hidden">
                  <motion.div
                    className="h-full rounded-full bg-[var(--editor-accent)]"
                    animate={{ x: ["-100%", "400%"] }}
                    transition={{ duration: 1.5, repeat: Infinity, ease: "easeInOut" }}
                  />
                </div>
              </div>
            )}

            {(phase === "preview" || phase === "saving") && (
              <>
                {error && (
                  <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
                    {error}
                  </div>
                )}

                {/* Metadata */}
                <div className="space-y-4 mb-5">
                  {/* Row 1: Name + Scenario */}
                  <div className="flex gap-3">
                    <div className="flex-1">
                      <label className="block text-[11px] font-medium text-[var(--editor-text-muted)] mb-1.5 uppercase tracking-wider">
                        {t.precipitate.nameLabel}
                      </label>
                      <input
                        type="text"
                        value={editedName}
                        onChange={(e) => setEditedName(e.target.value)}
                        disabled={phase === "saving"}
                        placeholder="my-skill-name"
                        className="w-full px-3 py-2 rounded-lg bg-[var(--editor-bg)] border border-[var(--editor-border)] text-sm text-[var(--editor-text)] placeholder:text-[var(--editor-text-muted)]/40 focus:outline-none focus:border-[var(--editor-accent)] transition-colors disabled:opacity-50"
                      />
                    </div>
                    <div className="w-[180px] shrink-0">
                      <label className="block text-[11px] font-medium text-[var(--editor-text-muted)] mb-1.5 uppercase tracking-wider">
                        {t.precipitate.scenarioLabel}
                      </label>
                      <select
                        value={editedScenario}
                        onChange={(e) => setEditedScenario(e.target.value)}
                        disabled={phase === "saving"}
                        className="w-full px-3 py-2 rounded-lg bg-[var(--editor-bg)] border border-[var(--editor-border)] text-sm text-[var(--editor-text)] focus:outline-none focus:border-[var(--editor-accent)] transition-colors disabled:opacity-50 appearance-none cursor-pointer"
                      >
                        {scenarioOptions(editedScenario).map((s) => (
                          <option key={s} value={s}>{s}</option>
                        ))}
                      </select>
                    </div>
                  </div>

                  {/* Row 2: Description textarea */}
                  <div>
                    <label className="block text-[11px] font-medium text-[var(--editor-text-muted)] mb-1.5 uppercase tracking-wider">
                      {t.precipitate.descriptionLabel}
                    </label>
                    <textarea
                      value={editedDescription}
                      onChange={(e) => setEditedDescription(e.target.value)}
                      disabled={phase === "saving"}
                      rows={3}
                      className="w-full px-3 py-2 rounded-lg bg-[var(--editor-bg)] border border-[var(--editor-border)] text-sm text-[var(--editor-text)] placeholder:text-[var(--editor-text-muted)]/40 focus:outline-none focus:border-[var(--editor-accent)] transition-colors disabled:opacity-50 resize-y min-h-[72px]"
                    />
                  </div>
                </div>

                {/* SKILL.md */}
                <label className="block text-[11px] font-medium text-[var(--editor-text-muted)] mb-1.5 uppercase tracking-wider">
                  SKILL.md
                </label>
                <div className="h-[420px] overflow-auto rounded-lg bg-[var(--editor-bg)] border border-[var(--editor-border)] p-4">
                  <pre className="text-[13px] text-[var(--editor-text)] font-mono whitespace-pre-wrap leading-relaxed break-words">
                    {preview?.skill_md}
                  </pre>
                </div>

                {/* Actions */}
                <div className="flex items-center justify-end gap-2 mt-4 pt-3 border-t border-[var(--editor-border)]">
                  <button
                    type="button"
                    onClick={handleRegenerate}
                    disabled={phase === "saving"}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors disabled:opacity-50"
                  >
                    <RefreshCw size={13} />
                    {t.precipitate.regenerate}
                  </button>
                  <button
                    type="button"
                    onClick={handleConfirm}
                    disabled={phase === "saving" || !editedName.trim()}
                    className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-xs font-medium bg-[var(--editor-accent)] text-[var(--editor-accent-text)] hover:opacity-90 transition-opacity disabled:opacity-50"
                  >
                    {phase === "saving" && <Loader2 size={13} className="animate-spin" />}
                    {phase === "saving" ? t.precipitate.saving : t.precipitate.confirm}
                  </button>
                </div>
              </>
            )}

            {phase === "done" && (
              <div className="flex flex-col items-center justify-center py-16 gap-3">
                <div className="w-10 h-10 rounded-full bg-green-500/20 flex items-center justify-center">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="text-green-400">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                </div>
                <span className="text-sm font-medium text-[var(--editor-text)]">
                  {t.precipitate.saved}
                </span>
                <div className="flex flex-col items-center gap-1 mt-2 p-4 rounded-lg bg-[var(--editor-bg)] border border-[var(--editor-border)] min-w-[320px]">
                  <div className="text-[11px] font-medium text-[var(--editor-text-muted)] uppercase tracking-wider mb-1">
                    {t.precipitate.nameLabel}
                  </div>
                  <span className="text-sm font-mono text-[var(--editor-text)]">{savedName}</span>
                  {savedPath && (
                    <>
                      <div className="text-[11px] font-medium text-[var(--editor-text-muted)] uppercase tracking-wider mt-3 mb-1">
                        Save Path
                      </div>
                      <div className="flex items-center gap-2">
                        <code className="text-xs text-[var(--editor-text-muted)] bg-[var(--editor-surface)] px-2 py-1 rounded">
                          {savedPath}
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText(savedPath)}
                          className="text-xs text-[var(--editor-accent)] hover:underline"
                        >
                          Copy
                        </button>
                      </div>
                    </>
                  )}
                </div>
                <button
                  type="button"
                  onClick={onClose}
                  className="mt-2 px-4 py-1.5 rounded-lg text-xs font-medium bg-[var(--editor-accent)] text-[var(--editor-accent-text)] hover:opacity-90 transition-opacity"
                >
                  {t.precipitate.done}
                </button>
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
    </Portal>
  );
}
