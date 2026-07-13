import { useState } from "react";
import { CheckCircle2, ChevronDown, Circle, ListTodo } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";

interface Props {
  todos: Array<{ content: string; status: string }>;
}

export function TodoInline({ todos }: Props) {
  const { t } = useLocale();
  const [open, setOpen] = useState(true);

  return (
    <div className="rounded-xl border border-[var(--editor-border)] bg-[var(--editor-control)]/50 backdrop-blur-sm overflow-hidden my-2">
      <button type="button" onClick={() => setOpen(!open)} className="w-full flex items-center gap-2 px-3 py-1.5 text-[11px] font-medium text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors">
        <ListTodo size={12} className="text-[var(--editor-accent)] shrink-0" />
        <span>{t.chat.taskList}</span>
        <span className="tabular-nums opacity-60">{todos.filter((t) => t.status === "completed").length}/{todos.length}</span>
        <ChevronDown size={11} className={`ml-auto transition-transform ${open ? "rotate-180" : ""}`} />
      </button>
      {open && (
        <div className="px-3 pb-2 flex flex-col gap-0.5">
          {todos.map((t, i) => (
            <div key={i} className="flex items-start gap-1.5 text-[11px]">
              <div className="mt-0.5 shrink-0">
                {t.status === "completed" ? <CheckCircle2 size={11} className="text-green-400" />
                : t.status === "in_progress" ? <div className="w-2.5 h-2.5 rounded-full border-2 border-[var(--editor-accent)] bg-[var(--editor-accent)]/20 animate-pulse" />
                : <Circle size={11} className="text-[var(--editor-text-muted)]" />}
              </div>
              <span className={`leading-relaxed ${t.status === "completed" ? "text-[var(--editor-text-muted)] line-through" : t.status === "in_progress" ? "text-[var(--editor-text)] font-medium" : "text-[var(--editor-text-muted)]"}`}>
                {t.content}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
