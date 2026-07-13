import type { Editor } from "grapesjs";
import * as api from "../../../../services/editorApi";
import type { SelectedToolbarStyles, ToolbarToggleItalicControl } from "../types";
import { controlDividerClass } from "../types";

interface Props {
  control: ToolbarToggleItalicControl;
  editor: Editor | null;
  styles: SelectedToolbarStyles | null;
  disabled: boolean;
  index: number;
  onAfterAction: () => void;
}

export function ToggleItalicControl({ control, editor, styles, disabled, index, onAfterAction }: Props) {
  const active = styles?.fontStyle === "italic";

  return (
    <div className={`flex items-center ${controlDividerClass(index)}`}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => {
          if (!editor) return;
          api.toggleItalic(editor);
          onAfterAction();
        }}
        className={`p-1.5 rounded-lg transition-colors disabled:opacity-40 ${
          active
            ? "bg-[var(--editor-control-hover)] text-[var(--editor-text)]"
            : "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)]"
        }`}
        title="Italic"
      >
        <control.icon size={16} />
      </button>
    </div>
  );
}
