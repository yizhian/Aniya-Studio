// Backward-compatible re-exports from split modules.
// Prefer importing from the individual modules directly.

export type { ControlledStyleKey, StylePatch } from "./editor/styleOperations";
export {
  getFontFamilyOptions,
  STYLE_CONSTRAINTS,
  updateSelectedStyles,
  setSelectedStyle,
  getStylePxBounds,
  setFontFamily,
  isBoldWeight,
  toggleBold,
  toggleItalic,
  getSelectedStyles,
} from "./editor/styleOperations";

export {
  getImageBlockHtml,
  getVideoBlockHtml,
  finalizeBlockDrop,
  getSelectedImageData,
  replaceSelectedImage,
} from "./editor/componentOperations";

export {
  undo,
  redo,
  canUndo,
  canRedo,
  setHtmlAndCss,
  serializePublishHtml,
  resetCssBaseline,
  importHtmlDocument,
} from "./editor/exportSerialization";

export { detectComponentType } from "./editor/componentDetection";
export type { ComponentType } from "./editor/componentDetection";

export {
  postSaveVersion,
  postRestoreVersion,
} from "./editor/versionApi";
