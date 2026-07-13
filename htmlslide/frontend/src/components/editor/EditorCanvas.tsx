import React, { useEffect, useRef, useState } from "react";
import { APP_CONFIG } from "../../app/config";
import { useEditor } from "../../hooks/useEditor";
import { clearEditorSelection } from "../../services/deckAdapter";

export function EditorCanvas({ advancedOpen: _advancedOpen }: { advancedOpen?: boolean }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const config = APP_CONFIG;
  const editor = useEditor();
  const [scale, setScale] = useState(config.editor.viewport.maxScale);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const { paddingX, paddingY, width, height, maxScale } = config.editor.viewport;

    const updateScale = () => {
      const { clientWidth, clientHeight } = el;
      const scaleX = (clientWidth - paddingX) / width;
      const scaleY = (clientHeight - paddingY) / height;
      setScale(Math.max(0.1, Math.min(scaleX, scaleY, maxScale)));
    };

    updateScale();

    // ResizeObserver detects actual container size changes — including those
    // caused by AnimatePresence exit animations that state alone can't time.
    const observer = new ResizeObserver(() => updateScale());
    observer.observe(el);

    window.addEventListener("resize", updateScale);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updateScale);
    };
  }, []);

  useEffect(() => {
    if (!editor) return;
    editor.Canvas.setZoom(Math.round(scale * 100));
    editor.refresh({ tools: true });
  }, [editor, scale]);

  return (
    <div
      className="w-full h-full flex items-center justify-center overflow-hidden bg-transparent relative"
      ref={containerRef}
      onMouseDown={(event) => {
        if (!editor) return;
        const target = event.target as HTMLElement;
        // Skip if clicking on the GrapesJS iframe — those events are
        // handled inside the iframe document (bindBlankDeselect).
        if (target.tagName === "IFRAME") return;
        clearEditorSelection(editor);
      }}
      onDragOver={(event) => {
        if (editor) {
          event.preventDefault();
          event.dataTransfer.dropEffect = "copy";
        }
      }}
      onDrop={(event) => {
        event.preventDefault();
        if (!editor) return;
        const em = (editor as any).em;
        const dragSource = em?.get?.("dragSource");
        if (dragSource?.content) {
          const bm = editor.BlockManager as any;
          if (typeof bm.endDrag === "function") {
            bm.endDrag(false);
          }
        }
      }}
    >
      <div
        style={{
          width: config.editor.viewport.width,
          height: config.editor.viewport.height,
        }}
        className="flex flex-col relative shrink-0 rounded-md overflow-hidden"
      >
        <div id="gjs-canvas" className="w-full h-full" />
      </div>
    </div>
  );
}
