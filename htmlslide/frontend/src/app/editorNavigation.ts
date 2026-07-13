import type { ParsedFile } from "../services/upload";

/** React Router location.state 中携带的 HTML 导入载荷 */
export const EDITOR_IMPORT_HTML_STATE_KEY = "importedHtml" as const;
export const EDITOR_PROJECT_ID_KEY = "projectId" as const;
export const EDITOR_PROMPT_KEY = "prompt" as const;
export const EDITOR_ATTACHMENTS_KEY = "attachments" as const;
export const EDITOR_UPLOADED_FILES_KEY = "uploadedFiles" as const;

export type EditorLocationState = {
  [EDITOR_IMPORT_HTML_STATE_KEY]?: string;
  [EDITOR_PROJECT_ID_KEY]?: string;
  [EDITOR_PROMPT_KEY]?: string;
  [EDITOR_ATTACHMENTS_KEY]?: string[];
  [EDITOR_UPLOADED_FILES_KEY]?: ParsedFile[];
};
