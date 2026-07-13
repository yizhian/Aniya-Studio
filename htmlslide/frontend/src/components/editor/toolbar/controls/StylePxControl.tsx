import type { Editor } from "grapesjs";
import type { StylePatch } from "../../../../services/editorApi";
import * as api from "../../../../services/editorApi";
import type { SelectedToolbarStyles, ToolbarStylePxControl } from "../types";
import { controlDividerClass } from "../types";
import { PxInput } from "../PxInput";

interface Props {
  control: ToolbarStylePxControl;
  editor: Editor | null;
  styles: SelectedToolbarStyles | null;
  disabled: boolean;
  index: number;
  onAfterAction: () => void;
}

export function StylePxControl({ control, editor, styles, disabled, index, onAfterAction }: Props) {
  const { min, max } = api.getStylePxBounds(control.styleKey);
  const raw = styles?.[control.readField];
  const numValue: number =
    typeof raw === "number" ? raw
      : typeof raw === "string" ? (parseInt(raw, 10) || 0)
        : min;

  return (
    <div className={`flex items-center gap-2 ${controlDividerClass(index)}`}>
      <control.icon size={16} className="text-[var(--editor-text-muted)] shrink-0" />
      <PxInput
        value={numValue}
        min={min}
        max={max}
        disabled={disabled}
        onChange={(v) => {
          if (!editor) return;
          const patch: StylePatch = { [control.styleKey]: v };
          api.updateSelectedStyles(editor, patch);
          onAfterAction();
        }}
      />
    </div>
  );
}
