import { useEffect, useRef, useState, type RefObject } from "react";
import { motion, AnimatePresence } from "motion/react";
import { Wand2, ChevronDown, Loader2, X, Eye } from "lucide-react";
import { useSkillList, type SkillItem } from "../../hooks/useSkillList";
import { useLocale } from "../../context/LocaleContext";
import { SkillPreviewPanel } from "../chat/SkillPreviewPanel";
import { Portal } from "../common/Portal";
import { Z_INDEX } from "../../constants/zIndex";
import type { SkillOption } from "../chat/StylePicker";

interface Props {
  selectedSkill: string | null;
  onSelect: (skillName: string, prompt: string) => void;
  onClear: () => void;
}

const THUMB_W = 300;
const THUMB_H = 169;
const IFRAME_W = 1280;
const IFRAME_H = 720;
const THUMB_SCALE = THUMB_W / IFRAME_W;

/** Load preview iframe only when the card enters the gallery scrollport. */
function useLazyPreviewVisible(scrollRootRef: RefObject<HTMLElement | null>, enabled: boolean) {
  const targetRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (!enabled) {
      setVisible(false);
      return;
    }

    const target = targetRef.current;
    const root = scrollRootRef.current;
    if (!target || !root) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { root, rootMargin: "120px", threshold: 0.01 },
    );

    observer.observe(target);
    return () => observer.disconnect();
  }, [scrollRootRef, enabled]);

  return { targetRef, visible };
}

function toSkillOption(skill: SkillItem): SkillOption {
  return {
    name: skill.name,
    description: skill.description,
    triggers: skill.triggers,
    scenario: skill.scenario,
    has_assets: skill.has_assets,
    has_preview: skill.has_preview,
  };
}

