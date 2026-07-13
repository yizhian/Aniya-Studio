import { useEffect, useRef, useState } from "react";
import { History, Loader2, ChevronDown } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import type { TimelineEntry } from "../../models/chat";

interface Round {
  userMessage: TimelineEntry;
  events: TimelineEntry[];
}

interface Props {
  rounds: Round[];
  activeRoundIdx: number;
  onSelectRound: (idx: number) => void;
  streamStatus: string;
}

export function AgentLog({ rounds, activeRoundIdx, onSelectRound, streamStatus }: Props) {
  const { t } = useLocale();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); }
    if (open) document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  const isStreaming = streamStatus === "streaming" || streamStatus === "connecting";
  const currentIdx = activeRoundIdx >= 0 ? activeRoundIdx : rounds.length - 1;

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className={`flex items-center justify-between gap-2 w-full px-3 py-2 text-xs font-medium rounded-2xl transition-all ${
          open ? "bg-[var(--editor-control-hover)] text-[var(--editor-text)]" : "bg-[var(--editor-control)]/60 text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)]"
        } border border-[var(--editor-border)] backdrop-blur-sm`}
      >
        <div className="flex items-center gap-2">
          {isStreaming ? <Loader2 size={13} className="animate-spin text-[var(--editor-accent)]" /> : <History size={13} />}
          <span>{t.chat.chatHistory}</span>
          <span className="tabular-nums opacity-60">{rounds.length}</span>
          {isStreaming && <span className="w-1.5 h-1.5 rounded-full bg-[var(--editor-accent)] animate-pulse" />}
        </div>
        <ChevronDown size={12} className={`transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {open && (
        <div className="absolute bottom-full left-0 mb-2 w-[310px] bg-[var(--editor-surface)] border border-[var(--editor-border)] rounded-2xl shadow-lg backdrop-blur-md overflow-hidden z-50">
          <div className="flex flex-col px-1 py-1 max-h-[260px] overflow-y-auto thin-scrollbar">
            {rounds.length === 0 && (
              <p className="text-xs text-[var(--editor-text-muted)] text-center py-4">{t.chat.noChatHistory}</p>
            )}
            {rounds.map((round, i) => {
              const isActive = i === currentIdx;
              const msg = round.userMessage.data.content?.replace(/【DOM 元素选中】[\s\S]*?用户指令：/, "").trim();
              const toolCount = round.events.filter(e => e.event === "tool" && e.data.phase === "result").length;
              const textEvents = round.events.filter(e => e.event === "text");
              const hasContent = textEvents.length > 0 || toolCount > 0;

              return (
                <button
                  key={i}
                  type="button"
                  onClick={() => { onSelectRound(i); setOpen(false); }}
                  className={`text-left w-full px-3 py-2 rounded-xl transition-colors ${
                    isActive
                      ? "bg-[var(--editor-accent-soft)] border border-[var(--editor-accent)]/10"
                      : "hover:bg-[var(--editor-control)] border border-transparent"
                  }`}
                >
                  <p className="text-xs text-[var(--editor-text)] leading-relaxed line-clamp-2 mb-0.5">
                    {msg?.slice(0, 100) || t.chat.emptyMessage}
                  </p>
                  <div className="flex items-center gap-2 text-[10px] text-[var(--editor-text-muted)]">
                    {hasContent ? (
                      <>
                        <span>{textEvents.length}{t.chat.replies}</span>
                        {toolCount > 0 && <span>· {toolCount}{t.chat.toolCount}</span>}
                      </>
                    ) : isStreaming && i === rounds.length - 1 ? (
                      <span className="flex items-center gap-1 text-[var(--editor-accent)]"><Loader2 size={9} className="animate-spin" />{t.chat.processing}</span>
                    ) : (
                      <span>{t.chat.waitingResponse}</span>
                    )}
                    {isActive && <span className="ml-auto text-[var(--editor-accent)]">●</span>}
                  </div>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
