import { useRef, useEffect, useState } from "react";
import type { Editor } from "grapesjs";
import { ChevronDown } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import * as api from "../../../../services/editorApi";
import type { SelectedToolbarStyles, ToolbarFontFamilyControl } from "../types";
import { controlDividerClass } from "../types";

interface Props {
  control: ToolbarFontFamilyControl;
  editor: Editor | null;
  styles: SelectedToolbarStyles | null;
  disabled: boolean;
  index: number;
  onAfterAction: () => void;
}

export function FontFamilyControl({ control, editor, styles, disabled, index, onAfterAction }: Props) {
  const [open, setOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  const fontOptions = api.getFontFamilyOptions(editor);
  const current =
    typeof styles?.[control.readField] === "string"
      ? (styles[control.readField] as string)
      : fontOptions[0].value;
  const currentLabel =
    fontOptions.find((o) => o.value === current)?.label ?? fontOptions[0].label;

  return (
    <div className={`flex items-center gap-2 ${controlDividerClass(index)}`} ref={dropdownRef}>
      <control.icon size={16} className="text-[var(--editor-text-muted)] shrink-0" />
      <div className="relative">
        <button
          type="button"
          disabled={disabled}
          onClick={() => setOpen((v) => !v)}
          className="h-8 rounded-lg border border-[var(--editor-border)] bg-[var(--editor-control)] px-2.5 flex items-center gap-1.5 text-xs text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors disabled:opacity-40"
          style={{ fontFamily: current }}
        >
          <span className="truncate max-w-[100px]">{currentLabel}</span>
          <ChevronDown size={12} className={`text-[var(--editor-text-muted)] transition-transform ${open ? "rotate-180" : ""}`} />
        </button>
        <AnimatePresence>
          {open && !disabled && (
            <motion.div
              initial={{ opacity: 0, y: -4, scale: 0.96 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -4, scale: 0.96 }}
              transition={{ duration: 0.14, ease: "easeOut" }}
              className="absolute top-10 left-0 z-[70] w-48 rounded-lg border border-[var(--editor-border)] bg-[var(--editor-surface)] shadow-sm overflow-hidden"
            >
              {fontOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => {
                    if (!editor) return;
                    api.setFontFamily(editor, option.value);
                    setOpen(false);
                    onAfterAction();
                  }}
                  className={`w-full text-left px-3 py-2 text-xs hover:bg-[var(--editor-control-hover)] transition-colors flex items-center justify-between ${
                    option.value === current
                      ? "text-[var(--editor-text)] bg-[var(--editor-control)]"
                      : "text-[var(--editor-text-muted)]"
                  }`}
                >
                  <span style={{ fontFamily: option.value }}>{option.label}</span>
                  {option.value === current && (
                    <span className="w-1.5 h-1.5 rounded-full bg-[var(--editor-accent)]" />
                  )}
                </button>
              ))}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}
