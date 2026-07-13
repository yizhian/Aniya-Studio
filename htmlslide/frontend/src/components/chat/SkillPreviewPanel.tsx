import { useState, useEffect } from "react";
import { motion } from "motion/react";
import { X, ChevronLeft, ChevronRight, Loader2 } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { Portal } from "../common/Portal";
import { Z_INDEX } from "../../constants/zIndex";
import type { SkillOption } from "./StylePicker";

interface Props {
  skill: SkillOption;
  skills: SkillOption[];
  onClose: () => void;
  onUseStyle: (skill: SkillOption) => void;
  onNavigate: (skill: SkillOption) => void;
}

export function SkillPreviewPanel({ skill, skills, onClose, onUseStyle, onNavigate }: Props) {
  const { t } = useLocale();
  const [loading, setLoading] = useState(true);
  const [noPreview, setNoPreview] = useState(false);

  const currentIndex = skills.findIndex((s) => s.name === skill.name);
  const hasPrev = currentIndex > 0;
  const hasNext = currentIndex < skills.length - 1;

  // Reset loading state whenever the skill changes.
  useEffect(() => {
    if (!skill.has_preview) {
      setLoading(false);
      setNoPreview(true);
      return;
    }
    setLoading(true);
    setNoPreview(false);
  }, [skill.name, skill.has_preview]);

  // Only Escape — no ArrowLeft/Right to avoid conflict with slide navigation inside example.html.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const goTo = (direction: "prev" | "next") => {
    const idx = direction === "prev" ? currentIndex - 1 : currentIndex + 1;
    if (idx < 0 || idx >= skills.length) return;
    onNavigate(skills[idx]);
  };

  const previewSrc = `/api/v1/skills/${encodeURIComponent(skill.name)}/preview`;

  return (
    <Portal>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.15 }}
        className="fixed inset-0 flex"
        style={{ zIndex: Z_INDEX.OVERLAY }}
      >
      {/* Left overlay */}
      <div className="flex-1 bg-black/50 cursor-pointer" onClick={onClose} />

      {/* Right panel — 900px for better 16:9 preview space */}
      <motion.div
        initial={{ x: 80, opacity: 0 }}
        animate={{ x: 0, opacity: 1 }}
        exit={{ x: 80, opacity: 0 }}
        transition={{ duration: 0.18, ease: "easeOut" }}
        className="w-[900px] flex flex-col bg-[var(--editor-surface)] border-l border-[var(--editor-border)] shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--editor-border)] shrink-0">
          <div className="min-w-0 flex-1">
            <h3 className="text-sm font-semibold text-[var(--editor-text)] truncate">
              {skill.name}
            </h3>
            {skill.description && (
              <p className="text-[11px] text-[var(--editor-text-muted)] mt-0.5 line-clamp-1">
                {skill.description}
              </p>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)] transition-colors ml-2"
          >
            <X size={16} />
          </button>
        </div>

        {/* Preview area — iframe fills the full flex space; the HTML's own scaleWrapper handles viewport scaling */}
        <div className="flex-1 relative overflow-hidden bg-[var(--editor-bg)]">
          {loading && !noPreview && (
            <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 text-[var(--editor-text-muted)] bg-[var(--editor-bg)]">
              <Loader2 size={24} className="animate-spin" />
              <span className="text-xs">{t.home.loadingSkillPreview}</span>
            </div>
          )}
          {noPreview ? (
            <div className="absolute inset-0 flex items-center justify-center text-center text-[var(--editor-text-muted)]">
              <p className="text-sm">{t.home.noSkillPreview}</p>
            </div>
          ) : (
            <iframe
              key={skill.name}
              src={previewSrc}
              className="absolute inset-0 w-full h-full border-0"
              sandbox="allow-scripts"
              title={`Preview: ${skill.name}`}
              onLoad={() => setLoading(false)}
              onError={() => {
                setLoading(false);
                setNoPreview(true);
              }}
            />
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-5 py-4 border-t border-[var(--editor-border)] shrink-0">
          <div className="flex items-center gap-1">
            <button
              type="button"
              disabled={!hasPrev}
              onClick={() => goTo("prev")}
              className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft size={18} />
            </button>
            <span className="text-[11px] text-[var(--editor-text-muted)] mx-1 tabular-nums">
              {currentIndex + 1} / {skills.length}
            </span>
            <button
              type="button"
              disabled={!hasNext}
              onClick={() => goTo("next")}
              className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronRight size={18} />
            </button>
          </div>

          <button
            type="button"
            onClick={() => onUseStyle(skill)}
            className="px-4 py-2 rounded-lg bg-[var(--editor-accent)] text-[var(--editor-accent-text)] text-sm font-medium hover:opacity-90 transition-opacity"
          >
            {t.home.useThisStyle}
          </button>
        </div>
      </motion.div>
      </motion.div>
    </Portal>
  );
}
