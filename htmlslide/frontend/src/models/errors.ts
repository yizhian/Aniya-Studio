/** Domain-specific error types for the editor. */
export type EditorErrorType =
  | 'save'
  | 'export_pdf'
  | 'export_pptx'
  | 'chat'
  | 'bootstrap'
  | 'image_replace'
  | 'version_restore'
  | 'upload'
  | 'canvas_update'
  | 'import_html';

/** Structured error with type discrimination for differentiated UI display. */
export interface EditorError {
  type: EditorErrorType;
  message: string;
}

/** Error display variant determines visual styling. */
export type ErrorVariant = 'danger' | 'warning' | 'info';

/** Maps error type to display variant. */
export function errorVariant(type: EditorErrorType): ErrorVariant {
  switch (type) {
    case 'save':
    case 'export_pdf':
    case 'export_pptx':
      return 'warning';
    default:
      return 'danger';
  }
}

/** CSS classes per error variant. */
export const ERROR_VARIANT_CLASSES: Record<ErrorVariant, string> = {
  danger: 'border-red-400/30 bg-red-500/10 text-[var(--editor-danger)]',
  warning: 'border-orange-400/30 bg-orange-500/10 text-orange-400',
  info: 'border-blue-400/30 bg-blue-500/10 text-blue-400',
};
