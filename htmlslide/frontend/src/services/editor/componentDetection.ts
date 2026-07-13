import type { Editor } from "grapesjs";

export type ComponentType = "text" | "image" | null;

export function detectComponentType(editor: Editor | null): ComponentType {
  if (!editor) return null;
  const selected = editor.getSelected();
  if (!selected) return null;
  const el = selected.getEl();
  if (!el) return null;
  if (el.tagName.toLowerCase() === "img") return "image";
  return "text";
}
