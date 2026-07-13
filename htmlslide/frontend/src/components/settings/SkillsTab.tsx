import { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { ChevronDown, ChevronRight, Loader2 } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { useSkillList } from "../../hooks/useSkillList";
import { useSkillContent } from "../../hooks/useSkillContent";

function SkillCard({ skill }: { skill: ReturnType<typeof useSkillList>["skills"][number] }) {
  const { t } = useLocale();
  const [expanded, setExpanded] = useState(false);
  const { content, loading, error } = useSkillContent(expanded ? skill.name : null);

  return (
    <div className="rounded-xl border border-[var(--editor-border)] bg-[var(--editor-control)]/20 overflow-hidden">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="w-full flex items-center justify-between px-3 py-2.5 text-left"
      >
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-[var(--editor-text)] truncate">{skill.name}</p>
          {!expanded && skill.description && (
            <p className="text-[11px] text-[var(--editor-text-muted)] truncate mt-0.5">
              {skill.description}
            </p>
          )}
        </div>
        {expanded ? (
          <ChevronDown size={14} className="shrink-0 text-[var(--editor-text-muted)] ml-2" />
        ) : (
          <ChevronRight size={14} className="shrink-0 text-[var(--editor-text-muted)] ml-2" />
        )}
      </button>

      <AnimatePresence>
        {expanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.16, ease: "easeOut" }}
            className="overflow-hidden"
          >
            <div className="px-3 pb-3 flex flex-col gap-2 border-t border-[var(--editor-border)]">
              {skill.description && (
                <p className="text-[11px] text-[var(--editor-text-muted)] pt-2 leading-relaxed">
                  {skill.description}
                </p>
              )}
              {skill.scenario && (
                <p className="text-[11px] text-[var(--editor-text-muted)] leading-relaxed">
                  <span className="font-semibold text-[var(--editor-text)]">
                    {t.settings.scenario}:{" "}
                  </span>
                  {skill.scenario}
                </p>
              )}
              {skill.triggers && skill.triggers.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  {skill.triggers.map((trigger) => (
                    <span
                      key={trigger}
                      className="px-1.5 py-0.5 text-[10px] rounded-md bg-[var(--editor-accent-soft)] text-[var(--editor-accent)]"
                    >
                      {trigger}
                    </span>
                  ))}
                </div>
              )}
              {loading && (
                <div className="flex items-center gap-2 py-4 justify-center">
                  <Loader2 size={14} className="animate-spin text-[var(--editor-text-muted)]" />
                </div>
              )}
              {error && <p className="text-xs text-red-400 py-2">{error}</p>}
              {content && (
                <pre className="text-[11px] leading-relaxed text-[var(--editor-text-muted)] whitespace-pre-wrap font-mono max-h-[280px] overflow-y-auto thin-scrollbar p-2.5 rounded-lg bg-[var(--editor-bg)] border border-[var(--editor-border)]">
                  {content}
                </pre>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

export function SkillsTab() {
  const { t } = useLocale();
  const { skills, loading, error } = useSkillList("deck");

  if (loading && skills.length === 0) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 size={20} className="animate-spin text-[var(--editor-text-muted)]" />
      </div>
    );
  }

  if (error) {
    return <p className="text-xs text-red-400 text-center py-8">{error}</p>;
  }

  if (skills.length === 0) {
    return (
      <p className="text-xs text-[var(--editor-text-muted)] text-center py-8">
        {t.home.noSkillsAvailable}
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-[11px] text-[var(--editor-text-muted)]">
        {t.settings.skillsCount.replace("{n}", String(skills.length))}
      </p>
      <div className="flex flex-col gap-1.5">
        {skills.map((skill) => (
          <SkillCard key={skill.name} skill={skill} />
        ))}
      </div>
    </div>
  );
}
