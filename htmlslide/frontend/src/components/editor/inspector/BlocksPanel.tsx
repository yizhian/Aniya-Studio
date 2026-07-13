import React, { useCallback, useEffect, useMemo, useState } from "react";
import type { Editor } from "grapesjs";
import { GripVertical } from "lucide-react";
import { useLocale } from "../../../context/LocaleContext";
import { useBlockDrag } from "../../../hooks/useBlockDrag";

type BlockEntry = {
  id: string;
  label: string;
  category: string;
  media: string;
};

type Props = {
  editor: Editor | null;
};

function getBlocks(editor: Editor): BlockEntry[] {
  const raw = editor.BlockManager.getAll();
  // getAll may return a Backbone Collection (array-like with .models) or a plain object/dict
  const models: any[] = Array.isArray(raw)
    ? raw
    : raw?.models ?? (typeof raw === "object" ? Object.values(raw) : []);
  return models.map((b: any) => {
    const getId = (): string => {
      if (typeof b?.get === "function") {
        const v = b.get("id");
        return typeof v === "string" ? v : String(v ?? "");
      }
      if (typeof b?.id === "string") return b.id;
      return "";
    };
    const getStr = (key: string): string => {
      if (typeof b?.get === "function") {
        const v = b.get(key);
        return typeof v === "string" ? v : "";
      }
      return typeof b?.[key] === "string" ? b[key] : "";
    };
    return {
      id: getId(),
      label: getStr("label"),
      category: getStr("category") || "General",
      media: getStr("media"),
    };
  });
}

export function BlocksPanel({ editor }: Props) {
  const { t } = useLocale();
  const [, setTick] = useState(0);

  const refresh = useCallback(() => setTick((n) => n + 1), []);

  useEffect(() => {
    if (!editor) return;
    editor.on("load", refresh);
    return () => {
      editor.off("load", refresh);
    };
  }, [editor, refresh]);

  const blocks = useMemo(() => (editor ? getBlocks(editor) : []), [editor, t]);

  // Group by category
  const categories = useMemo(() => {
    const map = new Map<string, BlockEntry[]>();
    for (const b of blocks) {
      const list = map.get(b.category) || [];
      list.push(b);
      map.set(b.category, list);
    }
    return Array.from(map.entries());
  }, [blocks]);

  const { handleDragStart, handleDragEnd } = useBlockDrag({ editor });

  const onDragStart = useCallback(
    (blockId: string, e: React.DragEvent<HTMLButtonElement>) => handleDragStart(blockId, e),
    [handleDragStart],
  );

  if (blocks.length === 0) {
    return (
      <div className="inspector-empty">
        <p className="inspector-empty-text">{t.panels.noComponents}</p>
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16, padding: "4px 0" }}>
      {categories.map(([category, items]) => (
        <div key={category}>
          <div
            style={{
              fontSize: 11,
              fontWeight: 700,
              color: "var(--inspector-section-title)",
              textTransform: "uppercase",
              letterSpacing: "0.14em",
              padding: "4px 8px",
              marginBottom: 6,
            }}
          >
            {category}
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {items.map((block) => (
              <button
                key={block.id}
                type="button"
                draggable
                onDragStart={(e) => onDragStart(block.id, e)}
                onDragEnd={handleDragEnd}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 10,
                  padding: "8px 10px",
                  borderRadius: 10,
                  border: "none",
                  background: "transparent",
                  color: "var(--inspector-text)",
                  cursor: "grab",
                  fontSize: 12,
                  fontWeight: 500,
                  fontFamily: "'Inter', system-ui, sans-serif",
                  textAlign: "left",
                  width: "100%",
                  transition: "background-color 120ms ease",
                }}
                onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = "var(--inspector-hover)")}
                onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = "transparent")}
                onMouseDown={(e) => (e.currentTarget.style.cursor = "grabbing")}
                onMouseUp={(e) => (e.currentTarget.style.cursor = "grab")}
              >
                {typeof block.media === "string" && block.media ? (
                  <span
                    className="inspector-block-icon"
                    dangerouslySetInnerHTML={{ __html: block.media }}
                    style={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      width: 36,
                      height: 36,
                      flexShrink: 0,
                      borderRadius: 8,
                      overflow: "hidden",
                    }}
                  />
                ) : (
                  <GripVertical size={16} style={{ color: "var(--inspector-text-muted)", flexShrink: 0 }} />
                )}
                <span
                  style={{
                    flex: 1,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {block.label}
                </span>
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
