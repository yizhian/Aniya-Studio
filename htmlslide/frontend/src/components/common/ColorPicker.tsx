import React from "react";

/**
 * Shared color picker UI: a colored swatch with a transparent <input type="color"> overlay.
 * Used by both the Inspector panel and the Toolbar controls.
 */
export interface ColorPickerProps {
  /** Current color value (hex, rgb, or named color) */
  color: string;
  /** Called when the user picks a new color */
  onChange: (color: string) => void;
  /** Optional disabled state */
  disabled?: boolean;
  /** Optional CSS class name for the wrapper */
  className?: string;
  /** Optional title/tooltip text */
  title?: string;
}

export function ColorPicker({
  color,
  onChange,
  disabled = false,
  className = "",
  title,
}: ColorPickerProps) {
  return (
    <div
      className={`relative overflow-hidden rounded-lg border border-[var(--editor-border)] hover:ring-2 hover:ring-[var(--editor-accent)]/40 transition-all disabled:opacity-40 disabled:cursor-not-allowed ${className}`}
      style={{ backgroundColor: color }}
      title={title}
    >
      <input
        type="color"
        value={color}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
        className="absolute inset-0 w-full h-full opacity-0 cursor-pointer disabled:cursor-not-allowed"
      />
    </div>
  );
}
