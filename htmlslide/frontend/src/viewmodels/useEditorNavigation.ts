import { useCallback, useEffect, useState } from "react";
import type { Editor } from "grapesjs";
import type { DeckState } from "../services/deckAdapter";

interface UseEditorNavigationOptions {
  editor: Editor | null;
  isDirtyRef: React.MutableRefObject<boolean>;
  checkAndUpdateDirty: (ed: Editor | null) => void;
  onSaveAndProceed: (mode: "navigate" | "rollback") => Promise<boolean>;
  onDiscardNavigate: () => void;
  onDiscardRollback: () => void;
  goToPage: (nextIndex: number) => void;
  currentPageIndex: number;
  deckState: DeckState | null;
  imageDialogOpen: boolean;
  previewUrl: string | null;
}

export function useEditorNavigation({
  editor,
  isDirtyRef,
  checkAndUpdateDirty,
  onSaveAndProceed,
  onDiscardNavigate,
  onDiscardRollback,
  goToPage,
  currentPageIndex,
  deckState,
  imageDialogOpen,
  previewUrl,
}: UseEditorNavigationOptions) {
  const [unsavedDialog, setUnsavedDialog] = useState<{
    open: boolean;
    mode: "navigate" | "rollback";
    onProceed: () => void;
  }>({ open: false, mode: "navigate", onProceed: () => {} });

  const requestNavigation = useCallback((proceedAction: () => void) => {
    checkAndUpdateDirty(editor);
    if (isDirtyRef.current) {
      setUnsavedDialog({ open: true, mode: "navigate", onProceed: proceedAction });
    } else {
      proceedAction();
    }
  }, [editor, checkAndUpdateDirty, isDirtyRef]);

  const requestRollback = useCallback((proceedAction: () => void) => {
    checkAndUpdateDirty(editor);
    if (isDirtyRef.current) {
      setUnsavedDialog({ open: true, mode: "rollback", onProceed: proceedAction });
    } else {
      proceedAction();
    }
  }, [editor, checkAndUpdateDirty, isDirtyRef]);

  const handleSaveAndProceed = useCallback(async () => {
    const proceed = unsavedDialog.onProceed;
    const ok = await onSaveAndProceed(unsavedDialog.mode);
    if (!ok) return;
    setUnsavedDialog({ open: false, mode: "navigate", onProceed: () => {} });
    proceed();
  }, [unsavedDialog, onSaveAndProceed]);

  const handleDiscard = useCallback(() => {
    const proceed = unsavedDialog.onProceed;
    if (unsavedDialog.mode === "navigate") {
      onDiscardNavigate();
    } else {
      onDiscardRollback();
    }
    setUnsavedDialog({ open: false, mode: "navigate", onProceed: () => {} });
    proceed();
  }, [unsavedDialog, onDiscardNavigate, onDiscardRollback]);

  const handleCancelNavigation = useCallback(() => {
    setUnsavedDialog({ open: false, mode: "navigate", onProceed: () => {} });
  }, []);

  // Keyboard navigation (ArrowLeft/ArrowRight)
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;

      const target = event.target as HTMLElement | null;
      const tagName = target?.tagName.toLowerCase();

      if (tagName === "input" || tagName === "textarea") return;
      if (target?.isContentEditable) return;

      if (imageDialogOpen || previewUrl) return;
      if (document.querySelector('[role="dialog"], [role="alertdialog"]')) return;

      const iframe = document.querySelector("#gjs-canvas iframe") as HTMLIFrameElement | null;
      const iframeDoc = iframe?.contentDocument;
      if (iframeDoc) {
        const ae = iframeDoc.activeElement;
        if (
          ae &&
          (ae.tagName === "INPUT" ||
            ae.tagName === "TEXTAREA" ||
            (ae as HTMLElement).isContentEditable)
        ) {
          return;
        }
      }

      if (event.key === "ArrowLeft") {
        event.preventDefault();
        goToPage((deckState?.current ?? currentPageIndex) - 1);
      }
      if (event.key === "ArrowRight") {
        event.preventDefault();
        goToPage((deckState?.current ?? currentPageIndex) + 1);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [currentPageIndex, deckState, goToPage, imageDialogOpen, previewUrl]);

  // beforeunload warning
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (isDirtyRef.current) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [isDirtyRef]);

  // Blob URL cleanup
  useEffect(() => {
    return () => {
      if (previewUrl) URL.revokeObjectURL(previewUrl);
    };
  }, [previewUrl]);

  return {
    unsavedDialog,
    requestNavigation,
    requestRollback,
    handleSaveAndProceed,
    handleDiscard,
    handleCancelNavigation,
  };
}
