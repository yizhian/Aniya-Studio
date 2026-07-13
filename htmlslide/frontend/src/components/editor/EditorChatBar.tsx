import React from "react";
import { AnimatePresence, motion } from "motion/react";
import { ArrowUp, FileText, Paperclip, X } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { APP_CONFIG } from "../../app/config";
import type { EditorError } from "../../models/errors";
import { errorVariant, ERROR_VARIANT_CLASSES } from "../../models/errors";

interface Props {
  mode: string;
  magicPrompt: string;
  magicError: string | null;
  editorError: EditorError | null;
  chatContextLabel: string | null;
  upload: {
    uploadStatus: string;
    uploadProgress: number | null;
    parseElapsed: number;
    fileChips: Array<{ name: string; status: string }>;
    files: Array<unknown>;
    setFiles: (updater: (prev: Array<unknown>) => Array<unknown>) => void;
    setParsedFiles: (updater: (prev: Array<unknown>) => Array<unknown>) => void;
    setUploadStatus: (status: string) => void;
    setUploadProgress: (progress: number | null) => void;
    fileInputRef: React.RefObject<HTMLInputElement | null>;
    handleUpload: (files: FileList | null) => void;
  };
  onMagicPromptChange: (value: string) => void;
  onApplyMagic: () => void;
}

export function EditorChatBar({
  mode,
  magicPrompt,
  magicError,
  editorError,
  chatContextLabel,
  upload,
  onMagicPromptChange,
  onApplyMagic,
}: Props) {
  const { t } = useLocale();
  const errVariant = editorError ? errorVariant(editorError.type) : 'danger';
  const errClasses = ERROR_VARIANT_CLASSES[errVariant];

  return (
    <div className="absolute bottom-6 left-0 right-0 z-50 flex flex-col items-center justify-end pointer-events-none">
      {magicError && (
        <div className={`pointer-events-auto mb-3 max-w-[640px] rounded-lg border px-4 py-2 text-xs shadow-sm ${errClasses}`}>
          <span className="font-semibold uppercase mr-1.5 opacity-60">[{editorError?.type ?? 'error'}]</span>
          {magicError}
        </div>
      )}
      <AnimatePresence>
        {mode === "design" && (
          <motion.div
            key="chat-bar"
            initial={{ opacity: 0, y: 16, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 12, scale: 0.97 }}
            transition={{ duration: 0.2, ease: "easeOut" }}
            className="pointer-events-auto w-full max-w-[640px] px-6 relative group"
          >
            {/* Upload progress */}
            {(upload.uploadStatus === "uploading" || upload.uploadStatus === "parsing") && (
              <div className="mb-2 flex items-center gap-3">
                <div className="flex-1 h-1.5 rounded-full bg-[var(--editor-text-muted)]/20 overflow-hidden">
                  {upload.uploadStatus === "uploading" ? (
                    <div
                      className="h-full rounded-full transition-all duration-300 ease-out bg-[var(--editor-accent)]"
                      style={{ width: `${upload.uploadProgress ?? 0}%` }}
                    />
                  ) : (
                    <div className="h-full w-full rounded-full animate-pulse bg-[var(--editor-accent)]" />
                  )}
                </div>
                <span className="text-[11px] font-medium text-[var(--editor-text-muted)] whitespace-nowrap">
                  {upload.uploadStatus === "uploading"
                    ? `${t.upload.uploading} ${upload.uploadProgress ?? 0}%`
                    : `${t.upload.parsing} ${upload.parseElapsed}s`}
                </span>
              </div>
            )}
            <div className="relative w-full bg-[var(--editor-surface)] border border-[var(--editor-border)] rounded-xl flex items-center shadow-sm transition-all focus-within:border-[var(--editor-text-muted)] focus-within:shadow-md">
              <div className="pl-4 flex items-center shrink-0">
                <button
                  type="button"
                  onClick={() => upload.fileInputRef.current?.click()}
                  className="text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors"
                  title={t.upload.uploadFile}
                >
                  <Paperclip size={18} strokeWidth={2} />
                </button>
                <input
                  type="file"
                  ref={upload.fileInputRef}
                  onChange={(event) => {
                    upload.handleUpload(event.target.files);
                    event.target.value = "";
                  }}
                  accept={APP_CONFIG.interaction.htmlFileAccept}
                  multiple
                  className="hidden"
                />
              </div>
              {/* DOM chip */}
              {chatContextLabel && (
                <div className="ml-2.5 flex max-w-[140px] items-center gap-1 rounded-md border border-[var(--editor-accent)]/20 bg-[var(--editor-accent-soft)] px-2 py-1 text-[11px] text-[var(--editor-accent)] font-mono">
                  <span className="h-1.5 w-1.5 rounded-full bg-[var(--editor-accent)] shrink-0" />
                  <span className="truncate">{chatContextLabel}</span>
                </div>
              )}
              {/* File chips */}
              {upload.fileChips.map((file, i) => (
                <div key={`${file.name}-${i}`} className="ml-1.5 flex max-w-[130px] items-center gap-1 rounded-md border border-green-500/15 bg-green-500/[0.04] px-2 py-1 text-[11px] text-[var(--editor-text-muted)] font-mono">
                  <span className={`h-1.5 w-1.5 rounded-full shrink-0 ${file.status === 'done' ? 'bg-green-500' : file.status === 'error' ? 'bg-red-500' : 'bg-[var(--editor-accent)] animate-pulse'}`} />
                  <FileText size={11} className="shrink-0" />
                  <span className="truncate font-medium text-[var(--editor-text)]">{file.name}</span>
                  <button
                    type="button"
                    className="p-0.5 rounded-full hover:opacity-60 transition-opacity shrink-0"
                    onClick={() => {
                      upload.setFiles((prev) => prev.filter((_, j) => j !== i));
                      upload.setParsedFiles((prev) => prev.filter((_, j) => j !== i));
                      if (upload.files.length === 1) {
                        upload.setUploadStatus("idle");
                        upload.setUploadProgress(null);
                      }
                    }}
                  >
                    <X size={10} />
                  </button>
                </div>
              ))}
              <input
                type="text"
                value={magicPrompt}
                onChange={(event) => onMagicPromptChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && !event.nativeEvent.isComposing) {
                    event.preventDefault();
                    onApplyMagic();
                  }
                }}
                placeholder={t.editor.magicInputPlaceholder}
                className="flex-1 bg-transparent border-none outline-none text-[var(--editor-text)] text-[15px] placeholder:text-[var(--editor-text-muted)] focus:ring-0 w-full py-3.5 pl-3"
              />
              <button
                onClick={onApplyMagic}
                className="w-8 h-8 rounded-lg bg-[var(--editor-control)] text-[var(--editor-text-muted)] flex items-center justify-center hover:bg-[var(--editor-accent)] hover:text-[var(--editor-accent-text)] hover:scale-105 transition-all mr-2 shrink-0"
              >
                <ArrowUp size={15} />
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
