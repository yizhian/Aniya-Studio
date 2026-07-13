import { useCallback, useLayoutEffect, useRef } from "react";

interface Options {
  minRows?: number;
  maxRows?: number;
  lineHeightPx?: number;
}

/**
 * Auto-grow a textarea up to maxRows, then scroll internally.
 */
export function useAutoResizeTextarea(
  value: string,
  { minRows = 3, maxRows = 8, lineHeightPx = 28 }: Options = {},
) {
  const ref = useRef<HTMLTextAreaElement>(null);

  const resize = useCallback(() => {
    const el = ref.current;
    if (!el) return;

    el.style.height = "auto";
    const minHeight = minRows * lineHeightPx;
    const maxHeight = maxRows * lineHeightPx;
    const next = Math.min(Math.max(el.scrollHeight, minHeight), maxHeight);
    el.style.height = `${next}px`;
    el.style.overflowY = el.scrollHeight > maxHeight ? "auto" : "hidden";
  }, [lineHeightPx, maxRows, minRows]);

  useLayoutEffect(() => {
    resize();
  }, [value, resize]);

  return { ref, onInput: resize };
}
