import React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";

type Props = {
  currentPage: number;
  pageCount: number;
  onPrev: () => void;
  onNext: () => void;
  className?: string;
};

export function PageControls({
  currentPage,
  pageCount,
  onPrev,
  onNext,
  className = "",
}: Props) {
  const { t } = useLocale();
  if (pageCount <= 1) return null;

  return (
    <div className={`pointer-events-none absolute left-1/2 z-[60] -translate-x-1/2 transition-[bottom] duration-500 ease-out ${className}`}>
      <div className="pointer-events-auto flex items-center gap-1 rounded-[50px] border border-[var(--editor-border)] bg-[var(--editor-surface)] p-1 shadow-sm">
        <button
          type="button"
          onClick={onPrev}
          disabled={currentPage === 0}
          className="flex items-center justify-center w-8 h-8 rounded-full text-[var(--editor-text-muted)] hover:bg-[var(--editor-control-hover)] hover:text-[var(--editor-text)] disabled:opacity-20 disabled:cursor-not-allowed transition-all"
          title={t.formatting.pagePrevious}
        >
          <ChevronLeft size={18} />
        </button>
        <span className="min-w-[4.5rem] text-center text-xs font-medium text-[var(--editor-text-muted)] font-mono tabular-nums tracking-wider select-none">
          {currentPage + 1} / {pageCount}
        </span>
        <button
          type="button"
          onClick={onNext}
          disabled={currentPage >= pageCount - 1}
          className="flex items-center justify-center w-8 h-8 rounded-full text-[var(--editor-text-muted)] hover:bg-[var(--editor-control-hover)] hover:text-[var(--editor-text)] disabled:opacity-20 disabled:cursor-not-allowed transition-all"
          title={t.formatting.pageNext}
        >
          <ChevronRight size={18} />
        </button>
      </div>
    </div>
  );
}
