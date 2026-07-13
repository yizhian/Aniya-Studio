import {
  Type,
  Baseline,
  AlignLeft,
  AlignCenter,
  AlignRight,
  Palette,
  Layout,
  MoveDiagonal,
  Eye,
  Sparkles,
  ArrowRightLeft,
  ArrowUpDown,
  Columns2,
  Braces,
  Maximize2,
  Square,
  Rows,
  Pilcrow,
  LetterText,
  BetweenVerticalStart,
  WrapText,
  PaintBucket,
  ScanEye,
  Radius,
  AlignStartVertical,
  AlignCenterVertical,
  AlignEndVertical,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

// ─── Inspector control types ───

type InspectorControlType =
  | "fontFamily"
  | "number"
  | "color"
  | "segmented"
  | "select";

export interface InspectorProperty {
  id: string;
  label: string;
  type: InspectorControlType;
  styleKey: string;
  icon?: LucideIcon;
  constraints?: {
    min?: number;
    max?: number;
    step?: number;
    unit?: string;
  };
  options?: ReadonlyArray<{ label: string; value: string; icon?: LucideIcon }>;
}

export interface InspectorSection {
  id: string;
  label: string;
  icon: LucideIcon;
  properties: InspectorProperty[];
}

// ─── Options ───

const FONT_WEIGHT_OPTIONS = [
  { label: "300", value: "300" },
  { label: "400", value: "400" },
  { label: "500", value: "500" },
  { label: "600", value: "600" },
  { label: "700", value: "700" },
] as const;

const TEXT_ALIGN_OPTIONS = [
  { label: "", value: "left", icon: AlignLeft },
  { label: "", value: "center", icon: AlignCenter },
  { label: "", value: "right", icon: AlignRight },
] as const;

const DISPLAY_OPTIONS = [
  { label: "Block", value: "block" },
  { label: "Flex", value: "flex" },
  { label: "Grid", value: "grid" },
  { label: "None", value: "none" },
] as const;

const FLEX_DIRECTION_OPTIONS = [
  { label: "", value: "row", icon: ArrowRightLeft },
  { label: "", value: "column", icon: ArrowUpDown },
] as const;

const JUSTIFY_OPTIONS = [
  { label: "", value: "flex-start", icon: AlignStartVertical },
  { label: "", value: "center", icon: AlignCenterVertical },
  { label: "", value: "flex-end", icon: AlignEndVertical },
  { label: "", value: "space-between", icon: BetweenVerticalStart },
] as const;

const ALIGN_ITEMS_OPTIONS = [
  { label: "", value: "flex-start", icon: AlignStartVertical },
  { label: "", value: "center", icon: AlignCenterVertical },
  { label: "", value: "flex-end", icon: AlignEndVertical },
] as const;

// ─── Feature map ───

export const INSPECTOR_SECTIONS: readonly InspectorSection[] = [
  {
    id: "typography",
    label: "Typography",
    icon: Type,
    properties: [
      {
        id: "font-family",
        label: "Font",
        type: "fontFamily",
        styleKey: "font-family",
        icon: LetterText,
      },
      {
        id: "font-size",
        label: "Size",
        type: "number",
        styleKey: "font-size",
        icon: Baseline,
        constraints: { min: 8, max: 200, step: 1, unit: "px" },
      },
      {
        id: "font-weight",
        label: "Weight",
        type: "segmented",
        styleKey: "font-weight",
        icon: Type,
        options: FONT_WEIGHT_OPTIONS,
      },
      {
        id: "color",
        label: "Color",
        type: "color",
        styleKey: "color",
        icon: Palette,
      },
      {
        id: "text-align",
        label: "Align",
        type: "segmented",
        styleKey: "text-align",
        icon: AlignLeft,
        options: TEXT_ALIGN_OPTIONS,
      },
      {
        id: "line-height",
        label: "Line H",
        type: "number",
        styleKey: "line-height",
        icon: WrapText,
        constraints: { min: 8, max: 400, step: 1, unit: "px" },
      },
      {
        id: "letter-spacing",
        label: "Letter",
        type: "number",
        styleKey: "letter-spacing",
        icon: BetweenVerticalStart,
        constraints: { min: -20, max: 100, step: 1, unit: "px" },
      },
    ],
  },
  {
    id: "layout",
    label: "Layout",
    icon: Layout,
    properties: [
      {
        id: "display",
        label: "Display",
        type: "segmented",
        styleKey: "display",
        icon: Columns2,
        options: DISPLAY_OPTIONS,
      },
      {
        id: "flex-direction",
        label: "Direction",
        type: "segmented",
        styleKey: "flex-direction",
        icon: ArrowRightLeft,
        options: FLEX_DIRECTION_OPTIONS,
      },
      {
        id: "justify-content",
        label: "Justify",
        type: "segmented",
        styleKey: "justify-content",
        icon: Rows,
        options: JUSTIFY_OPTIONS,
      },
      {
        id: "align-items",
        label: "Align",
        type: "segmented",
        styleKey: "align-items",
        icon: AlignLeft,
        options: ALIGN_ITEMS_OPTIONS,
      },
      {
        id: "gap",
        label: "Gap",
        type: "number",
        styleKey: "gap",
        icon: Braces,
        constraints: { min: 0, max: 500, step: 1, unit: "px" },
      },
      {
        id: "width",
        label: "Width",
        type: "number",
        styleKey: "width",
        icon: Maximize2,
        constraints: { min: 0, max: 9999, step: 1, unit: "px" },
      },
      {
        id: "height",
        label: "Height",
        type: "number",
        styleKey: "height",
        icon: Maximize2,
        constraints: { min: 0, max: 9999, step: 1, unit: "px" },
      },
    ],
  },
  {
    id: "spacing",
    label: "Spacing",
    icon: MoveDiagonal,
    properties: [
      {
        id: "padding",
        label: "Padding",
        type: "number",
        styleKey: "padding",
        icon: Square,
        constraints: { min: 0, max: 500, step: 1, unit: "px" },
      },
      {
        id: "margin",
        label: "Margin",
        type: "number",
        styleKey: "margin",
        icon: Square,
        constraints: { min: -500, max: 500, step: 1, unit: "px" },
      },
      {
        id: "border-radius",
        label: "Radius",
        type: "number",
        styleKey: "border-radius",
        icon: Radius,
        constraints: { min: 0, max: 500, step: 1, unit: "px" },
      },
    ],
  },
  {
    id: "effects",
    label: "Effects",
    icon: Sparkles,
    properties: [
      {
        id: "background-color",
        label: "BG Color",
        type: "color",
        styleKey: "background-color",
        icon: PaintBucket,
      },
      {
        id: "opacity",
        label: "Opacity",
        type: "number",
        styleKey: "opacity",
        icon: ScanEye,
        constraints: { min: 0, max: 1, step: 0.05 },
      },
      {
        id: "border-width",
        label: "Border",
        type: "number",
        styleKey: "border-width",
        icon: Square,
        constraints: { min: 0, max: 50, step: 1, unit: "px" },
      },
      {
        id: "border-color",
        label: "Bd Color",
        type: "color",
        styleKey: "border-color",
        icon: Palette,
      },
    ],
  },
];
