import type { Editor } from "grapesjs";
import type { StylePatch } from "../../../../services/editorApi";
import * as api from "../../../../services/editorApi";
import type { SelectedToolbarStyles, ToolbarStyleColorControl } from "../types";
import { controlDividerClass } from "../types";
import { ColorPicker } from "../../../common/ColorPicker";

interface Props {
  control: ToolbarStyleColorControl;
  editor: Editor | null;
  styles: SelectedToolbarStyles | null;
  disabled: boolean;
  index: number;
  onAfterAction: () => void;
}

export function ColorSwatchControl({ control, editor, styles, disabled, index, onAfterAction }: Props) {
  const colorValue =
    typeof styles?.[control.readField] === "string"
      ? (styles[control.readField] as string)
      : control.styleKey === "background-color"
        ? "#000000"
        : "#ffffff";

  return (
    <div className={`flex items-center gap-2 ${controlDividerClass(index)}`}>
      <control.icon size={16} className="text-[var(--editor-text-muted)] shrink-0" />
      <ColorPicker
        color={colorValue}
        disabled={disabled}
        title={control.styleKey === "background-color" ? "Background Color" : "Font Color"}
        className="h-7 w-9"
        onChange={(color) => {
          if (!editor) return;
          const patch: StylePatch = { [control.styleKey]: color };
          api.updateSelectedStyles(editor, patch);
          onAfterAction();
        }}
      />
    </div>
  );
}
