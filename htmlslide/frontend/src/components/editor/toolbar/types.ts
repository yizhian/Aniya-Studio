import type { LucideIcon } from "lucide-react";
import type { ControlledStyleKey } from "../../../services/editorApi";
import * as api from "../../../services/editorApi";
import type { EditorTextAlignValue } from "../../../models/editor";

export type SelectedToolbarStyles = NonNullable<ReturnType<typeof api.getSelectedStyles>>;

export type ToolbarStylePxControl = {
  id: string;
  type: "stylePx";
  styleKey: ControlledStyleKey;
  readField: keyof SelectedToolbarStyles;
  icon: LucideIcon;
};

export type ToolbarStyleColorControl = {
  id: string;
  type: "styleColor";
  styleKey: ControlledStyleKey;
  readField: keyof SelectedToolbarStyles;
  icon: LucideIcon;
};

export type ToolbarFontFamilyControl = {
  id: string;
  type: "fontFamily";
  styleKey: "font-family";
  readField: keyof SelectedToolbarStyles;
  icon: LucideIcon;
};

export type ToolbarToggleBoldControl = {
  id: string;
  type: "toggleBold";
  icon: LucideIcon;
};

export type ToolbarToggleItalicControl = {
  id: string;
  type: "toggleItalic";
  icon: LucideIcon;
};

export type ToolbarAlignControl = {
  id: string;
  type: "align";
  styleKey: "text-align";
  readField: keyof SelectedToolbarStyles;
  options: ReadonlyArray<{ value: EditorTextAlignValue; icon: LucideIcon }>;
};

export type ToolbarStyleControl =
  | ToolbarStylePxControl
  | ToolbarStyleColorControl
  | ToolbarFontFamilyControl
  | ToolbarToggleBoldControl
  | ToolbarToggleItalicControl
  | ToolbarAlignControl;

export function controlDividerClass(index: number) {
  return index > 0 ? "border-l border-[var(--editor-border)] pl-3" : "";
}
