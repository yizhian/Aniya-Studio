import {
  AlignCenter,
  AlignLeft,
  AlignRight,
  Baseline,
  Bold,
  Italic,
  PaintBucket,
  Palette,
  Type,
} from "lucide-react";
import { EditorTextAlign } from "../../../models/editor";
import type { ToolbarStyleControl } from "./types";

export const EDITOR_TOOLBAR_STYLE_CONTROLS: readonly ToolbarStyleControl[] = [
  {
    id: "font-family",
    type: "fontFamily",
    styleKey: "font-family",
    readField: "fontFamily",
    icon: Type,
  },
  {
    id: "font-size",
    type: "stylePx",
    styleKey: "font-size",
    readField: "fontSize",
    icon: Baseline,
  },
  {
    id: "color",
    type: "styleColor",
    styleKey: "color",
    readField: "color",
    icon: PaintBucket,
  },
  {
    id: "toggle-bold",
    type: "toggleBold",
    icon: Bold,
  },
  {
    id: "toggle-italic",
    type: "toggleItalic",
    icon: Italic,
  },
  {
    id: "background-color",
    type: "styleColor",
    styleKey: "background-color",
    readField: "backgroundColor",
    icon: Palette,
  },
  {
    id: "text-align",
    type: "align",
    styleKey: "text-align",
    readField: "textAlign",
    options: [
      { value: EditorTextAlign.Left, icon: AlignLeft },
      { value: EditorTextAlign.Center, icon: AlignCenter },
      { value: EditorTextAlign.Right, icon: AlignRight },
    ],
  },
] as const;
