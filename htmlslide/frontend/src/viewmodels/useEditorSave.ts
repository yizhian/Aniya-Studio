import { useCallback, useState } from "react";
import type { Editor } from "grapesjs";
import * as api from "../services/editorApi";
import { useLocale } from "../context/LocaleContext";
import { logError } from "../utils/errorLogger";
import { readDeckState } from "../services/deckAdapter";

export function useEditorSave(
  editor: Editor | null,
  routeProjectId: string | undefined,
  persistedHtml: string,
  captureCleanSnapshot: (ed: Editor | null) => void,
) {
  const { t } = useLocale();
  const [saveState, setSaveState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [versionRefreshKey, setVersionRefreshKey] = useState(0);

  const handleSaveAndReturnOk = useCallback(async (): Promise<boolean> => {
    if (!editor || !routeProjectId || !persistedHtml) return false;
    setSaveState("saving");
    try {
      const html = api.serializePublishHtml(persistedHtml, editor);
      const res = await api.postSaveVersion(routeProjectId, t.chat.userManualEdit, html);
      api.resetCssBaseline(editor);
      captureCleanSnapshot(editor);
      setSaveState("saved");
      setVersionRefreshKey((k) => k + 1);
      return true;
    } catch (err) {
      logError('handleSaveAndReturnOk', err);
      setSaveState("error");
      return false;
    } finally {
      setTimeout(() => setSaveState("idle"), 2000);
    }
  }, [editor, routeProjectId, persistedHtml, captureCleanSnapshot, t]);

  const handleSave = useCallback(() => {
    handleSaveAndReturnOk();
  }, [handleSaveAndReturnOk]);

  const handleActualRestore = useCallback(async (versionId: string): Promise<boolean> => {
    if (!editor || !routeProjectId) return false;
    try {
      const data = await api.postRestoreVersion(routeProjectId, versionId);
      api.importHtmlDocument(editor, data.html, t, routeProjectId);
      captureCleanSnapshot(editor);
      setVersionRefreshKey((k) => k + 1);
      return true;
    } catch (err) {
      logError('handleActualRestore', err);
      return false;
    }
  }, [editor, routeProjectId, captureCleanSnapshot, t]);

  return {
    saveState,
    versionRefreshKey,
    handleSaveAndReturnOk,
    handleSave,
    handleActualRestore,
  };
}
