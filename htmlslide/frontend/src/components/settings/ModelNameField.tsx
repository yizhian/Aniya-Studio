import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { ChevronDown } from "lucide-react";

interface Props {
  value: string;
  onChange: (v: string) => void;
  models: string[];
  placeholder: string;
  hint: string | null;
  onFetch: () => void;
  fetching: boolean;
  fetchLabel: string;
  loadingLabel: string;
  pickLabel: string;
  disabled: boolean;
}

export function ModelNameField({
  value,
  onChange,
  models,
  placeholder,
  hint,
  onFetch,
  fetching,
  fetchLabel,
  loadingLabel,
  pickLabel,
  disabled,
}: Props) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  const canPick = models.length > 0;

  return (
    <div className="flex flex-col gap-1.5" ref={ref}>
      <div className="flex items-center justify-between gap-2">
        <label className="text-[11px] font-semibold uppercase tracking-wider text-[var(--editor-text-muted)]">
          {pickLabel}
        </label>
        <button
          type="button"
          onClick={onFetch}
          disabled={fetching || disabled}
          className="text-[10px] font-semibold uppercase tracking-wider text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] disabled:opacity-40 transition-colors"
        >
          {fetching ? loadingLabel : fetchLabel}
        </button>
      </div>

      <div className="relative">
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onFocus={() => canPick && setOpen(true)}
          placeholder={placeholder}
          className="w-full px-3 py-2 pr-9 rounded-xl bg-[var(--editor-control)]/50 border border-[var(--editor-border)] text-sm text-[var(--editor-text)] placeholder:text-[var(--editor-text-muted)] focus:outline-none focus:border-[var(--editor-text)]/40 transition-colors"
        />
        {canPick && (
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors"
          >
            <ChevronDown
              size={14}
              className={`transition-transform ${open ? "rotate-180" : ""}`}
            />
          </button>
        )}
      </div>

      <AnimatePresence>
        {open && canPick && (
          <motion.div
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -2 }}
            transition={{ duration: 0.12 }}
            className="rounded-xl border border-[var(--editor-border)] bg-[var(--editor-bg)] overflow-hidden"
          >
            <div className="max-h-[160px] overflow-y-auto thin-scrollbar p-1 flex flex-col gap-0.5">
              {models.map((model) => (
                <button
                  key={model}
                  type="button"
                  onClick={() => {
                    onChange(model);
                    setOpen(false);
                  }}
                  className={`w-full text-left px-2.5 py-2 rounded-lg text-xs font-medium truncate transition-colors ${
                    value === model
                      ? "bg-[var(--editor-text)] text-[var(--editor-bg)]"
                      : "text-[var(--editor-text-muted)] hover:bg-[var(--editor-control)]/60 hover:text-[var(--editor-text)]"
                  }`}
                >
                  {model}
                </button>
              ))}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {hint && (
        <p className="text-[10px] text-[var(--editor-text-muted)] leading-relaxed">
          {hint}
        </p>
      )}
    </div>
  );
}
