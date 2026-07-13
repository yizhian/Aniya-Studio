import { useEffect, useMemo, useRef, useState } from "react";
import { motion } from "motion/react";
import {
  CheckCircle2,
  Loader2,
  Sparkles,
  StopCircle,
  ChevronRight,
  Lightbulb,
} from "lucide-react";
import type { ChatState, TimelineEntry } from "../../models/chat";
import { UserPromptCard } from "./UserPromptCard";
import { InlineMarkdown } from "./InlineMarkdown";
import { AgentLog } from "./AgentLog";
import { ToolPill } from "./ToolPill";
import { TodoInline } from "./TodoInline";
import { BreathingBorder } from "../common/BreathingBorder";
import { useLocale } from "../../context/LocaleContext";

// ─── Main Panel ─────────────────────────────────────────────────────────────────

interface Round {
  userMessage: TimelineEntry;
  events: TimelineEntry[];
}

interface Props {
  isOpen: boolean;
  onClose: () => void;
  chatState: ChatState;
  onStop?: () => void;
  onRetry?: () => void;
}

export function AIChatPanel({ isOpen, onClose, chatState, onStop, onRetry }: Props) {
  const { t } = useLocale();
  const scrollRef = useRef<HTMLDivElement>(null);
  const [thinkingExpanded, setThinkingExpanded] = useState(false);
  const [activeRoundIdx, setActiveRoundIdx] = useState(-1); // -1 = latest

  const isProcessing = chatState.streamStatus === "streaming" || chatState.streamStatus === "connecting";

  // ─── Split timeline into rounds ──────────────────────────────────────────
  const rounds = useMemo((): Round[] => {
    const result: Round[] = [];
    let current: TimelineEntry[] = [];
    let currentUser: TimelineEntry | null = null;

    for (const entry of chatState.timeline) {
      if (entry.event === "user_message") {
        if (currentUser && current.length > 0) {
          result.push({ userMessage: currentUser, events: current });
        }
        currentUser = entry;
        current = [];
      } else {
        current.push(entry);
      }
    }
    if (currentUser) {
      result.push({ userMessage: currentUser, events: current });
    }
    return result;
  }, [chatState.timeline]);

  const activeRound: Round | null =
    rounds.length === 0 ? null
    : activeRoundIdx >= 0 && activeRoundIdx < rounds.length ? rounds[activeRoundIdx]
    : rounds[rounds.length - 1];

  // Auto-scroll during streaming
  useEffect(() => {
    if (!scrollRef.current || !isProcessing) return;
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [chatState.timeline, isProcessing]);

  if (!isOpen) return null;

  return (
    <motion.div
      initial={{ opacity: 0, x: -20, scale: 0.95 }}
      animate={{ opacity: 1, x: 0, scale: 1 }}
      exit={{ opacity: 0, x: -20, scale: 0.95 }}
      transition={{ duration: 0.2, ease: "easeOut" }}
      className="absolute left-24 top-16 z-50 w-[420px]"
      style={{ maxHeight: "calc(100vh - 144px)" }}
    >
      <BreathingBorder active={isProcessing}>
        <div className="h-full max-h-[calc(100vh-144px)] bg-[var(--editor-surface)] backdrop-blur-xl rounded-2xl border border-[var(--editor-border)] shadow-lg flex flex-col overflow-hidden relative">

          {/* ── Collapse handle ── */}
          <div className="h-2 shrink-0" />
          <div className="px-3 h-10 flex items-center shrink-0 relative">
            <button onClick={onClose} className="appearance-none bg-transparent border-0 p-0 cursor-pointer rounded-full hover:opacity-80 transition-opacity" title={t.chat.collapsePanel}>
              <svg width="40" height="16" viewBox="0 0 39 16" fill="transparent"><rect width="38.68" height="15.99" rx="8" className="fill-[var(--editor-text)]" /><circle cx="15.09" cy="7.62" r="2.05" className="fill-[var(--editor-surface)]" /><circle cx="22.9" cy="7.62" r="2.05" className="fill-[var(--editor-surface)]" /></svg>
            </button>
            <div className="flex gap-2 items-center absolute right-2 top-1/2 -translate-y-1/2">
              {isProcessing && (
                <span className="flex items-center gap-1 px-2 py-0.5 rounded-full bg-[var(--editor-accent-soft)] border border-[var(--editor-accent)]/20 text-[10px] text-[var(--editor-accent)] animate-pulse">
                  <Sparkles size={10} />{t.chat.thinking}
                </span>
              )}
              {chatState.streamStatus === "done" && activeRoundIdx === -1 && (
                <span className="flex items-center gap-1 px-2 py-0.5 rounded-full bg-green-500/5 border border-green-400/20 text-[10px] text-green-400">
                  <CheckCircle2 size={10} />{t.chat.done}
                </span>
              )}
            </div>
          </div>

          {/* ── Round navigator (shown when viewing history) ── */}
          {activeRoundIdx >= 0 && activeRoundIdx < rounds.length - 1 && (
            <div className="px-3 pb-2 shrink-0">
              <button
                type="button"
                onClick={() => setActiveRoundIdx(-1)}
                className="text-[11px] text-[var(--editor-accent)] hover:opacity-80 transition-opacity flex items-center gap-1"
              >
                <ChevronRight size={12} className="rotate-180" />
                {t.chat.backToLatest}
              </button>
            </div>
          )}

          {/* ── User prompt card ── */}
          {activeRound && (
            <UserPromptCard
              content={activeRound.userMessage.data.content}
              domContext={activeRound.userMessage.data.dom_context}
              attachments={activeRound.userMessage.data.attachments}
              defaultExpanded={false}
            />
          )}

          {/* ── Event stream ── */}
          <div
            ref={scrollRef}
            className="flex-1 min-h-0 overflow-y-auto thin-scrollbar flex flex-col select-text"
            style={{
              maskImage: "linear-gradient(black calc(100% - 48px), rgba(0,0,0,0.6) calc(100% - 24px), rgba(0,0,0,0.15) calc(100% - 8px), transparent)",
              WebkitMaskImage: "linear-gradient(black calc(100% - 48px), rgba(0,0,0,0.6) calc(100% - 24px), rgba(0,0,0,0.15) calc(100% - 8px), transparent)",
            }}
          >
            <div className="flex flex-col flex-1 px-4 min-h-full">
              {/* Empty states */}
              {rounds.length === 0 && chatState.streamStatus === "idle" && (
                <div className="flex items-center justify-center flex-1 text-sm text-[var(--editor-text-muted)]">{t.chat.inputInstructions}</div>
              )}
              {rounds.length === 0 && chatState.streamStatus === "connecting" && (
                <div className="flex items-center justify-center flex-1"><div className="flex flex-col items-center gap-3"><Loader2 size={24} className="animate-spin text-[var(--editor-accent)]" /><span className="text-xs text-[var(--editor-text-muted)]">{t.chat.connecting}</span></div></div>
              )}

              {/* Render events in order */}
              {activeRound && activeRound.events
                .filter(e => e.event !== "tool" || (e.data.phase === "result" && e.data.success !== false))
                .map((entry, i) => {
                  const key = `${entry.event}-${i}-${entry.timestamp}`;

                  // ── THINKING ──
                  if (entry.event === "thinking") {
                    return (
                      <div key={key} className="mb-2">
                        <button
                          type="button"
                          onClick={() => setThinkingExpanded(!thinkingExpanded)}
                          className="flex items-center gap-1.5 text-[11px] font-medium text-[var(--editor-text-muted)] uppercase tracking-wider hover:text-[var(--editor-text)] transition-colors"
                        >
                          {thinkingExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                          <Lightbulb size={12} className="text-[var(--editor-accent)]" />{t.chat.thinkingProcess}
                        </button>
                        {thinkingExpanded && (
                          <div className="mt-1.5 border-l-2 border-[var(--editor-border)] pl-3 ml-1 max-h-[120px] overflow-y-auto thin-scrollbar">
                            <p className="text-xs text-[var(--editor-text-muted)] leading-relaxed whitespace-pre-wrap font-mono">{entry.data.content}</p>
                          </div>
                        )}
                      </div>
                    );
                  }

                  // ── TEXT ──
                  if (entry.event === "text") {
                    return (
                      <div key={key} className="text-sm leading-relaxed mb-2 text-[var(--editor-text)]">
                        <InlineMarkdown text={entry.data.content} />
                      </div>
                    );
                  }

                  // ── TOOL ──
                  if (entry.event === "tool") {
                    const d = entry.data;
                    return (
                      <div key={key} className="mb-2 flex flex-wrap gap-1">
                        <ToolPill
                          name={d.name}
                          status="done"
                          success={d.success}
                          summary={d.summary}
                          durationMs={d.duration_ms}
                          t={t}
                        />
                      </div>
                    );
                  }

                  // ── TODO ──
                  if (entry.event === "todo_write") {
                    return (
                      <div key={key} className="mb-2">
                        <TodoInline todos={entry.data.todos || []} />
                      </div>
                    );
                  }

                  // ── Hook warnings ──
                  if (entry.event === "hook:warn") {
                    const firstLine = (entry.data.message || "").split("\n")[0];
                    return (
                      <div key={key} className="mb-1 px-2 py-0.5 text-[11px] text-gray-500 italic">
                        {firstLine}
                      </div>
                    );
                  }

                  return null;
                })}

              {/* Streaming cursor */}
              {isProcessing && (
                <div className="flex items-center gap-2 text-xs text-[var(--editor-text-muted)] pb-1">
                  <span className="inline-block w-1.5 h-4 bg-[var(--editor-accent)] animate-pulse rounded-full align-middle" />
                </div>
              )}

              {/* Error */}
              {chatState.error && (
                <div className="rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-xs mb-2">
                  <p className="text-red-400 font-medium">{chatState.error.message}</p>
                  {chatState.error.recoverable && <button type="button" onClick={onRetry} className="mt-1 text-[11px] font-medium text-red-400 hover:text-red-300 underline">{t.chat.retry}</button>}
                </div>
              )}

              {/* Done */}
              {chatState.streamStatus === "done" && chatState.doneMeta && (
                <div className="flex items-center gap-2 text-xs text-[var(--editor-text-muted)] py-2">
                  <CheckCircle2 size={12} className="text-green-400" />
                  <span>{t.chat.reasoningComplete}{chatState.doneMeta.totalRounds}{t.chat.rounds}</span>
                </div>
              )}

              <div className="min-h-[8px] shrink-0" />
            </div>
          </div>

          {/* ── Bottom bar ── */}
          <div className="shrink-0 px-3 pb-3 pt-1 flex flex-col gap-2">
            {isProcessing && (
              <button type="button" onClick={onStop} className="flex items-center justify-center gap-2 w-full py-2 rounded-xl text-xs font-medium text-red-400 hover:bg-red-500/10 border border-red-400/20 transition-colors">
                <StopCircle size={13} />{t.chat.stopGenerating}
              </button>
            )}
            {chatState.streamStatus === "error" && chatState.error?.recoverable && (
              <button type="button" onClick={onRetry} className="flex items-center justify-center gap-2 w-full py-2 rounded-xl text-xs font-medium text-[var(--editor-accent)] hover:bg-[var(--editor-accent-soft)] border border-[var(--editor-accent)]/20 transition-colors">
                {t.chat.retry}
              </button>
            )}

            <AgentLog
              rounds={rounds}
              activeRoundIdx={activeRoundIdx}
              onSelectRound={(idx) => setActiveRoundIdx(idx)}
              streamStatus={chatState.streamStatus}
            />
          </div>
        </div>
      </BreathingBorder>
    </motion.div>
  );
}
