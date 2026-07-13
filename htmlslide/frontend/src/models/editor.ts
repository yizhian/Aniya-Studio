export const EditorTextAlign = {
  Left: "left",
  Center: "center",
  Right: "right",
} as const;

export type EditorTextAlignValue =
  (typeof EditorTextAlign)[keyof typeof EditorTextAlign];

export const EditorMode = {
  Direct: "direct",
  Design: "design",
} as const;

export type EditorModeValue = (typeof EditorMode)[keyof typeof EditorMode];

export const ThemeMode = {
  Dark: "dark",
  Light: "light",
} as const;

export type ThemeModeValue = (typeof ThemeMode)[keyof typeof ThemeMode];

export interface PageState {
  id: string;
  title: string;
  html: string;
  css: string;
}

export interface SelectedComponentContext {
  id: string;
  tagName: string;
  label: string;
  html: string;
  styles: Record<string, unknown>;
  path: string;
}


export interface SaveVersionResponse {
  project_id: string;
  version_id: string;
  version_tag: string;
  title: string;
  html: string;
  css: string;
  created_at: string;
}

export interface VersionRestoreResponse {
  project_id: string;
  restored_to: string;
  new_version_id: string;
  new_version_tag: string;
  html: string;
  css: string;
}
