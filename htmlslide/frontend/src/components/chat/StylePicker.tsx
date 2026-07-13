import { motion } from "motion/react";
import { Sparkles, X, Eye, Check } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";

export interface SkillOption {
  name: string;
  description: string;
  triggers?: string[];
  reason?: string;
  scenario?: string;
  has_assets?: boolean;
  has_preview?: boolean;
}

interface Props {
  skills: SkillOption[];
  onSelect: (skill: SkillOption) => void;
  onPreview: (skill: SkillOption) => void;
  onClose: () => void;
  previewedSkills?: Set<string>;
}

export function StylePicker({ skills, onSelect, onPreview, onClose, previewedSkills }: Props) {
  const { t } = useLocale();
  if (skills.length === 0) return null;

  return (
    <motion.div
      initial={{ opacity: 0, y: 12, scale: 0.97 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 8, scale: 0.97 }}
      transition={{ duration: 0.18, ease: "easeOut" }}
      className="pointer-events-auto w-full max-w-[520px] bg-[var(--editor-surface)] border border-[var(--editor-border)] rounded-2xl shadow-xl backdrop-blur-xl overflow-hidden"
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--editor-border)]">
        <div className="flex items-center gap-2">
          <Sparkles size={14} className="text-[var(--editor-accent)]" />
          <span className="text-xs font-medium text-[var(--editor-text)]">{t.chat.selectStyle}</span>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="p-0.5 rounded-md text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)] transition-colors"
        >
          <X size={14} />
        </button>
      </div>

      {/* Skill options */}
      <div className="flex flex-col max-h-[320px] overflow-y-auto thin-scrollbar p-1">
        {skills.map((sk) => {
          const wasPreviewed = previewedSkills?.has(sk.name);
          return (
            <div
              key={sk.name}
              className="px-3 py-2.5 rounded-xl hover:bg-[var(--editor-control)]/50 transition-colors"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium text-[var(--editor-text)] truncate">
                      {sk.name}
                    </p>
                    {wasPreviewed && (
                      <span className="text-[10px] text-[var(--editor-accent)] bg-[var(--editor-accent-soft)] px-1.5 py-0.5 rounded-md border border-[var(--editor-accent)]/10 shrink-0">
                        Previewed
                      </span>
                    )}
                  </div>
                  <p className="text-[11px] leading-relaxed text-[var(--editor-text-muted)] mt-0.5 line-clamp-2">
                    {sk.reason || sk.description || sk.scenario || ""}
                  </p>
                </div>
              </div>
              {sk.triggers && sk.triggers.length > 0 && (
                <div className="flex flex-wrap gap-1 mt-1.5">
                  {sk.triggers.slice(0, 5).map((t) => (
                    <span
                      key={t}
                      className="px-1.5 py-0.5 text-[10px] rounded-md bg-[var(--editor-accent-soft)] text-[var(--editor-accent)] border border-[var(--editor-accent)]/10"
                    >
                      {t}
                    </span>
                  ))}
                </div>
              )}
              {/* Action buttons */}
              <div className="flex items-center gap-2 mt-2">
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); onPreview(sk); }}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-[11px] font-medium text-[var(--editor-text-muted)] border border-[var(--editor-border)] hover:text-[var(--editor-text)] hover:border-[var(--editor-text-muted)] transition-colors"
                >
                  <Eye size={12} />
                  Preview
                </button>
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); onSelect(sk); }}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-[11px] font-medium bg-[var(--editor-accent)] text-[var(--editor-accent-text)] hover:opacity-90 transition-opacity"
                >
                  <Check size={12} />
                  Select
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </motion.div>
  );
}
