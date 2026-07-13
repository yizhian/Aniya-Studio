import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { APP_CONFIG } from "../app/config";
import { uploadToAgentgo, type ParsedFile, type UploadStatus } from "../services/upload";
import type { FileChipFile } from "../components/chat/FileChip";
import { fetchProjectUploads } from "../api/projects";

export function useEditorUpload(
  routeProjectId: string | undefined,
  onError?: (message: string) => void,
) {
  const appConfig = APP_CONFIG;
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [parsedFiles, setParsedFiles] = useState<ParsedFile[]>([]);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [uploadStatus, setUploadStatus] = useState<UploadStatus>("idle");
  const [parseElapsed, setParseElapsed] = useState(0);
  const parseTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [documentViewerOpen, setDocumentViewerOpen] = useState(false);

  const clearParseTimer = useCallback(() => {
    if (parseTimerRef.current) {
      clearInterval(parseTimerRef.current);
      parseTimerRef.current = null;
    }
  }, []);

  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;

  useEffect(() => {
    if (!routeProjectId) return;
    fetchProjectUploads(appConfig.interaction.agentgoBaseUrl, routeProjectId)
      .then((files) => {
        if (files.length > 0) setParsedFiles(files);
      })
      .catch(() => {});
  }, [routeProjectId, appConfig.interaction.agentgoBaseUrl]);

  const fileChips: FileChipFile[] = useMemo(() => {
    return files.map((f, i) => {
      const p = parsedFiles[i];
      return {
        name: f.name,
        type: p?.type,
        pages: p?.pages,
        charCount: p?.char_count,
        error: p?.error,
        status: uploadStatus === "error" ? "error" : uploadStatus === "done" ? "done" : uploadStatus,
      };
    });
  }, [files, parsedFiles, uploadStatus]);

  const handleUpload = useCallback(
    async (selectedFiles: FileList | null) => {
      if (!selectedFiles || selectedFiles.length === 0) return;
      const fileArray = Array.from(selectedFiles);
      setFiles(fileArray);
      setUploadStatus("idle");
      setUploadProgress(null);
      setParseElapsed(0);
      clearParseTimer();
      try {
        const result = await uploadToAgentgo(
          fileArray,
          appConfig.interaction.agentgoBaseUrl,
          (progress) => setUploadProgress(progress),
          (status) => {
            setUploadStatus(status);
            if (status === "parsing") {
              parseTimerRef.current = setInterval(() => {
                setParseElapsed((prev) => prev + 1);
              }, 1000);
            }
          },
          routeProjectId,
        );
        clearParseTimer();
        setParsedFiles(result.files);
        setUploadStatus("done");
      } catch (err) {
        clearParseTimer();
        setUploadStatus("error");
        onErrorRef.current?.(err instanceof Error ? err.message : "Upload failed");
      }
    },
    [appConfig.interaction.agentgoBaseUrl, routeProjectId, clearParseTimer],
  );

  return {
    files,
    setFiles,
    parsedFiles,
    setParsedFiles,
    uploadProgress,
    setUploadProgress,
    uploadStatus,
    setUploadStatus,
    parseElapsed,
    fileChips,
    fileInputRef,
    handleUpload,
    documentViewerOpen,
    setDocumentViewerOpen,
  };
}
