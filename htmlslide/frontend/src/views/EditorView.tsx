import { useCallback } from "react";
import { AnimatePresence, motion } from "motion/react";
import { useNavigate } from "react-router";
import { Moon, Redo2, Sun, Undo2 } from "lucide-react";
import type { Editor } from "grapesjs";
import { EditorProvider } from "../context/EditorContext";
import { useLocale } from "../context/LocaleContext";
import { APP_CONFIG } from "../app/config";
import { LocaleSwitcher } from "../components/LocaleSwitcher";
import { LoadingOverlay } from "../components/LoadingOverlay";
import { EditorFloatingMenu } from "../components/editor/EditorFloatingMenu";
import { EditorToolbar } from "../components/editor/EditorToolbar";
import { EditorCanvas } from "../components/editor/EditorCanvas";
import { EditorBlockPanel } from "../components/editor/EditorBlockPanel";
import { EditorRightDock } from "../components/editor/EditorRightDock";
import { AIChatPanel, VersionPanel } from "../components/editor/EditorPanels";
import { useEditor } from "../hooks/useEditor";
import { StylePicker } from "../components/chat/StylePicker";
import { SkillPreviewPanel } from "../components/chat/SkillPreviewPanel";
import type { SkillRecommendation } from "../models/chat";
import { ThemeMode, type ThemeModeValue } from "../models/editor";
import * as api from "../services/editorApi";
import { EditorPreviewModal } from "../components/editor/EditorPreviewModal";
import { PrecipitateSkillModal } from "../components/editor/PrecipitateSkillModal";
import { UnsavedChangesDialog } from "../components/editor/UnsavedChangesDialog";
import { ImageReplaceDialog } from "../components/editor/ImageReplaceDialog";
import { DocumentViewerModal } from "../components/editor/DocumentViewerModal";
import { PageControls } from "../components/editor/PageControls";
import { useThemeMode } from "../hooks/useThemeMode";
import { DotGridBackground } from "../components/DotGridBackground";
import { useEditorViewModel } from "../viewmodels/useEditorViewModel";
import { useEditorNavigation } from "../viewmodels/useEditorNavigation";
import { useEditorEventWiring } from "../hooks/useEditorEventWiring";
import { EditorModeToggle } from "../components/editor/EditorModeToggle";
import { EditorSideActions } from "../components/editor/EditorSideActions";
import { EditorChatBar } from "../components/editor/EditorChatBar";
import { Z_INDEX } from "../constants/zIndex";
import type { SkillOption } from "../components/chat/StylePicker";

function mapRecommendationsToSkills(recommendations: SkillRecommendation[]): SkillOption[] {
  return recommendations.map((r): SkillOption => ({
    name: r.name,
    description: r.description,
    reason: r.reason,
    triggers: [],
    scenario: r.scenario,
    has_assets: r.has_assets,
    has_preview: r.has_preview,
  }));
}

