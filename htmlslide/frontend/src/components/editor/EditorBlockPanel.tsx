import React, { useCallback } from "react";
import {
  BarChart3,
  BookOpen,
  Frame,
  Image as ImageIcon,
  Minus,
  Type,
  Video,
} from "lucide-react";
import { useEditor } from "../../hooks/useEditor";
import { useLocale } from "../../context/LocaleContext";
import { useBlockDrag } from "../../hooks/useBlockDrag";

type BlockItem = {
  blockId: string;
  icon: React.ComponentType<{ size?: number | string }>;
  label: string;
};

type Props = {
  onOpenDocuments?: () => void;
};

export function EditorBlockPanel({ onOpenDocuments }: Props) {
  const editor = useEditor();
  const { t } = useLocale();
  const { dragError, handleDragStart, handleDragEnd } = useBlockDrag({
    editor,
    onError: () => {},
  });

  const ITEMS: BlockItem[] = [
    { blockId: "div-container", icon: Frame, label: t.blocks.container },
    { blockId: "paragraph", icon: Type, label: t.blocks.paragraph },
    { blockId: "image", icon: ImageIcon, label: t.blocks.image },
    { blockId: "video", icon: Video, label: t.blocks.video },
    { blockId: "chart", icon: BarChart3, label: t.blocks.chart },
    { blockId: "divider", icon: Minus, label: t.blocks.divider },
  ];

  const onDragStart = useCallback(
    (blockId: string, e: React.DragEvent) => handleDragStart(blockId, e),
    [handleDragStart],
  );

  return (
    <div className="flex flex-col items-end gap-2">
      {dragError && (
        <div className="rounded-lg border border-red-400/30 bg-red-500/10 px-3 py-1.5 text-[11px] text-[var(--editor-danger)] shadow-lg">
          {dragError}
        </div>
      )}

      <div className="bg-[var(--editor-surface)] border border-[var(--editor-border)] p-2 rounded-xl flex flex-col gap-1.5 shadow-sm">
        {ITEMS.map((item) => (
          <button
            key={item.blockId}
            draggable
            onDragStart={(e) => onDragStart(item.blockId, e)}
            onDragEnd={handleDragEnd}
            className="relative group p-2.5 rounded-lg bg-transparent hover:bg-[var(--editor-control-hover)] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors cursor-grab active:cursor-grabbing"
          >
            <item.icon size={18} />

            <span className="absolute right-full mr-2.5 top-1/2 -translate-y-1/2 px-2 py-1 rounded-md bg-[var(--editor-surface)] border border-[var(--editor-border)] text-[11px] text-[var(--editor-text)] whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity duration-150 pointer-events-none shadow-sm">
              {item.label}
            </span>
          </button>
        ))}

        <div className="border-t border-[var(--editor-border)] mx-1" />

        <button
          onClick={onOpenDocuments}
          className="relative group p-2.5 rounded-lg bg-transparent hover:bg-[var(--editor-control-hover)] text-[var(--editor-text-muted)] hover:text-[var(--editor-accent)] transition-colors cursor-pointer"
        >
          <BookOpen size={18} />

          <span className="absolute right-full mr-2.5 top-1/2 -translate-y-1/2 px-2 py-1 rounded-md bg-[var(--editor-surface)] border border-[var(--editor-border)] text-[11px] text-[var(--editor-text)] whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity duration-150 pointer-events-none shadow-sm">
            {t.blocks.referenceDocs || "参考文档"}
          </span>
        </button>
      </div>
    </div>
  );
}
