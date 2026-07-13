import React, { useCallback, useEffect, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import type { Editor } from "grapesjs";
import { getFontFamilyOptions } from "../../../../services/editorApi";
import type { InspectorProperty } from "../../../../config/editorFeatureMap";

type Props = {
  editor: Editor | null;
  property: InspectorProperty;
  value: string;
  onChange: (value: string) => void;
};

export function FontFamilyControl({ editor, value, onChange }: Props) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const options = getFontFamilyOptions(editor);
  const current = options.find((o) => o.value === value) ?? options[0];

  const handleClickOutside = useCallback((e: MouseEvent) => {
    if (ref.current && !ref.current.contains(e.target as Node)) {
      setOpen(false);
    }
  }, []);

  useEffect(() => {
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [handleClickOutside]);

  return (
    <div ref={ref} className="relative" style={{ width: "100%" }}>
      <button
        type="button"
        className="inspector-font-button"
        style={{ width: "100%" }}
        onClick={() => setOpen((p) => !p)}
      >
        <span className="truncate">{current?.label ?? "Select"}</span>
        <ChevronDown
          size={12}
          className={`inspector-chevron ${open ? "open" : "closed"}`}
          style={{ color: "var(--inspector-text-muted)", flexShrink: 0 }}
        />
      </button>
      {open && (
        <div
          style={{
            position: "absolute",
            top: "100%",
            left: 0,
            right: 0,
            marginTop: 4,
            zIndex: 50,
            background: "var(--inspector-control-bg)",
            border: "1px solid var(--inspector-border)",
            borderRadius: 12,
            boxShadow: "0 4px 16px rgba(0,0,0,0.08)",
            overflow: "hidden",
            maxHeight: 240,
            overflowY: "auto",
          }}
        >
          {options.map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => {
                onChange(opt.value);
                setOpen(false);
              }}
              style={{
                display: "block",
                width: "100%",
                textAlign: "left",
                padding: "8px 12px",
                fontSize: 13,
                fontWeight: opt.value === value ? 600 : 400,
                color: "var(--inspector-text)",
                background:
                  opt.value === value
                    ? "var(--inspector-hover)"
                    : "transparent",
                border: "none",
                cursor: "pointer",
                fontFamily: opt.value,
              }}
            >
              {opt.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