function SkillGalleryCard({
  skill,
  scrollRootRef,
  onPreview,
  onUse,
}: {
  skill: SkillItem;
  scrollRootRef: RefObject<HTMLElement | null>;
  onPreview: () => void;
  onUse: () => void;
}) {
  const { t } = useLocale();
  const { targetRef, visible } = useLazyPreviewVisible(
    scrollRootRef,
    Boolean(skill.has_preview),
  );
  const previewSrc = `/api/v1/skills/${encodeURIComponent(skill.name)}/preview`;

  return (
    <div
      className="shrink-0 w-[300px] flex flex-col rounded-2xl border border-[var(--editor-border)] bg-[var(--editor-control)]/20 overflow-hidden snap-start"
      style={{ scrollSnapAlign: "start" }}
    >
      <button
        type="button"
        onClick={onPreview}
        className="relative w-full text-left group"
        style={{ height: THUMB_H }}
      >
        <div ref={targetRef} className="absolute inset-0 bg-[var(--editor-bg)] overflow-hidden">
          {skill.has_preview ? (
            visible ? (
              <iframe
                src={previewSrc}
                sandbox="allow-scripts"
                title={`${t.home.skillPreview}: ${skill.name}`}
                className="absolute top-0 left-0 border-0 pointer-events-none"
                style={{
                  width: IFRAME_W,
                  height: IFRAME_H,
                  transform: `scale(${THUMB_SCALE})`,
                  transformOrigin: "top left",
                }}
              />
            ) : (
              <div className="w-full h-full flex items-center justify-center">
                <Loader2 size={16} className="animate-spin text-[var(--editor-text-muted)]/50" />
              </div>
            )
          ) : (
            <div className="w-full h-full flex items-center justify-center px-4">
              <p className="text-[11px] text-[var(--editor-text-muted)] text-center line-clamp-4 leading-relaxed">
                {skill.description || skill.scenario || t.home.noSkillPreview}
              </p>
            </div>
          )}
        </div>
        <div className="absolute inset-0 bg-black/0 group-hover:bg-black/10 transition-colors flex items-center justify-center opacity-0 group-hover:opacity-100">
          <span className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-[var(--editor-text)] text-[var(--editor-bg)]">
            <Eye size={13} />
            {t.home.viewFullPreview}
          </span>
        </div>
      </button>

      <div className="px-3 py-3 flex flex-col gap-2 border-t border-[var(--editor-border)]">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-[var(--editor-text)] truncate">
            {skill.name}
          </p>
          {skill.description && (
            <p className="text-[11px] text-[var(--editor-text-muted)] mt-0.5 line-clamp-2 leading-relaxed">
              {skill.description}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={onPreview}
            className="flex-1 py-1.5 rounded-lg text-[11px] font-medium border border-[var(--editor-border)] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:border-[var(--editor-text-muted)] transition-colors"
          >
            {t.home.skillPreview}
          </button>
          <button
            type="button"
            onClick={onUse}
            className="flex-1 py-1.5 rounded-lg text-[11px] font-semibold transition-opacity hover:opacity-85"
            style={{
              backgroundColor: "var(--editor-text)",
              color: "var(--editor-bg)",
            }}
          >
            {t.home.useSkillTemplate}
          </button>
        </div>
      </div>
    </div>
  );
}

export function SkillTemplatePicker({ selectedSkill, onSelect, onClear }: Props) {
  const { t } = useLocale();
  const [galleryOpen, setGalleryOpen] = useState(false);
  const [previewSkill, setPreviewSkill] = useState<SkillItem | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const { skills, loading } = useSkillList("deck");

  const skillOptions = skills.map(toSkillOption);

  useEffect(() => {
    if (!galleryOpen) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape" && !previewSkill) setGalleryOpen(false);
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [galleryOpen, previewSkill]);

  function handleApply(skill: SkillItem) {
    const prompt = t.home.skillPromptTemplate.replace("{skillName}", skill.name);
    onSelect(skill.name, prompt);
    setPreviewSkill(null);
    setGalleryOpen(false);
  }

  return (
    <>
      <div className="relative flex items-center gap-2" ref={ref}>
        {selectedSkill && (
          <button
            type="button"
            onClick={onClear}
            className="flex items-center gap-1.5 px-2 py-1 rounded-lg text-[11px] font-medium border border-[var(--editor-border)] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:border-[var(--editor-text-muted)] transition-colors max-w-[140px]"
            title={t.home.clearSkillTemplate}
          >
            <span className="truncate">{selectedSkill}</span>
            <X size={11} className="shrink-0" />
          </button>
        )}

        <button
          type="button"
          onClick={() => setGalleryOpen(true)}
          className="flex items-center gap-1 text-[var(--editor-text)] hover:opacity-60 transition-opacity"
          title={t.home.skillTemplates}
          disabled={loading && skills.length === 0}
        >
          <Wand2 size={20} strokeWidth={2.5} />
          {skills.length > 0 && (
            <ChevronDown size={12} strokeWidth={2.5} />
          )}
        </button>
      </div>

      <Portal>
      <AnimatePresence>
        {galleryOpen && (
          <>
            <motion.div
              key="gallery-backdrop"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.18 }}
              className="fixed inset-0 bg-black/25 backdrop-blur-[2px]"
              style={{ zIndex: Z_INDEX.OVERLAY_BACKDROP }}
              onClick={() => setGalleryOpen(false)}
            />

            <motion.div
              key="gallery-panel"
              initial={{ opacity: 0, y: 32 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 24 }}
              transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
              className="fixed inset-x-0 bottom-0 flex justify-center px-4 pb-6 pointer-events-none"
              style={{ zIndex: Z_INDEX.OVERLAY }}
            >
              <div
                className="pointer-events-auto w-full max-w-[1100px] flex flex-col rounded-[20px] bg-[var(--editor-surface)] overflow-hidden"
                style={{ border: "3px solid var(--editor-text)" }}
                onClick={(e) => e.stopPropagation()}
              >
                <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--editor-border)] shrink-0">
                  <div className="flex items-center gap-2.5">
                    <div
                      className="w-2 h-2 rounded-full shrink-0"
                      style={{ backgroundColor: "var(--editor-text)" }}
                    />
                    <h2 className="text-[11px] font-bold tracking-[0.15em] text-[var(--editor-text)] uppercase">
                      {t.home.skillTemplates}
                    </h2>
                    <span className="text-[11px] text-[var(--editor-text-muted)] font-mono tabular-nums">
                      {skills.length}
                    </span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setGalleryOpen(false)}
                    className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)] transition-colors"
                  >
                    <X size={16} />
                  </button>
                </div>

                <p className="px-5 pt-3 pb-1 text-[11px] text-[var(--editor-text-muted)]">
                  {t.home.skillGalleryHint}
                </p>

                <div
                  ref={scrollRef}
                  className="px-5 py-4 overflow-x-auto thin-scrollbar snap-x snap-mandatory"
                >
                  {loading && skills.length === 0 && (
                    <div className="flex items-center justify-center py-16">
                      <Loader2 size={20} className="animate-spin text-[var(--editor-text-muted)]" />
                    </div>
                  )}
                  {!loading && skills.length === 0 && (
                    <p className="text-sm text-[var(--editor-text-muted)] text-center py-16">
                      {t.home.noSkillsAvailable}
                    </p>
                  )}
                  <div className="flex gap-4 pb-1">
                    {skills.map((skill, i) => (
                      <motion.div
                        key={skill.name}
                        initial={{ opacity: 0, y: 12 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{
                          duration: 0.3,
                          ease: [0.16, 1, 0.3, 1],
                          delay: i * 0.04,
                        }}
                      >
                        <SkillGalleryCard
                          skill={skill}
                          scrollRootRef={scrollRef}
                          onPreview={() => setPreviewSkill(skill)}
                          onUse={() => handleApply(skill)}
                        />
                      </motion.div>
                    ))}
                  </div>
                </div>
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>
      </Portal>

      <AnimatePresence>
        {previewSkill && (
          <SkillPreviewPanel
            skill={toSkillOption(previewSkill)}
            skills={skillOptions}
            onClose={() => setPreviewSkill(null)}
            onUseStyle={(sk) => {
              const item = skills.find((s) => s.name === sk.name);
              if (item) handleApply(item);
            }}
            onNavigate={(sk) => {
              const item = skills.find((s) => s.name === sk.name);
              if (item) setPreviewSkill(item);
            }}
          />
        )}
      </AnimatePresence>
    </>
  );
}
