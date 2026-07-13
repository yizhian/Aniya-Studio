import React from "react";

/**
 * Shared font family picker UI: a styled <select> element for font selection.
 * Used by both the Inspector panel and the Toolbar controls.
 */
export interface FontPickerProps {
  /** Currently selected font family */
  value: string;
  /** Available font family options */
  options: Array<{ label: string; value: string }>;
  /** Called when the user selects a new font */
  onChange: (value: string) => void;
  /** Optional disabled state */
  disabled?: boolean;
  /** Optional CSS class name */
  className?: string;
}

export function FontPicker({
  value,
  options,
  onChange,
  disabled = false,
  className = "",
}: FontPickerProps) {
  return (
    <select
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value)}
      className={`w-full px-3 py-1.5 rounded-lg bg-[var(--editor-bg)] border border-[var(--editor-border)] text-sm text-[var(--editor-text)] focus:outline-none focus:border-[var(--editor-accent)] transition-colors disabled:opacity-50 appearance-none cursor-pointer ${className}`}
    >
      {options.map((opt) => (
        <option key={opt.value} value={opt.value}>
          {opt.label}
        </option>
      ))}
    </select>
  );
}
