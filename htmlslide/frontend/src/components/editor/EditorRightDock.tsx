import React, { useCallback, useEffect, useState } from "react";
import { motion } from "motion/react";
import type { Editor } from "grapesjs";
import { X } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import * as api from "../../services/editorApi";
import { detectComponentType } from "../../services/editor/componentDetection";
import { InspectorPanel } from "./inspector/InspectorPanel";
import { TraitsPanel } from "./inspector/TraitsPanel";
import { LayerPanel } from "./inspector/LayerPanel";
import { BlocksPanel } from "./inspector/BlocksPanel";
import { ImageReplaceSection } from "./inspector/ImageReplaceSection";

type TabId = "style" | "traits" | "layers" | "blocks";

type Props = {
  isOpen: boolean;
  onClose: () => void;
  editor: Editor | null;
};


export function EditorRightDock({ onClose, editor }: Props) {
  const { t } = useLocale();
  const [activeTab, setActiveTab] = useState<TabId>("style");
  const [componentType, setComponentType] = useState<"text" | "image" | null>(null);
  const [imageSrc, setImageSrc] = useState("");
  const [imageError, setImageError] = useState<string | null>(null);

  const syncSelection = useCallback(() => {
    const type = detectComponentType(editor);
    setComponentType(type);
    if (type === "image" && editor) {
      const data = api.getSelectedImageData(editor);
      setImageSrc(data?.src || "");
      setImageError(null);
    }
  }, [editor]);

  useEffect(() => {
    if (!editor) return;
    syncSelection();
    editor.on("component:selected", syncSelection);
    editor.on("component:deselected", syncSelection);
    editor.on("component:update", syncSelection);
    return () => {
      editor.off("component:selected", syncSelection);
      editor.off("component:deselected", syncSelection);
      editor.off("component:update", syncSelection);
    };
  }, [editor, syncSelection]);

  const TABS: { id: TabId; label: string }[] = [
    { id: "style", label: t.panels.style },
    { id: "traits", label: t.panels.traits },
    { id: "layers", label: t.panels.layers },
    { id: "blocks", label: t.panels.blocks },
  ];

  const handleImageReplace = () => {
    if (!editor) return;
    try {
      api.replaceSelectedImage(editor, { src: imageSrc }, t);
      setImageError(null);
    } catch (e) {
      setImageError(e instanceof Error ? e.message : t.panels.replaceFailed);
    }
  };

  const isImage = componentType === "image";

  return (
    <motion.div
      initial={{ x: "100%", opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      exit={{ x: "100%", opacity: 0 }}
      transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
      style={{
        width: 320,
        minWidth: 300,
        maxWidth: 360,
        flexShrink: 0,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
        background: "var(--inspector-bg)",
        borderLeft: "1.5px solid var(--inspector-border-strong)",
        borderRadius: "24px 0 0 24px",
        boxShadow: "none",
        position: "relative",
        zIndex: 50,
      }}
    >
      {/* Dot grid background */}
      <div
        className="inspector-dot-grid"
        style={{ position: "absolute", inset: 0, pointerEvents: "none", zIndex: 0 }}
      />

      {/* Header */}
      <div
        style={{
          position: "relative",
          zIndex: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "16px 14px 10px",
          flexShrink: 0,
        }}
      >
        <div className="inspector-tab-container">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`inspector-tab-btn${activeTab === tab.id ? " active" : ""}`}
              style={{ whiteSpace: "nowrap" }}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={onClose}
          style={{
            padding: 6,
            borderRadius: 10,
            border: "none",
            background: "transparent",
            color: "var(--inspector-text-muted)",
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            transition: "all 220ms cubic-bezier(0.22, 1, 0.36, 1)",
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.color = "var(--inspector-text)";
            e.currentTarget.style.backgroundColor = "var(--inspector-hover)";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.color = "var(--inspector-text-muted)";
            e.currentTarget.style.backgroundColor = "transparent";
          }}
        >
          <X size={15} />
        </button>
      </div>

      {/* Content */}
      <div
        className="inspector-scrollbar"
        style={{
          position: "relative",
          zIndex: 1,
          flex: 1,
          overflowY: "auto",
          padding: "0 14px 16px",
        }}
      >
        {activeTab === "style" && <InspectorPanel editor={editor} />}
        {activeTab === "traits" && <TraitsPanel editor={editor} />}
        {activeTab === "layers" && <LayerPanel editor={editor} />}
        {activeTab === "blocks" && <BlocksPanel editor={editor} />}

        {isImage && (activeTab === "style" || activeTab === "traits") && (
          <ImageReplaceSection
            imageSrc={imageSrc}
            imageError={imageError}
            onImageSrcChange={setImageSrc}
            onReplace={handleImageReplace}
          />
        )}

      </div>
    </motion.div>
  );
}
