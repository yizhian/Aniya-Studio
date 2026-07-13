import { useCallback, useState } from "react";
import type { Editor } from "grapesjs";
import * as api from "../services/editorApi";
import { useLocale } from "../context/LocaleContext";
import { logError } from "../utils/errorLogger";

export function useEditorExport(
  editor: Editor | null,
  persistedHtml: string,
  routeProjectId: string | undefined,
) {
  const { t } = useLocale();
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const openPreview = useCallback(() => {
    if (!editor || !persistedHtml) {
      setPreviewError(t.errors.editorNotInitializedPreview);
      setPreviewUrl(null);
      return;
    }
    try {
      setPreviewError(null);
      if (previewUrl) URL.revokeObjectURL(previewUrl);
      let html = api.serializePublishHtml(persistedHtml, editor, { skipFontInjection: true });
      if (routeProjectId) {
        html = html.replace('<head>', `<head><base href="/api/v1/projects/${encodeURIComponent(routeProjectId)}/">`);
      }
      const blob = new Blob([html], { type: "text/html;charset=utf-8" });
      setPreviewUrl(URL.createObjectURL(blob));
    } catch (error) {
      setPreviewError(error instanceof Error ? error.message : t.errors.generatePreviewFailed);
      setPreviewUrl(null);
    }
  }, [editor, persistedHtml, previewUrl, routeProjectId, t]);

  const handleExport = useCallback(() => {
    if (!editor || !persistedHtml) return;
    const html = api.serializePublishHtml(persistedHtml, editor);
    const blob = new Blob([html], { type: "text/html;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "export.html";
    a.click();
    URL.revokeObjectURL(url);
  }, [editor, persistedHtml]);

  const downloadBinaryExport = useCallback(async (format: "pdf" | "pptx") => {
    if (!routeProjectId) return;
    try {
      const response = await fetch(
        `/api/v1/projects/${encodeURIComponent(routeProjectId)}/export/${format}`,
      );
      if (!response.ok) throw new Error("Export failed");
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `export.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      logError(`handleExport${format.toUpperCase()}`, err);
    }
  }, [routeProjectId]);

  const handleExportPdf = useCallback(() => downloadBinaryExport("pdf"), [downloadBinaryExport]);
  const handleExportPptx = useCallback(() => downloadBinaryExport("pptx"), [downloadBinaryExport]);

  const handlePrecipitateSkill = useCallback(() => {
    if (!editor || !persistedHtml) return;
    const html = api.serializePublishHtml(persistedHtml, editor);
    return html;
  }, [editor, persistedHtml]);

  return {
    previewUrl, setPreviewUrl, previewError, setPreviewError,
    openPreview,
    handleExport, handleExportPdf, handleExportPptx,
    handlePrecipitateSkill,
  };
}
