import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import type { Editor } from "grapesjs";
import { logError } from "../utils/errorLogger";
import type { EditorError } from "../models/errors";
import {
  EDITOR_IMPORT_HTML_STATE_KEY,
  EDITOR_PROJECT_ID_KEY,
  EDITOR_ATTACHMENTS_KEY,
  EDITOR_PROMPT_KEY,
  EDITOR_UPLOADED_FILES_KEY,
  type EditorLocationState,
} from "../app/editorNavigation";
import { useLocale } from "../context/LocaleContext";
import { useChatStream } from "../hooks/useChatStream";
import { EditorMode, type EditorModeValue, type PageState } from "../models/editor";
import type { AttachmentMeta, ChatRequestPayload, DoneMeta } from "../models/chat";
import type { SkillOption } from "../components/chat/StylePicker";
import * as api from "../services/editorApi";
import type { ParsedFile } from "../services/upload";
import { clearEditorSelection, goToDeckSlide, readDeckState, type DeckState } from "../services/deckAdapter";
import { useEditorUpload } from "./useEditorUpload";
import { useEditorDirtyTracking } from "./useEditorDirtyTracking";
import { useEditorPanelState } from "./useEditorPanelState";
import { useEditorSelection } from "./useEditorSelection";
import { useEditorExport } from "./useEditorExport";
import { useEditorSave } from "./useEditorSave";

