import type { translations } from "../i18n/translations";

type Translations = typeof translations["zh-CN"];

export function getToolDisplayName(name: string, t: Translations): string {
  // Map legacy tool names to translation keys
  const keyMap: Record<string, string> = {
    todo_write: "update_todo",
  };
  const key = keyMap[name] ?? name;
  return t.toolNames[key] ?? name;
}
