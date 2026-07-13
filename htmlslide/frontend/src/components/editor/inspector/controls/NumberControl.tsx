import React, { useCallback, useEffect, useRef, useState } from "react";
import type { InspectorProperty } from "../../../../config/editorFeatureMap";

type Props = {
  property: InspectorProperty;
  value: string;
  onChange: (value: string) => void;
};

export function NumberControl({ property, value, onChange }: Props) {
  const { constraints } = property;
  const min = constraints?.min;
  const max = constraints?.max;
  const step = constraints?.step ?? 1;
  const unit = constraints?.unit ?? "";

  // Strip unit from incoming value for display
  const stripUnit = useCallback(
    (v: string) => {
      if (unit && v.toLowerCase().endsWith(unit.toLowerCase())) {
        return v.slice(0, -unit.length).trim();
      }
      // Fallback: strip any non-numeric suffix
      return v.replace(/[^0-9.\-]/g, "");
    },
    [unit],
  );

  const displayValue = stripUnit(value);

  const [local, setLocal] = useState(displayValue);
  const localRef = useRef(local);
  localRef.current = local;

  // Sync when parent value changes
  const prevValueRef = useRef(value);
  useEffect(() => {
    if (value !== prevValueRef.current) {
      prevValueRef.current = value;
      setLocal(stripUnit(value));
    }
  }, [value, stripUnit]);

  const commit = useCallback(() => {
    const raw = localRef.current;
    if (raw === "" || raw === "-") {
      // Revert to last known good value
      setLocal(stripUnit(value));
      return;
    }
    const parsed = parseFloat(raw);
    if (!Number.isFinite(parsed)) {
      setLocal(stripUnit(value));
      return;
    }
    let clamped = parsed;
    if (min !== undefined) clamped = Math.max(min, clamped);
    if (max !== undefined) clamped = Math.min(max, clamped);
    if (step >= 1 && !Number.isInteger(clamped)) {
      clamped = Math.round(clamped);
    }
    const next = String(clamped);
    if (next !== stripUnit(value)) {
      onChange(next);
    }
  }, [value, min, max, step, stripUnit, onChange]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter") {
        e.preventDefault();
        (e.target as HTMLInputElement).blur();
      }
      if (e.key === "Escape") {
        setLocal(stripUnit(value));
        (e.target as HTMLInputElement).blur();
      }
      // Arrow up/down nudge
      if (e.key === "ArrowUp" || e.key === "ArrowDown") {
        e.preventDefault();
        const current = parseFloat(localRef.current) || 0;
        const delta = e.key === "ArrowUp" ? step : -step;
        const next = current + delta;
        let clamped = next;
        if (min !== undefined) clamped = Math.max(min, clamped);
        if (max !== undefined) clamped = Math.min(max, clamped);
        if (step >= 1) clamped = Math.round(clamped);
        onChange(String(clamped));
      }
    },
    [value, min, max, step, stripUnit, onChange],
  );

  return (
    <div style={{ position: "relative", display: "flex", alignItems: "center", width: "100%" }}>
      <input
        type="text"
        inputMode="numeric"
        className="inspector-number-input"
        value={local}
        onChange={(e) => setLocal(e.target.value)}
        onBlur={commit}
        onKeyDown={handleKeyDown}
        style={unit ? { paddingRight: 24 } : undefined}
      />
      {unit && (
        <span
          style={{
            fontSize: 11,
            fontWeight: 600,
            color: "var(--inspector-text-muted)",
            position: "absolute",
            right: 10,
            pointerEvents: "none",
          }}
        >
          {unit}
        </span>
      )}
    </div>
  );
}
