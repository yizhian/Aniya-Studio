import { useCallback, useState } from "react";
import type { Editor } from "grapesjs";

interface UseBlockDragOptions {
  editor: Editor | null;
  onError?: (message: string) => void;
}

export function useBlockDrag({ editor, onError }: UseBlockDragOptions) {
  const [dragError, setDragError] = useState<string | null>(null);

  const handleDragStart = useCallback(
    (blockId: string, e: React.DragEvent) => {
      if (!editor) {
        setDragError("Editor not initialized");
        e.preventDefault();
        onError?.("Editor not initialized");
        return;
      }
      const block = editor.BlockManager.get(blockId);
      if (!block) {
        setDragError("Component not found");
        e.preventDefault();
        onError?.("Component not found");
        return;
      }
      try {
        setDragError(null);
        const dt = e.dataTransfer;
        dt.effectAllowed = "copy";
        dt.setData("text/plain", blockId);
        editor.BlockManager.startDrag(block, e.nativeEvent);
      } catch {
        setDragError("Drag failed");
        e.preventDefault();
        onError?.("Drag failed");
      }
    },
    [editor, onError],
  );

  const handleDragEnd = useCallback(() => {
    setDragError(null);
    editor?.BlockManager.endDrag(true);
  }, [editor]);

  return { dragError, handleDragStart, handleDragEnd, setDragError };
}
