import type { Editor } from "grapesjs";
import type { StylePatch } from "../../../../services/editorApi";
import * as api from "../../../../services/editorApi";
import { EditorTextAlign } from "../../../../models/editor";
import type { SelectedToolbarStyles, ToolbarAlignControl } from "../types";
import { controlDividerClass } from "../types";

interface Props {
  control: ToolbarAlignControl;
  editor: Editor | null;
  styles: SelectedToolbarStyles | null;
  disabled: boolean;
  index: number;
  onAfterAction: () => void;
}

export function AlignControl({ control, editor, styles, disabled, index, onAfterAction }: Props) {
  const current = (styles?.[control.readField] as string | undefined) ?? EditorTextAlign.Left;

  return (
    <div className={`flex items-center gap-1 pl-0 ${controlDividerClass(index)}`}>
      {control.options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          disabled={disabled}
          onClick={() => {
            if (!editor) return;
            const patch: StylePatch = { [control.styleKey]: opt.value };
            api.updateSelectedStyles(editor, patch);
            onAfterAction();
          }}
          className={`p-1.5 rounded-lg transition-colors disabled:opacity-40 ${
            current === opt.value
              ? "bg-[var(--editor-control-hover)] text-[var(--editor-text)]"
              : "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)]"
          }`}
        >
          <opt.icon size={16} />
        </button>
      ))}
    </div>
  );
}
