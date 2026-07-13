import React, { useRef, useState } from "react";
import { useAutoResizeTextarea } from "../hooks/useAutoResizeTextarea";
import { motion } from "motion/react";
import {
  ArrowUp,
  Moon,
  Paperclip,
  Settings,
  Sun,
} from "lucide-react";
import { useHomeViewModel } from "../viewmodels/useHomeViewModel";
import { ThemeMode } from "../models/editor";
import { useThemeMode } from "../hooks/useThemeMode";
import { useLocale } from "../context/LocaleContext";
import { useProjectList } from "../hooks/useProjectList";
import { ProjectDrawer } from "../components/ProjectDrawer";
import { ProjectDrawerTrigger } from "../components/ProjectDrawerTrigger";
import { LocaleSwitcher } from "../components/LocaleSwitcher";
import { AniyaLogo } from "../components/AniyaLogo";
import { DotGridBackground } from "../components/DotGridBackground";
import { FileChipGroup } from "../components/chat/FileChip";
import { SkillTemplatePicker } from "../components/home/SkillTemplatePicker";
import { SettingsPanel } from "../components/settings/SettingsPanel";
import { Z_INDEX } from "../constants/zIndex";

export function HomeView() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const vm = useHomeViewModel();
  const { t } = useLocale();
  const { themeMode, toggleThemeMode } = useThemeMode();
  const { projects } = useProjectList();
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [selectedSkill, setSelectedSkill] = useState<string | null>(null);
  const promptArea = useAutoResizeTextarea(vm.prompt, { minRows: 3, maxRows: 8 });

  const isLight = themeMode === ThemeMode.Light;

  return (
    <div className="relative h-screen w-full bg-[var(--editor-bg)] text-[var(--editor-text)] flex flex-col items-center overflow-hidden selection:bg-[var(--editor-accent-soft)]">
      <DotGridBackground
        dotColor={isLight ? "rgba(0,0,0,0.05)" : "rgba(255,255,255,0.06)"}
        glowColor={isLight ? "rgba(100,160,240,0.3)" : "rgba(100,160,240,0.28)"}
      />

      {/* Top nav — kept below Z_INDEX.OVERLAY so modals/drawers/previews
          (all rendered via Portal into document.body) always win. */}
      <div
        className="absolute top-0 left-0 w-full p-6 flex items-center justify-between max-w-[1400px] mx-auto right-0"
        style={{ zIndex: Z_INDEX.CHROME }}
      >
        <motion.div
          initial={{ opacity: 0, x: -12 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="flex items-center gap-2.5"
        >
          <AniyaLogo size={28} state="idle" className="text-[var(--editor-text)]" />
          <span className="font-bold text-lg tracking-tight">
            {vm.text.brand}
          </span>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, x: 12 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="flex items-center gap-4"
        >
          <LocaleSwitcher />
          <button
            type="button"
            onClick={toggleThemeMode}
            className="flex items-center gap-2 text-[13px] font-semibold text-[var(--editor-text)] hover:opacity-60 transition-opacity"
          >
            {isLight ? <Sun size={16} /> : <Moon size={16} />}
            <span>{isLight ? t.theme.dark : t.theme.light}</span>
          </button>
          <button
            type="button"
            onClick={() => setIsSettingsOpen(true)}
            className="flex items-center text-[var(--editor-text)] hover:opacity-60 transition-opacity"
            title={t.settings.title}
          >
            <Settings size={18} strokeWidth={2} />
          </button>
        </motion.div>
      </div>

      {/* Hero */}
      <section className="flex-1 flex flex-col items-center justify-center w-full z-10">
        <div className="flex flex-col items-center w-full max-w-3xl px-4">

          {/* Character — large animated pixel art */}
          <motion.div
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1], delay: 0.05 }}
            className="mb-10 md:mb-12"
          >
            <AniyaLogo
              size={180}
              state="idle"
              className="text-[var(--editor-text)]"
            />
          </motion.div>

          {/* Input card */}
          <motion.div
            initial={{ opacity: 0, y: 24 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, ease: "easeOut", delay: 0.35 }}
            className="w-full max-w-[720px] relative"
            style={{
              boxShadow: isLight
                ? "0 25px 60px -10px rgba(0,0,0,0.15)"
                : "0 25px 60px -10px rgba(0,0,0,0.4)",
            }}
          >
            <div
              className="relative rounded-[20px] bg-[var(--editor-bg)] p-6 md:p-7"
              style={{ border: "3px solid var(--editor-text)" }}
            >
              {/* Tag — inside the card, top-left */}
              <div className="flex items-center gap-2.5 mb-5 pl-1">
                <div
                  className="w-2 h-2 rounded-full"
                  style={{ backgroundColor: "var(--editor-text)" }}
                />
                <span className="text-[11px] font-bold tracking-[0.15em] text-[var(--editor-text)] uppercase">
                  {t.home.tagline}
                </span>
              </div>

              {/* File chips */}
              <FileChipGroup files={vm.fileChips} onRemove={vm.onRemoveFile} />

              {/* Upload progress bar */}
              {(vm.uploadStatus === "uploading" || vm.uploadStatus === "parsing") && (
                <div className="flex items-center gap-3 mb-3 ml-1">
                  <div className="flex-1 h-1.5 rounded-full bg-[var(--editor-text-muted)] overflow-hidden">
                    {vm.uploadStatus === "uploading" ? (
                      <div
                        className="h-full rounded-full transition-all duration-300 ease-out"
                        style={{
                          width: `${vm.uploadProgress ?? 0}%`,
                          backgroundColor: "var(--editor-text)",
                        }}
                      />
                    ) : (
                      <div className="h-full w-full rounded-full animate-pulse" style={{ backgroundColor: "var(--editor-text)" }} />
                    )}
                  </div>
                  <span className="text-xs font-semibold text-[var(--editor-text)] whitespace-nowrap min-w-[110px]">
                    {vm.uploadStatus === "uploading"
                      ? `${t.upload.uploading} ${vm.uploadProgress ?? 0}%`
                      : `${t.upload.parsing} ${vm.parseElapsed}s`}
                  </span>
                </div>
              )}

              {/* Textarea */}
              <textarea
                ref={promptArea.ref}
                value={vm.prompt}
                onChange={(event) => vm.setPrompt(event.target.value)}
                onInput={promptArea.onInput}
                onKeyDown={(event) => {
                  if (
                    event.key === vm.appConfig.interaction.submitKey &&
                    !event.shiftKey &&
                    !event.nativeEvent.isComposing
                  ) {
                    event.preventDefault();
                    vm.onGenerate(selectedSkill);
                  }
                }}
                placeholder={vm.text.promptPlaceholder}
                className="flex-1 w-full bg-transparent border-none resize-none text-[var(--editor-text)] placeholder:text-[var(--editor-text-muted)] text-lg md:text-xl font-medium pl-1 leading-relaxed no-scrollbar focus:outline-none"
                rows={3}
                disabled={vm.isGenerating}
                style={{ outline: "none", boxShadow: "none", lineHeight: "28px" }}
              />

              {/* Bottom bar */}
              <div className="flex justify-between items-end mt-2">
                <div className="flex items-center gap-5 pl-1 pb-1">
                  <button
                    onClick={() => fileInputRef.current?.click()}
                    className="text-[var(--editor-text)] hover:opacity-60 transition-opacity"
                    disabled={vm.isGenerating || vm.uploadStatus === "uploading" || vm.uploadStatus === "parsing"}
                    title={t.home.attachFiles}
                  >
                    <Paperclip size={20} strokeWidth={2.5} />
                  </button>
                  <input
                    type="file"
                    ref={fileInputRef}
                    onChange={(event) => {
                      vm.onUploadFiles(event.target.files);
                      event.target.value = "";
                    }}
                    accept={vm.appConfig.interaction.htmlFileAccept}
                    multiple
                    className="hidden"
                  />
                  <SkillTemplatePicker
                    selectedSkill={selectedSkill}
                    onSelect={(skillName, prompt) => {
                      setSelectedSkill(skillName);
                      vm.setPrompt(prompt);
                    }}
                    onClear={() => {
                      setSelectedSkill(null);
                      vm.setPrompt("");
                    }}
                  />
                </div>

                {/* Generate button */}
                <button
                  onClick={() => vm.onGenerate(selectedSkill)}
                  disabled={!vm.canGenerate || vm.isGenerating}
                  className="flex items-center gap-2.5 px-5 py-2.5 rounded-xl text-[14px] font-bold transition-transform active:scale-95 disabled:opacity-20 disabled:cursor-not-allowed disabled:active:scale-100"
                  style={{
                    backgroundColor: "var(--editor-text)",
                    color: "var(--editor-bg)",
                  }}
                >
                  {vm.isGenerating ? vm.text.generating : vm.text.generate}
                  <ArrowUp size={15} strokeWidth={2.5} />
                </button>
              </div>
            </div>
          </motion.div>

          {/* Error */}
          {vm.error && (
            <motion.div
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              className="mt-4 rounded-xl px-4 py-2.5 text-sm text-center font-medium"
              style={{
                border: "2px solid var(--editor-danger, #ef4444)",
                color: "var(--editor-danger, #ef4444)",
              }}
            >
              {vm.error}
            </motion.div>
          )}

        </div>
      </section>

      {/* Project drawer trigger */}
      <ProjectDrawerTrigger
        onClick={() => setIsDrawerOpen(true)}
        projectCount={projects.length}
        isOpen={isDrawerOpen}
      />

      {/* Project drawer */}
      <ProjectDrawer
        isOpen={isDrawerOpen}
        onClose={() => setIsDrawerOpen(false)}
      />

      {/* Settings panel */}
      <SettingsPanel
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
      />
    </div>
  );
}