function EditorViewContent({
  themeMode,
  onToggleTheme,
}: {
  themeMode: ThemeModeValue;
  onToggleTheme: () => void;
}) {
  const { t } = useLocale();
  const editor = useEditor();
  const navigate = useNavigate();
  const vm = useEditorViewModel(editor);
  const nav = useEditorNavigation({
    editor,
    isDirtyRef: vm.isDirtyRef,
    checkAndUpdateDirty: vm.checkAndUpdateDirty,
    onSaveAndProceed: vm.onSaveAndProceed,
    onDiscardNavigate: vm.onDiscardNavigate,
    onDiscardRollback: vm.onDiscardRollback,
    goToPage: vm.goToPage,
    currentPageIndex: vm.currentPageIndex,
    deckState: vm.deckState,
    imageDialogOpen: vm.imageDialogOpen,
    previewUrl: vm.previewUrl,
  });

  // ─── GrapesJS event wiring ───
  useEditorEventWiring({
    editor,
    mode: vm.mode,
    markDirty: vm.markDirty,
    checkAndUpdateDirty: vm.checkAndUpdateDirty,
    setDeckState: vm.setDeckState,
    setSelectedContext: vm.setSelectedContext,
  });

  // ─── Local helpers ───

  const openImageReplace = useCallback(() => {
    vm.setIsTerminalOpen(false);
    vm.setIsVersionOpen(false);
    if (!editor) {
      vm.setImageError(t.errors.editorNotInitialized);
      vm.setImageDialogOpen(true);
      return;
    }
    const data = api.getSelectedImageData(editor);
    if (!data) {
      vm.setImageError(t.errors.selectImageFirst);
      vm.setImageDialogOpen(true);
      return;
    }
    vm.setImageData(data);
    vm.setImageError(null);
    vm.setImageDialogOpen(true);
  }, [editor, vm.setIsTerminalOpen, vm.setIsVersionOpen, vm.setImageError, vm.setImageDialogOpen, vm.setImageData, t]);

  const toggleAdvancedDesign = useCallback(() => {
    if (!editor) return;
    vm.setIsAdvancedDesignOpen((prev) => !prev);
  }, [editor, vm.setIsAdvancedDesignOpen]);

  // ─── Destructure for render ───

  const {
    isTerminalOpen, setIsTerminalOpen,
    isVersionOpen, setIsVersionOpen,
    isAdvancedDesignOpen,
    mode, setMode,
    currentPageIndex,
    deckState,
    pages,
    upload,
    saveState,
    versionRefreshKey,
    emptyProject,
    projectDesignSkill,
    magicPrompt, setMagicPrompt,
    magicError, editorError,
    imageDialogOpen, setImageDialogOpen,
    imageData,
    imageError, setImageError,
    precipitateOpen, setPrecipitateOpen, precipitateHtml,
    previewUrl, setPreviewUrl, previewError, setPreviewError,
    projectName,
    chatState,
    stopChat,
    handleRetryChat,
    handleSave,
    handleActualRestore,
    handleSkillSelect, handleSkillClose, handlePreview, handleClosePreview, handleNavigatePreview, handleUseStyle,
    saveCurrentPage,
    goToPage,
    openPreview,
    handleExport, handleExportPdf, handleExportPptx,
    handlePrecipitateSkill,
    previewSkill, previewedSkills,
    isGenerating,
    effectiveLogoState,
    chatContextLabel,
    isDirtyRef,
    persistedHtml,
  } = vm;

  return (
    <div
      data-editor-mode={mode}
      className="flex flex-col h-screen w-screen overflow-hidden bg-[var(--editor-bg)] text-[var(--editor-text)] selection:bg-[var(--editor-accent-soft)] font-sans relative"
    >
      {!isAdvancedDesignOpen && (
        <EditorFloatingMenu
          onPreview={() => nav.requestNavigation(openPreview)}
          onExport={() => nav.requestNavigation(handleExport)}
          onExportPdf={() => nav.requestNavigation(handleExportPdf)}
          onExportPptx={() => nav.requestNavigation(handleExportPptx)}
          onPrecipitateSkill={() => nav.requestNavigation(handlePrecipitateSkill)}
          onNavigateHome={() => nav.requestNavigation(() => navigate(APP_CONFIG.routes.home))}
          projectName={projectName}
        />
      )}

      {/* Mode toggle */}
      {!isAdvancedDesignOpen && (
        <EditorModeToggle
          mode={mode}
          onSetMode={(newMode) => {
            if (mode === "design" && newMode === "direct") saveCurrentPage();
            if (mode === "direct" && newMode === "design") {
              nav.requestNavigation(() => setMode(newMode));
            } else {
              setMode(newMode);
            }
          }}
        />
      )}

      <div className="flex-1 flex flex-row overflow-hidden w-full h-full">
        <div className="flex-1 flex flex-col relative overflow-hidden">
          {!isAdvancedDesignOpen && (
            <EditorToolbar
              mode={mode}
              isAdvancedDesignOpen={isAdvancedDesignOpen}
              onToggleAdvancedDesign={toggleAdvancedDesign}
              onOpenImageReplace={openImageReplace}
            />
          )}
          <div className="flex-1 overflow-hidden relative w-full h-full">
            <DotGridBackground
              dotColor={themeMode === ThemeMode.Light ? "rgba(0,0,0,0.04)" : "rgba(255,255,255,0.05)"}
              glowColor="rgba(100,160,240,0.22)"
            />
            <div className="relative z-[1] w-full h-full">
              <EditorCanvas advancedOpen={isAdvancedDesignOpen} />
              {isAdvancedDesignOpen && (
                <div className="absolute top-4 left-1/2 -translate-x-1/2 z-40 flex items-center gap-1.5 p-1 rounded-xl bg-[var(--editor-surface)] border border-[var(--editor-border)] shadow-sm">
                  <button
                    type="button"
                    onClick={() => editor?.UndoManager?.undo()}
                    className="p-2 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-all"
                    title={t.toolbar.undoTitle}
                  >
                    <Undo2 size={16} strokeWidth={2} />
                  </button>
                  <button
                    type="button"
                    onClick={() => editor?.UndoManager?.redo()}
                    className="p-2 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-all"
                    title={t.toolbar.redoTitle}
                  >
                    <Redo2 size={16} strokeWidth={2} />
                  </button>
                </div>
              )}
            </div>
            {isGenerating && (
              <LoadingOverlay variant={emptyProject ? "plain" : "frosted"} themeMode={themeMode} />
            )}
            {emptyProject && !isGenerating && (
              <LoadingOverlay variant="plain" themeMode={themeMode} />
            )}
          </div>

          {/* Block panel (direct mode) */}
          <AnimatePresence>
            {!isAdvancedDesignOpen && mode === "direct" && (
              <motion.div
                className="absolute right-6 top-1/2 -translate-y-1/2 z-50"
                initial={{ opacity: 0, x: 16, scale: 0.96 }}
                animate={{ opacity: 1, x: 0, scale: 1 }}
                exit={{ opacity: 0, x: 16, scale: 0.96 }}
                transition={{ duration: 0.2, ease: "easeOut" }}
              >
                <EditorBlockPanel onOpenDocuments={() => upload.setDocumentViewerOpen(true)} />
              </motion.div>
            )}
          </AnimatePresence>

          <PageControls
            currentPage={deckState?.current ?? currentPageIndex}
            pageCount={deckState?.total ?? pages.length}
            onPrev={() => goToPage((deckState?.current ?? currentPageIndex) - 1)}
            onNext={() => goToPage((deckState?.current ?? currentPageIndex) + 1)}
            className={mode === "design" ? "bottom-20" : "bottom-6"}
          />

          {/* Side actions */}
          {!isAdvancedDesignOpen && (
            <EditorSideActions
              isTerminalOpen={isTerminalOpen}
              isVersionOpen={isVersionOpen}
              saveState={saveState}
              isDirty={isDirtyRef.current}
              hasProject={!!vm.routeProjectId}
              hasHtml={!!persistedHtml}
              effectiveLogoState={effectiveLogoState}
              onToggleTerminal={() => {
                setIsTerminalOpen(!isTerminalOpen);
                if (!isTerminalOpen) { setIsVersionOpen(false); setImageDialogOpen(false); }
              }}
              onToggleVersion={() => {
                setIsVersionOpen(!isVersionOpen);
                if (!isVersionOpen) { setIsTerminalOpen(false); setImageDialogOpen(false); }
              }}
              onSave={handleSave}
            />
          )}

          {/* Panels */}
          <AnimatePresence>
            {!isAdvancedDesignOpen && isTerminalOpen && (
              <AIChatPanel
                isOpen={isTerminalOpen}
                onClose={() => setIsTerminalOpen(false)}
                chatState={chatState}
                onStop={stopChat}
                onRetry={handleRetryChat}
              />
            )}
          </AnimatePresence>
          <AnimatePresence>
            {!isAdvancedDesignOpen && isVersionOpen && (
              <VersionPanel
                isOpen={isVersionOpen}
                onClose={() => setIsVersionOpen(false)}
                projectId={vm.routeProjectId}
                onRequestRestore={(vid) => nav.requestRollback(() => handleActualRestore(vid))}
                refreshTrigger={versionRefreshKey}
              />
            )}
          </AnimatePresence>

          <UnsavedChangesDialog
            open={nav.unsavedDialog.open}
            mode={nav.unsavedDialog.mode}
            saving={saveState === "saving"}
            onSaveAndProceed={nav.handleSaveAndProceed}
            onDiscard={nav.handleDiscard}
            onCancel={nav.handleCancelNavigation}
          />

          {/* Chat bar */}
          {!isAdvancedDesignOpen && (
            <>
              <AnimatePresence>
                {previewSkill && (
                  <SkillPreviewPanel
                    key={previewSkill.name}
                    skill={previewSkill}
                    skills={chatState.skillRecommendations ? mapRecommendationsToSkills(chatState.skillRecommendations) : []}
                    onClose={handleClosePreview}
                    onUseStyle={handleUseStyle}
                    onNavigate={handleNavigatePreview}
                  />
                )}
                {previewUrl !== null && (
                  <EditorPreviewModal
                    key="editor-preview"
                    src={previewUrl}
                    error={previewError}
                    onClose={() => {
                      if (previewUrl) URL.revokeObjectURL(previewUrl);
                      setPreviewUrl(null);
                      setPreviewError(null);
                    }}
                  />
                )}
              </AnimatePresence>

              <AnimatePresence>
                {!projectDesignSkill && chatState.skillRecommendations && chatState.skillRecommendations.length > 0 && !previewSkill && (
                  <div key="style-picker-wrapper" className="pointer-events-auto absolute bottom-32 left-0 right-0 z-50 flex flex-col items-center">
                    <div className="mb-3 w-full max-w-[520px]">
                      <StylePicker
                        skills={mapRecommendationsToSkills(chatState.skillRecommendations)}
                        onSelect={handleSkillSelect}
                        onPreview={handlePreview}
                        onClose={handleSkillClose}
                        previewedSkills={previewedSkills}
                      />
                    </div>
                  </div>
                )}
              </AnimatePresence>

              <EditorChatBar
                mode={mode}
                magicPrompt={magicPrompt}
                magicError={magicError}
                editorError={editorError}
                chatContextLabel={chatContextLabel}
                upload={upload}
                onMagicPromptChange={setMagicPrompt}
                onApplyMagic={vm.applyMagicPrompt}
              />
            </>
          )}
        </div>

        {/* Advanced design dock */}
        <AnimatePresence
          onExitComplete={() => {
            if (editor) editor.refresh({ tools: true });
          }}
        >
          {isAdvancedDesignOpen && editor && (
            <EditorRightDock
              isOpen={isAdvancedDesignOpen}
              onClose={() => vm.setIsAdvancedDesignOpen(false)}
              editor={editor}
            />
          )}
        </AnimatePresence>

        <DocumentViewerModal
          isOpen={upload.documentViewerOpen}
          onClose={() => upload.setDocumentViewerOpen(false)}
          documents={upload.parsedFiles}
          projectId={vm.routeProjectId ?? ""}
          agentgoBaseUrl={APP_CONFIG.interaction.agentgoBaseUrl}
        />
      </div>

      {/* Image replace dialog */}
      <ImageReplaceDialog
        isOpen={imageDialogOpen && !isAdvancedDesignOpen}
        initialSrc={imageData.src}
        error={imageError}
        onClose={() => setImageDialogOpen(false)}
        onSubmit={(payload) => {
          if (!editor) return setImageError(t.errors.editorNotInitialized);
          try {
            api.replaceSelectedImage(editor, payload, t);
            setImageError(null);
            setImageDialogOpen(false);
          } catch (error) {
            setImageError(error instanceof Error ? error.message : t.errors.replaceImageFailed);
          }
        }}
      />

      {precipitateOpen && vm.routeProjectId && (
        <PrecipitateSkillModal
          projectId={vm.routeProjectId}
          htmlContent={precipitateHtml}
          onClose={() => setPrecipitateOpen(false)}
        />
      )}

      {/* Top-right controls — hidden in advanced design mode, where the
          right dock occupies the same corner of the screen. */}
      {!isAdvancedDesignOpen && (
        <div
          className="absolute right-6 top-6 flex items-center gap-4"
          style={{ zIndex: Z_INDEX.CHROME }}
        >
          <LocaleSwitcher />
          <button
            type="button"
            onClick={onToggleTheme}
            className="flex items-center gap-2 text-[13px] font-semibold text-[var(--editor-text)] hover:opacity-60 transition-opacity"
          >
            {themeMode === ThemeMode.Light ? <Sun size={16} /> : <Moon size={16} />}
            <span>{themeMode === ThemeMode.Light ? t.theme.dark : t.theme.light}</span>
          </button>
        </div>
      )}
    </div>
  );
}

export function EditorView() {
  const { themeMode, toggleThemeMode } = useThemeMode();
  const { locale } = useLocale();

  return (
    <EditorProvider themeMode={themeMode} locale={locale}>
      <EditorViewContent
        themeMode={themeMode}
        onToggleTheme={toggleThemeMode}
      />
    </EditorProvider>
  );
}
