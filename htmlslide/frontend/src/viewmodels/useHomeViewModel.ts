import { useCallback, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { useLocale } from "../context/LocaleContext";
import { useSettings } from "../hooks/useSettings";
import { APP_CONFIG } from "../app/config";
import { EDITOR_ATTACHMENTS_KEY, EDITOR_PROMPT_KEY, EDITOR_UPLOADED_FILES_KEY } from "../app/editorNavigation";
import { uploadToAgentgo, type ParsedFile, type UploadResponse, type UploadStatus } from "../services/upload";
import type { FileChipFile } from "../components/chat/FileChip";
import { createProject, updateProject } from "../api/projects";

export function buildFileChips(files: File[], parsed: ParsedFile[], status: UploadStatus): FileChipFile[] {
  return files.map((f, i) => {
    const p = parsed[i];
    return {
      name: f.name,
      type: p?.type,
      pages: p?.pages,
      charCount: p?.char_count,
      error: p?.error,
      status: status === "error" ? "error" : status === "done" ? "done" : status,
    };
  });
}

export function useHomeViewModel() {
  const navigate = useNavigate();
  const { t } = useLocale();
  const { syncToAgent, getActiveConfig } = useSettings();
  const [prompt, setPrompt] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [parsedFiles, setParsedFiles] = useState<ParsedFile[]>([]);
  const [isGenerating, setIsGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [uploadStatus, setUploadStatus] = useState<UploadStatus>("idle");
  const [parseElapsed, setParseElapsed] = useState(0);
  const parseTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [projectId, setProjectId] = useState<string | null>(null);

  const isUploading = uploadStatus === "uploading" || uploadStatus === "parsing";
  const canGenerate = Boolean(
    (prompt.trim() || files.length > 0) && !isUploading,
  );

  const fileChips = useMemo(
    () => buildFileChips(files, parsedFiles, uploadStatus),
    [files, parsedFiles, uploadStatus],
  );

  const clearParseTimer = useCallback(() => {
    if (parseTimerRef.current) {
      clearInterval(parseTimerRef.current);
      parseTimerRef.current = null;
    }
  }, []);

  const onUploadFiles = useCallback(
    async (selectedFiles: FileList | null) => {
      if (!selectedFiles || selectedFiles.length === 0) return;
      const fileArray = Array.from(selectedFiles);
      setFiles(fileArray);
      setParsedFiles([]);
      setProjectId(null);
      setError(null);
      setUploadStatus("uploading");
      setUploadProgress(null);
      setParseElapsed(0);
      clearParseTimer();

      try {
        const project = await createProject(t.editor.projectName);
        const pid: string = project.id;
        setProjectId(pid);

        const result: UploadResponse = await uploadToAgentgo(
          fileArray,
          APP_CONFIG.interaction.agentgoBaseUrl,
          (progress) => setUploadProgress(progress),
          (status) => {
            setUploadStatus(status);
            if (status === "parsing") {
              parseTimerRef.current = setInterval(() => {
                setParseElapsed((prev) => prev + 1);
              }, 1000);
            }
          },
          pid,
        );
        clearParseTimer();
        setParsedFiles(result.files);
        setUploadStatus("done");

        const succeeded = result.files.filter(
          (f) => f.type !== "unsupported" && f.type !== "error",
        ).length;
        if (succeeded === 0 && result.files.length > 0) {
          setError(t.errors.unsupportedFormats);
        }
      } catch (err) {
        clearParseTimer();
        setUploadStatus("error");
        setError(err instanceof Error ? err.message : t.errors.retryUpload);
      }
    },
    [t.errors.unsupportedFormats, t.errors.retryUpload, clearParseTimer],
  );

  const onRemoveFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
    setParsedFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const onGenerate = async (designSkill?: string | null) => {
    if (!canGenerate || isGenerating) return;

    setIsGenerating(true);
    setError(null);

    try {
      const { provider, cfg } = getActiveConfig();
      if (cfg.apiKey && cfg.modelName) {
        const synced = await syncToAgent(provider);
        if (!synced.ok) {
          throw new Error(synced.message || t.errors.operationFailed);
        }
      }

      const finalPrompt = prompt.trim() || t.chat.inputInstructions;
      const projectName = finalPrompt.slice(0, 60) || t.editor.projectName;
      const projectPayload: Record<string, string> = {
        name: projectName,
        brief: finalPrompt,
      };
      if (designSkill) {
        projectPayload.design_skill = designSkill;
      }

      let pid: string;
      if (projectId) {
        await updateProject(projectId, projectPayload);
        pid = projectId;
      } else {
        const data = await createProject(projectPayload.name);
        pid = data.id;
      }

      const succeededParsed = parsedFiles.filter(
        (f) => f.type !== "unsupported" && f.type !== "error",
      );

      navigate(`${APP_CONFIG.routes.editor}/${pid}`, {
        state: {
          [EDITOR_PROMPT_KEY]: finalPrompt,
          ...(files.length > 0
            ? { [EDITOR_ATTACHMENTS_KEY]: files.map((f) => f.name) }
            : {}),
          ...(succeededParsed.length > 0
            ? { [EDITOR_UPLOADED_FILES_KEY]: succeededParsed }
            : {}),
        },
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errors.operationFailed);
      setIsGenerating(false);
    }
  };

  return {
    text: t.home,
    prompt,
    setPrompt,
    files,
    fileChips,
    onUploadFiles,
    onRemoveFile,
    isGenerating,
    canGenerate,
    onGenerate,
    error,
    appConfig: APP_CONFIG,
    uploadProgress,
    uploadStatus,
    parseElapsed,
    projectId,
  };
}