export function useEditorViewModel(editor: Editor | null) {
  const location = useLocation();
  const navigate = useNavigate();
  const params = useParams();
  const { t } = useLocale();
  const locationState = location.state as EditorLocationState | null;
  const routeProjectId = params.projectId || locationState?.[EDITOR_PROJECT_ID_KEY];
  const routePrompt = locationState?.[EDITOR_PROMPT_KEY];
  const routeAttachments = locationState?.[EDITOR_ATTACHMENTS_KEY];
  const routeUploadedFiles = locationState?.[EDITOR_UPLOADED_FILES_KEY];

  // ─── Refs ───
  const pendingAttachmentsRef = useRef<string[]>(routeAttachments ?? []);
  const pendingParsedFilesRef = useRef<ParsedFile[]>(routeUploadedFiles ?? []);
  const modeRef = useRef<EditorModeValue>(EditorMode.Design);
  const chatStartedRef = useRef(false);

  // ─── Editor state (must precede sub-hooks that depend on it) ───
  const [mode, setMode] = useState<EditorModeValue>(EditorMode.Design);
  const [pages, setPages] = useState<PageState[]>([
    { id: "page-1", title: "Page 1", html: "", css: "" },
  ]);
  const [currentPageIndex, setCurrentPageIndex] = useState(0);
  const [deckState, setDeckState] = useState<DeckState | null>(null);
  const [persistedHtml, setPersistedHtml] = useState<string>("");
  const [projectName, setProjectName] = useState<string>("");

  // ─── Sub-hooks (depends on persistedHtml) ───
  const dirty = useEditorDirtyTracking();
  const panels = useEditorPanelState();
  const selection = useEditorSelection();
  const exp = useEditorExport(editor, persistedHtml, routeProjectId);
  const save = useEditorSave(editor, routeProjectId, persistedHtml, dirty.captureCleanSnapshot);

  // ─── Chat interaction state ───
  const [magicPrompt, setMagicPrompt] = useState("");
  const [magicError, setMagicError] = useState<string | null>(null);

  // ─── Bootstrap / Skill state ───
  const [emptyProject, setEmptyProject] = useState(false);
  const [projectDesignSkill, setProjectDesignSkill] = useState("");
  const [previewSkill, setPreviewSkill] = useState<SkillOption | null>(null);
  const [previewedSkills, setPreviewedSkills] = useState<Set<string>>(new Set());
  const [doneSince, setDoneSince] = useState<number | null>(null);

  // ─── Upload ───
  const upload = useEditorUpload(routeProjectId, (msg) => setMagicError(msg));

  // ─── Basic callbacks ───

  const toAttachmentMeta = useCallback((f: ParsedFile): AttachmentMeta => ({
    original_name: f.original_name,
    saved_path_rel: f.saved_path_rel,
    type: f.type,
    pages: f.pages,
    char_count: f.char_count,
    width: f.width,
    height: f.height,
    format: f.format,
    error: f.error,
  }), []);

  const updateProjectNameFromHtml = useCallback(
    (html: string) => {
      if (!routeProjectId) return;
      let title: string | undefined;
      const titleMatch = html.match(/<title[^>]*>(.*?)<\/title>/is);
      title = titleMatch?.[1]?.trim();
      if (!title) {
        const h1Match = html.match(/<h1[^>]*>(.*?)<\/h1>/is);
        title = h1Match?.[1]?.trim();
      }
      if (!title || title === projectName) return;
      fetch(`/api/v1/projects/${encodeURIComponent(routeProjectId)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: title }),
      }).then(() => setProjectName(title)).catch((err) => logError('updateProjectName', err));
    },
    [routeProjectId, projectName],
  );

  // ─── Chat stream ───

  const onChatDone = useCallback(async (meta: DoneMeta) => {
    if (!editor || !routeProjectId) return;
    try {
      const response = await fetch(`/api/v1/projects/${routeProjectId}/export`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const { html } = await response.json();
      setPersistedHtml(html);
      api.importHtmlDocument(editor, html, t, routeProjectId);
      setPages([{ id: "page-1", title: "Page 1", html: editor.getHtml(), css: editor.getCss() }]);
      setCurrentPageIndex(0);
      setDeckState(readDeckState(editor));
      dirty.captureCleanSnapshot(editor);
      setEmptyProject(false);
      updateProjectNameFromHtml(html);
    } catch (err) {
      logError('importHtml', err);
      setMagicError(t.errors.canvasUpdateFailed);
      setEmptyProject(false);
    }
  }, [editor, routeProjectId, t, dirty.captureCleanSnapshot, updateProjectNameFromHtml]);

  const onChatError = useCallback(() => {
    setEmptyProject(false);
  }, []);

  const { chatState, startChat, stopChat, resetChat, loadChatHistory, clearSkillRecommendations, recommendStyles } = useChatStream({
    onDone: onChatDone,
    onError: onChatError,
  });

  // ─── Callbacks that depend on chat stream + upload ───

  const resolveAttachments = useCallback((): AttachmentMeta[] | undefined => {
    if (upload.files.length > 0 && upload.uploadStatus === "done") {
      return upload.parsedFiles
        .filter((f) => f.type !== "unsupported" && f.type !== "error")
        .map(toAttachmentMeta);
    }
    if (pendingParsedFilesRef.current.length > 0) {
      return pendingParsedFilesRef.current.map(toAttachmentMeta);
    }
    return undefined;
  }, [upload.files, upload.parsedFiles, upload.uploadStatus, toAttachmentMeta]);

  const sendWithSkillCheck = useCallback(
    async (projectId: string, prompt: string, selectedDom?: ChatRequestPayload["selected_dom"]) => {
      setMagicError(null);
      setMagicPrompt("");
      panels.setIsTerminalOpen(true);
      const attachments = resolveAttachments();
      startChat(projectId, prompt, selectedDom, attachments);
      pendingAttachmentsRef.current = [];
      pendingParsedFilesRef.current = [];
      if (upload.files.length > 0) {
        upload.setFiles([]);
        upload.setParsedFiles([]);
        upload.setUploadStatus("idle");
        upload.setUploadProgress(null);
      }
    },
    [startChat, resolveAttachments, upload.files.length, upload.setFiles, upload.setParsedFiles, upload.setUploadStatus, upload.setUploadProgress],
  );

  const handleSkillSelect = useCallback(
    async (sk: SkillOption) => {
      clearSkillRecommendations();
      if (!routeProjectId) return;
      setMagicError(null);
      setMagicPrompt("");
      panels.setIsTerminalOpen(true);

      try {
        await fetch(`/api/v1/projects/${encodeURIComponent(routeProjectId)}`, {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ design_skill: sk.name }),
        });
        setProjectDesignSkill(sk.name);
      } catch (err) {
        logError('handleSkillSelect:patchDesignSkill', err);
      }

      const prompt = chatState.lastPrompt || routePrompt || "";
      if (!prompt) return;
      const attachments = resolveAttachments();
      startChat(routeProjectId, prompt, chatState.lastSelectedDom, attachments);
      pendingAttachmentsRef.current = [];
      pendingParsedFilesRef.current = [];
    },
    [startChat, clearSkillRecommendations, routeProjectId, chatState.lastPrompt, chatState.lastSelectedDom, routePrompt, resolveAttachments],
  );

  const handleSkillClose = useCallback(() => {
    clearSkillRecommendations();
    setPreviewSkill(null);
  }, [clearSkillRecommendations]);

  const handlePreview = useCallback((sk: SkillOption) => {
    setPreviewedSkills((prev) => new Set(prev).add(sk.name));
    setPreviewSkill(sk);
  }, []);

  const handleClosePreview = useCallback(() => {
    setPreviewSkill(null);
  }, []);

  const handleNavigatePreview = useCallback((sk: SkillOption) => {
    setPreviewedSkills((prev) => new Set(prev).add(sk.name));
    setPreviewSkill(sk);
  }, []);

  const handleUseStyle = useCallback(
    (sk: SkillOption) => {
      setPreviewSkill(null);
      handleSkillSelect(sk);
    },
    [handleSkillSelect],
  );

  // ─── Editor operation callbacks ───

  const saveCurrentPage = useCallback(() => {
    if (!editor) return pages;
    if (deckState) return pages;
    const next = pages.map((page, index) =>
      index === currentPageIndex
        ? { ...page, html: editor.getHtml(), css: editor.getCss() }
        : page,
    );
    setPages(next);
    return next;
  }, [currentPageIndex, deckState, editor, pages]);

  const goToPage = useCallback(
    (nextIndex: number) => {
      if (!editor) return;
      if (deckState) {
        const nextDeck = goToDeckSlide(editor, nextIndex);
        if (nextDeck) {
          clearEditorSelection(editor);
          selection.setSelectedContext(null);
          setDeckState(nextDeck);
        } else {
          setDeckState(null);
        }
        return;
      }
      if (nextIndex < 0 || nextIndex >= pages.length || nextIndex === currentPageIndex) return;
      const nextPages = saveCurrentPage();
      const target = nextPages[nextIndex];
      api.setHtmlAndCss(editor, target.html, target.css);
      clearEditorSelection(editor);
      selection.setSelectedContext(null);
      setCurrentPageIndex(nextIndex);
    },
    [currentPageIndex, deckState, editor, pages.length, saveCurrentPage, selection.setSelectedContext],
  );

  const applyMagicPrompt = useCallback(() => {
    if (!editor) return setMagicError(t.errors.editorNotInitialized);
    if (mode !== EditorMode.Design) return setMagicError(t.errors.switchToDesignFirst);
    const prompt = magicPrompt.trim();
    if (!prompt) return setMagicError(t.errors.enterStylePrompt);
    if (!routeProjectId) return setMagicError(t.errors.projectIdMissing);

    const selected = editor.getSelected();
    const ctx = selection.selectedContext;
    const domContext = selected && ctx
      ? {
          css_path: ctx.path,
          tag: ctx.tagName,
          text: ctx.label,
          styles: ctx.styles || {},
        }
      : undefined;

    sendWithSkillCheck(routeProjectId, prompt, domContext);
  }, [editor, mode, magicPrompt, routeProjectId, selection.selectedContext, sendWithSkillCheck]);

  const handleRetryChat = useCallback(() => {
    if (!routeProjectId) return;
    const prompt = chatState.lastPrompt || routePrompt;
    if (!prompt) return;
    sendWithSkillCheck(routeProjectId, prompt);
  }, [routeProjectId, chatState.lastPrompt, routePrompt, sendWithSkillCheck]);

  // ─── Navigation delegates ───

  const onSaveAndProceed = useCallback(
    async (_mode: "navigate" | "rollback"): Promise<boolean> => {
      return save.handleSaveAndReturnOk();
    },
    [save.handleSaveAndReturnOk],
  );

  const onDiscardNavigate = useCallback(() => {
    if (editor && persistedHtml) {
      api.importHtmlDocument(editor, persistedHtml, t, routeProjectId);
      setPages([{ id: "page-1", title: "Page 1", html: editor.getHtml(), css: editor.getCss() }]);
      setCurrentPageIndex(0);
      setDeckState(readDeckState(editor));
    }
    dirty.captureCleanSnapshot(editor);
  }, [editor, persistedHtml, routeProjectId, dirty.captureCleanSnapshot]);

  const onDiscardRollback = useCallback(() => {
    dirty.isDirtyRef.current = false;
  }, [dirty.isDirtyRef]);

  // ─── Computed values ───

  const isGenerating =
    chatState.streamStatus === "connecting" || chatState.streamStatus === "streaming";

  const chatContextLabel = useMemo(() => {
    if (mode !== EditorMode.Design || !selection.selectedContext) return null;
    return selection.selectedContext.label;
  }, [mode, selection.selectedContext]);

  // ─── Mode ref sync ───
  modeRef.current = mode;

  // ─── Effects ───

  // Logo doneSince timer
  useEffect(() => {
    if (chatState.streamStatus === "done") {
      const now = Date.now();
      setDoneSince(now);
      const timer = window.setTimeout(() => setDoneSince(null), 10000);
      return () => window.clearTimeout(timer);
    }
    setDoneSince(null);
  }, [chatState.streamStatus]);

  // Bootstrap: design_skill SSOT → startChat; otherwise recommend-styles
  useEffect(() => {
    if (!editor || !routeProjectId || chatStartedRef.current) return;

    let cancelled = false;
    (async () => {
      try {
        const resp = await fetch(`/api/v1/projects/${encodeURIComponent(routeProjectId)}`);
        if (!resp.ok || cancelled) return;
        const project = await resp.json();
        if (cancelled) return;

        setProjectDesignSkill(project.design_skill || "");
        if (project.name) setProjectName(project.name);

        if (project.has_html) return;

        const prompt = routePrompt || project.brief;
        if (!prompt) return;

        chatStartedRef.current = true;
        if (routePrompt) {
          navigate(location.pathname, { replace: true, state: {} });
        }

        panels.setIsTerminalOpen(true);
        const attachments = resolveAttachments();

        if (project.design_skill) {
          clearSkillRecommendations();
          startChat(routeProjectId, prompt, undefined, attachments);
          pendingAttachmentsRef.current = [];
          pendingParsedFilesRef.current = [];
        } else {
          recommendStyles(
            routeProjectId,
            prompt,
            pendingParsedFilesRef.current.map(toAttachmentMeta),
          );
        }
      } catch (err) {
        logError('bootstrapChat', err);
      }
    })();

    return () => { cancelled = true; };
  }, [
    editor,
    routeProjectId,
    routePrompt,
    navigate,
    location.pathname,
    recommendStyles,
    startChat,
    clearSkillRecommendations,
    resolveAttachments,
    toAttachmentMeta,
  ]);

  // Load existing project content when arriving without a prompt
  useEffect(() => {
    if (!editor || !routeProjectId || routePrompt) return;

    let cancelled = false;
    const load = async () => {
      try {
        const response = await fetch(`/api/v1/projects/${routeProjectId}/active-file`);
        if (!response.ok) {
          if (response.status === 404) {
            setMagicError(t.errors.projectNotFound);
          }
          return;
        }
        if (cancelled) return;
        const { filename, content } = await response.json();
        if (!content || !filename) {
          setEmptyProject(true);
          return;
        }
        if (cancelled) return;
        setPersistedHtml(content);
        api.importHtmlDocument(editor, content, t, routeProjectId);
        setPages([{ id: "page-1", title: "Page 1", html: editor.getHtml(), css: editor.getCss() }]);
        setCurrentPageIndex(0);
        setDeckState(readDeckState(editor));
        dirty.captureCleanSnapshot(editor);
        updateProjectNameFromHtml(content);
      } catch (err) {
        logError('loadProjectContent', err);
      }
    };
    load();
    return () => { cancelled = true; };
  }, [editor, routeProjectId, routePrompt, t, dirty.captureCleanSnapshot, updateProjectNameFromHtml]);

  // Load chat history when opening an existing project
  useEffect(() => {
    if (!routeProjectId) return;
    loadChatHistory(routeProjectId);
  }, [routeProjectId, loadChatHistory]);

  // Load project name from metadata
  useEffect(() => {
    if (!routeProjectId) return;
    let cancelled = false;
    (async () => {
      try {
        const resp = await fetch(`/api/v1/projects/${encodeURIComponent(routeProjectId)}`);
        if (!resp.ok || cancelled) return;
        const project = await resp.json();
        if (cancelled) return;
        if (project.name) setProjectName(project.name);
        setProjectDesignSkill(project.design_skill || "");
      } catch (err) {
        logError('loadProjectName', err);
      }
    })();
    return () => { cancelled = true; };
  }, [routeProjectId]);

  // Imported HTML (upload flow)
  useEffect(() => {
    if (!editor) return;
    const state = location.state as EditorLocationState | null;
    const html = state?.[EDITOR_IMPORT_HTML_STATE_KEY];
    if (!html || typeof html !== "string" || !html.trim() || routeProjectId) return;

    try {
      setPersistedHtml(html);
      api.importHtmlDocument(editor, html, t);
    } catch (error) {
      setMagicError(error instanceof Error ? error.message : t.errors.importHtmlFailed);
      return;
    }
    setPages([{ id: "page-1", title: "Page 1", html: editor.getHtml(), css: editor.getCss() }]);
    setCurrentPageIndex(0);
    const timer = window.setTimeout(() => {
      setDeckState(readDeckState(editor));
      dirty.captureCleanSnapshot(editor);
    }, 120);
    navigate(location.pathname, { replace: true, state: {} });
    return () => window.clearTimeout(timer);
  }, [editor, location.pathname, location.state, navigate, routeProjectId, t, dirty.captureCleanSnapshot]);

  // Derived error for structured display
  const editorError: EditorError | null = useMemo(() => {
    if (!magicError) return null;
    // Classify based on message content
    if (magicError.includes('export') || magicError.includes('Export')) return { type: 'export_pdf', message: magicError };
    if (magicError.includes('version') || magicError.includes('Version')) return { type: 'version_restore', message: magicError };
    if (magicError.includes('import') || magicError.includes('Import') || magicError.includes('canvas update')) return { type: 'canvas_update', message: magicError };
    return { type: 'chat', message: magicError };
  }, [magicError]);

  // Effective logo state
  const effectiveLogoState =
    chatState.streamStatus === "connecting" || chatState.streamStatus === "streaming"
      ? ("thinking" as const)
      : chatState.streamStatus === "done" && doneSince !== null
        ? ("done" as const)
        : ("idle" as const);

  return {
    // Route
    routeProjectId,

    // Upload
    upload,

    // Panels
    isTerminalOpen: panels.isTerminalOpen,
    setIsTerminalOpen: panels.setIsTerminalOpen,
    isVersionOpen: panels.isVersionOpen,
    setIsVersionOpen: panels.setIsVersionOpen,
    isAdvancedDesignOpen: panels.isAdvancedDesignOpen,
    setIsAdvancedDesignOpen: panels.setIsAdvancedDesignOpen,

    // Editor
    mode, setMode,
    pages, currentPageIndex,
    deckState, setDeckState,
    persistedHtml,
    projectName,

    // Preview
    previewUrl: exp.previewUrl,
    setPreviewUrl: exp.setPreviewUrl,
    previewError: exp.previewError,
    setPreviewError: exp.setPreviewError,

    // Modals
    precipitateOpen: panels.precipitateOpen,
    setPrecipitateOpen: panels.setPrecipitateOpen,
    precipitateHtml: panels.precipitateHtml,

    imageDialogOpen: panels.imageDialogOpen,
    setImageDialogOpen: panels.setImageDialogOpen,
    imageData: panels.imageData,
    setImageData: panels.setImageData,
    imageError: panels.imageError,
    setImageError: panels.setImageError,

    // Selection
    selectedContext: selection.selectedContext,
    setSelectedContext: selection.setSelectedContext,

    // Chat
    magicPrompt, setMagicPrompt,
    magicError, setMagicError,
    editorError,
    emptyProject,
    projectDesignSkill,
    chatContextLabel,

    // Dirty / Save
    isDirtyRef: dirty.isDirtyRef,
    dirtyTick: dirty.dirtyTick,
    saveState: save.saveState,
    versionRefreshKey: save.versionRefreshKey,
    markDirty: dirty.markDirty,
    captureCleanSnapshot: dirty.captureCleanSnapshot,
    checkAndUpdateDirty: dirty.checkAndUpdateDirty,
    handleSaveAndReturnOk: save.handleSaveAndReturnOk,

    // Navigation delegates
    onSaveAndProceed,
    onDiscardNavigate,
    onDiscardRollback,

    // Skill
    previewSkill, previewedSkills,
    handleSkillSelect, handleSkillClose,
    handlePreview, handleClosePreview, handleNavigatePreview, handleUseStyle,

    // Operations
    saveCurrentPage,
    goToPage,
    openPreview: exp.openPreview,
    handleExport: exp.handleExport,
    handleExportPdf: exp.handleExportPdf,
    handleExportPptx: exp.handleExportPptx,
    handlePrecipitateSkill: exp.handlePrecipitateSkill,
    handleSave: save.handleSave,
    handleActualRestore: save.handleActualRestore,
    handleRetryChat,
    applyMagicPrompt,

    // Chat stream
    chatState,
    stopChat,

    // Computed
    isGenerating,
    effectiveLogoState,
  };
}
