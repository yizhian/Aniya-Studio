import { useState, useEffect } from "react";
import { X } from "lucide-react";
import { motion, AnimatePresence } from "motion/react";
import type { ParsedFile } from "../../services/upload";
import { detectFileType, fileTypeIcon, fileUrl } from "./viewers/utils";
import { PdfViewer } from "./viewers/PdfViewer";
import { DocxViewer } from "./viewers/DocxViewer";
import { ImageViewer } from "./viewers/ImageViewer";
import { TextFileViewer } from "./viewers/TextFileViewer";
import { MarkdownViewer } from "./viewers/MarkdownViewer";
import { Portal } from "../common/Portal";
import { Z_INDEX } from "../../constants/zIndex";

type ViewerDoc = ParsedFile;

type Props = {
  isOpen: boolean;
  onClose: () => void;
  documents: ViewerDoc[];
  projectId: string;
  agentgoBaseUrl: string;
};

export function DocumentViewerModal({ isOpen, onClose, documents, projectId, agentgoBaseUrl }: Props) {
  const [selectedIdx, setSelectedIdx] = useState(0);

  useEffect(() => {
    if (isOpen && documents.length > 0) {
      setSelectedIdx(0);
    }
  }, [isOpen, documents]);

  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    if (isOpen) {
      document.addEventListener("keydown", handleKey);
      return () => document.removeEventListener("keydown", handleKey);
    }
  }, [isOpen, onClose]);

  const selected = documents[selectedIdx];

  const renderContent = () => {
    if (!selected) {
      return (
        <div className="flex items-center justify-center h-full text-[var(--editor-text-muted)] text-sm">
          No documents
        </div>
      );
    }
    const ft = detectFileType(selected);
    const url = fileUrl(agentgoBaseUrl, projectId, selected);
    const noOriginal = !selected.original_path_rel && (ft === "pdf" || ft === "docx");
    const effectiveType = noOriginal ? "text" : ft;
    switch (effectiveType) {
      case "pdf":
        return <PdfViewer url={url} />;
      case "docx":
        return <DocxViewer url={url} />;
      case "image":
        return <ImageViewer url={url} alt={selected.original_name} />;
      case "markdown":
        return <MarkdownViewer url={url} />;
      case "text":
        return <TextFileViewer url={url} />;
      default:
        return (
          <div className="flex items-center justify-center h-full text-[var(--editor-text-muted)] text-sm">
            Preview not available for this file type.
          </div>
        );
    }
  };

  return (
    <Portal>
    <AnimatePresence>
      {isOpen && (
        <motion.div
          className="fixed inset-0 flex items-center justify-center"
          style={{ zIndex: Z_INDEX.OVERLAY }}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          <div className="absolute inset-0 bg-black/50" onClick={onClose} />

          <motion.div
            className="relative z-10 w-full max-w-5xl h-[85vh] bg-[var(--editor-surface)] border border-[var(--editor-border)] rounded-2xl shadow-2xl flex flex-col overflow-hidden"
            initial={{ opacity: 0, scale: 0.96, y: 12 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 12 }}
            transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
          >
            <div className="flex items-center justify-between px-5 py-3 border-b border-[var(--editor-border)] shrink-0">
              <h2 className="text-sm font-medium text-[var(--editor-text)]">
                {selected ? selected.original_name : "Documents"}
              </h2>
              <button
                onClick={onClose}
                className="p-1.5 rounded-lg hover:bg-[var(--editor-control-hover)] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors"
              >
                <X size={16} />
              </button>
            </div>

            <div className="flex flex-1 overflow-hidden">
              <div className="w-56 shrink-0 border-r border-[var(--editor-border)] overflow-y-auto p-2 flex flex-col gap-0.5">
                {documents.map((doc, idx) => (
                  <button
                    key={doc.original_name}
                    onClick={() => setSelectedIdx(idx)}
                    className={`flex items-center gap-2 px-3 py-2 rounded-lg text-left text-sm transition-colors ${
                      idx === selectedIdx
                        ? "bg-[var(--editor-control-hover)] text-[var(--editor-text)]"
                        : "text-[var(--editor-text-muted)] hover:bg-[var(--editor-control-hover)] hover:text-[var(--editor-text)]"
                    }`}
                  >
                    {fileTypeIcon(doc)}
                    <span className="truncate flex-1">{doc.original_name}</span>
                  </button>
                ))}
                {documents.length === 0 && (
                  <p className="text-xs text-[var(--editor-text-muted)] px-3 py-4 text-center">
                    No documents uploaded yet.
                  </p>
                )}
              </div>

              <div className="flex-1 overflow-y-auto">
                {renderContent()}
              </div>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
    </Portal>
  );
}
