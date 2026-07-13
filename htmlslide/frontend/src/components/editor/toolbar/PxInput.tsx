import React from "react";

export function PxInput({
  value,
  min,
  max,
  disabled,
  onChange,
}: {
  value: number;
  min: number;
  max: number;
  disabled: boolean;
  onChange: (v: number) => void;
}) {
  const [local, setLocal] = React.useState(String(value));
  const localRef = React.useRef(local);
  localRef.current = local;

  const prevValueRef = React.useRef(value);
  React.useEffect(() => {
    if (value !== prevValueRef.current) {
      prevValueRef.current = value;
      setLocal(String(value));
    }
  }, [value]);

  const commit = React.useCallback(() => {
    const raw = localRef.current;
    const parsed = parseInt(raw, 10);
    if (raw === "" || !Number.isFinite(parsed)) {
      setLocal(String(value));
      return;
    }
    let clamped = parsed;
    if (min !== undefined) clamped = Math.max(min, clamped);
    if (max !== undefined) clamped = Math.min(max, clamped);
    setLocal(String(clamped));
    if (clamped !== value) onChange(clamped);
  }, [value, min, max, onChange]);

  return (
    <input
      type="text"
      inputMode="numeric"
      value={local}
      disabled={disabled}
      onChange={(e) => setLocal(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          (e.target as HTMLInputElement).blur();
        }
        if (e.key === "Escape") {
          setLocal(String(value));
          (e.target as HTMLInputElement).blur();
        }
      }}
      className="w-12 bg-transparent text-[var(--editor-text)] text-sm focus:outline-none text-center disabled:opacity-40"
    />
  );
}
