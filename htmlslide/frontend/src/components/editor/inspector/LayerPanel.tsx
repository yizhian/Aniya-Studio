import React, { useCallback, useEffect, useState } from "react";
import type { Component, Editor } from "grapesjs";
import {
  Type,
  ImageIcon,
  Code2,
  Square,
  Columns2,
  Minus,
  Film,
  BarChart3,
  MousePointerClick,
  ChevronRight,
  GripVertical,
} from "lucide-react";
import { useLocale } from "../../../context/LocaleContext";

type LayerNode = {
  component: Component;
  children: LayerNode[];
  depth: number;
};

type Props = {
  editor: Editor | null;
};

function getIcon(component: Component): React.ReactNode {
  const type = component.get("type");
  const tagName = component.get("tagName") || component.getEl()?.tagName?.toLowerCase();
  const iconSize = 14;

  if (tagName === "img" || type === "image") return <ImageIcon size={iconSize} />;
  if (type === "custom-code") return <Code2 size={iconSize} />;
  if (tagName === "h1" || tagName === "h2" || tagName === "h3") return <Type size={iconSize} />;
  if (tagName === "p" || type === "text") return <Type size={iconSize} />;
  if (tagName === "button") return <MousePointerClick size={iconSize} />;
  if (tagName === "hr") return <Minus size={iconSize} />;
  if (tagName === "iframe" || type === "video") return <Film size={iconSize} />;
  if (tagName === "blockquote") return <Type size={iconSize} />;
  if (component.get("classes")?.includes?.("chart") || type === "chart") return <BarChart3 size={iconSize} />;
  const children = component.components?.() || [];
  if (children.length > 0) {
    // Check if it has columns (flex children)
    const style = (component.getStyle() || {}) as Record<string, unknown>;
    if (style.display === "flex" || style.display === "grid") return <Columns2 size={iconSize} />;
  }
  return <Square size={iconSize} />;
}

function getLabel(component: Component, t: ReturnType<typeof useLocale>["t"]): string {
  const tagName = component.get("tagName") || component.getEl()?.tagName?.toLowerCase() || "div";
  const type = component.get("type");

  if (tagName === "img" || type === "image") return t.panels.image;
  if (type === "custom-code") return t.panels.customCode;
  if (tagName === "h1") return t.panels.h1Heading;
  if (tagName === "h2") return t.panels.h2Heading;
  if (tagName === "p" || type === "text") {
    const el = component.getEl();
    const text = el?.textContent?.trim().substring(0, 24) || "";
    return text ? `"${text}"` : t.elementLabels.paragraph;
  }
  if (tagName === "button") return t.elementLabels.button;
  if (tagName === "hr") return t.elementLabels.divider;
  if (tagName === "iframe" || type === "video") return t.elementLabels.video;
  if (tagName === "blockquote") return t.panels.blockquote;
  return tagName;
}

function buildTree(component: Component, depth: number): LayerNode {
  const children = (component.components?.() || []).map((c: Component) =>
    buildTree(c, depth + 1),
  );
  return { component, children, depth };
}

export function LayerPanel({ editor }: Props) {
  const { t } = useLocale();
  const [tree, setTree] = useState<LayerNode[]>([]);

  const syncTree = useCallback(() => {
    if (!editor) return;
    const wrapper = editor.getWrapper();
    if (!wrapper) return;
    // Only show layers from the active slide
    const activeSlide = wrapper.find?.(".slide.active")?.[0] as Component | undefined;
    if (activeSlide) {
      const children = activeSlide.components?.() || [];
      setTree(children.map((c: Component) => buildTree(c, 0)));
    } else {
      // Fallback: no .slide.active, show wrapper children
      const children = wrapper.components?.() || [];
      setTree(children.map((c: Component) => buildTree(c, 0)));
    }
  }, [editor]);

  useEffect(() => {
    if (!editor) return;
    syncTree();
    editor.on("component:add", syncTree);
    editor.on("component:remove", syncTree);
    editor.on("component:update", syncTree);
    editor.on("load", syncTree);
    return () => {
      editor.off("component:add", syncTree);
      editor.off("component:remove", syncTree);
      editor.off("component:update", syncTree);
      editor.off("load", syncTree);
    };
  }, [editor, syncTree]);

  const handleSelect = useCallback(
    (component: Component) => {
      editor?.select(component);
    },
    [editor],
  );

  const selectedId = editor?.getSelected()?.getId() ?? null;

  if (tree.length === 0) {
    return (
      <div className="inspector-empty">
        <p className="inspector-empty-text">{t.panels.noLayers}</p>
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 1, padding: "4px 0" }}>
      {tree.map((node) => (
        <LayerItem
          key={node.component.getId()}
          node={node}
          selectedId={selectedId}
          onSelect={handleSelect}
          t={t}
        />
      ))}
    </div>
  );
}

function LayerItem({
  node,
  selectedId,
  onSelect,
  t,
}: {
  node: LayerNode;
  selectedId: string | null;
  onSelect: (c: Component) => void;
  t: ReturnType<typeof useLocale>["t"];
}) {
  const isSelected = node.component.getId() === selectedId;
  const hasChildren = node.children.length > 0;
  const [collapsed, setCollapsed] = useState(false);

  return (
    <>
      <div
        className={`inspector-layer-item${isSelected ? " selected" : ""}`}
        style={{
          paddingLeft: 6 + node.depth * 14,
          display: "flex",
          alignItems: "center",
          gap: 6,
        }}
        onClick={() => onSelect(node.component)}
      >
        {/* Collapse toggle for parents */}
        <span
          style={{
            width: 16,
            height: 16,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
            visibility: hasChildren ? "visible" : "hidden",
          }}
          onClick={(e) => {
            e.stopPropagation();
            setCollapsed((p) => !p);
          }}
        >
          <ChevronRight
            size={12}
            style={{
              color: "var(--inspector-text-muted)",
              transition: "transform 180ms ease",
              transform: collapsed ? "rotate(0deg)" : "rotate(90deg)",
            }}
          />
        </span>
        {/* No-collapse spacer for leaf nodes */}
        {!hasChildren && <span style={{ width: 16, flexShrink: 0 }} />}

        {/* Icon */}
        <span
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            width: 24,
            height: 24,
            borderRadius: 6,
            flexShrink: 0,
            color: isSelected ? "var(--inspector-text)" : "var(--inspector-text-muted)",
            backgroundColor: isSelected ? "var(--inspector-hover)" : "transparent",
            transition: "all 120ms ease",
          }}
        >
          {getIcon(node.component)}
        </span>

        {/* Label */}
        <span
          style={{
            flex: 1,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            fontSize: 12,
            fontWeight: isSelected ? 600 : 500,
            color: isSelected ? "var(--inspector-text)" : "var(--inspector-text-secondary)",
          }}
        >
          {getLabel(node.component, t)}
        </span>

        {/* Child count badge */}
        {hasChildren && (
          <span
            style={{
              fontSize: 10,
              fontWeight: 600,
              color: "var(--inspector-text-muted)",
              backgroundColor: "var(--inspector-control-bg)",
              borderRadius: 4,
              padding: "1px 5px",
              flexShrink: 0,
            }}
          >
            {node.children.length}
          </span>
        )}
      </div>

      {/* Children (recursive) */}
      {!collapsed &&
        node.children.map((child) => (
          <LayerItem
            key={child.component.getId()}
            node={child}
            selectedId={selectedId}
            onSelect={onSelect}
            t={t}
          />
        ))}
    </>
  );
}
