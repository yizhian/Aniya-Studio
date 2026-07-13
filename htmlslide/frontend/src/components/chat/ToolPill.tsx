import { useState } from "react";
import { CheckCircle2, ChevronDown, Loader2, XCircle } from "lucide-react";
import type { translations } from "../../i18n/translations";
import { getToolDisplayName } from "../../services/toolNameMap";

interface Props {
  name: string;
  status: "running" | "done";
  success?: boolean;
  summary?: string;
  durationMs?: number;
  t: typeof translations["zh-CN"];
}

export function ToolPill({ name, status, success, summary, durationMs, t }: Props) {
  const dname = getToolDisplayName(name, t);
  const [open, setOpen] = useState(false);

  return (
    <div className="inline-flex flex-col">
      <button
        type="button"
        onClick={() => summary && setOpen(!open)}
        className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] transition-all ${
          status === "running" ? "bg-[var(--editor-accent-soft)] border border-[var(--editor-accent)]/20 text-[var(--editor-accent)]"
          : success === false ? "bg-red-500/5 border border-red-400/20 text-red-400"
          : "bg-green-500/5 border border-green-400/20 text-green-400"
        } ${summary ? "cursor-pointer hover:opacity-80" : ""}`}
      >
        {status === "running" ? <Loader2 size={10} className="animate-spin shrink-0" />
        : success === false ? <XCircle size={10} className="shrink-0" />
        : <CheckCircle2 size={10} className="shrink-0" />}
        <span className="font-medium truncate max-w-[140px]">{dname}</span>
        {status === "running" && <span className="animate-pulse">…</span>}
        {status === "done" && durationMs !== undefined && durationMs > 0 && (
          <span className="opacity-60">{durationMs < 1000 ? `${durationMs}ms` : `${(durationMs / 1000).toFixed(1)}s`}</span>
        )}
        {summary && <ChevronDown size={9} className={`transition-transform shrink-0 ${open ? "rotate-180" : ""}`} />}
      </button>
      {open && summary && (
        <div className="mt-1 ml-1.5 pl-2 border-l-2 border-[var(--editor-border)]">
          <p className="text-[11px] text-[var(--editor-text-muted)] leading-relaxed line-clamp-3">{summary}</p>
        </div>
      )}
    </div>
  );
}
