import React, { useCallback, useEffect, useState } from "react";
import type { Component, Editor } from "grapesjs";
import { Tag, Hash, Layers } from "lucide-react";
import { useLocale } from "../../../context/LocaleContext";

type Props = {
  editor: Editor | null;
};

type TraitField = {
  key: string;
  label: string;
  value: string;
  icon: React.ReactNode;
  editable: boolean;
};

function readTraits(component: Component | null, t: ReturnType<typeof useLocale>["t"]): TraitField[] {
  if (!component) return [];
  const el = component.getEl();
  const tagName = el?.tagName.toLowerCase() ?? component.get("tagName") ?? "div";
  const id = component.getId() || "";

  const fields: TraitField[] = [
    {
      key: "tagName",
      label: t.panels.tag,
      value: tagName,
      icon: <Tag size={13} />,
      editable: false,
    },
    {
      key: "id",
      label: t.panels.id,
      value: id,
      icon: <Hash size={13} />,
      editable: false,
    },
  ];

  // Classes
  const classes = component.get("classes");
  if (classes) {
    const clsStr = Array.isArray(classes)
      ? classes.map((c: any) => (typeof c === "string" ? c : c?.get?.()?.name ?? "")).filter(Boolean).join(" ")
      : String(classes);
    fields.push({
      key: "classes",
      label: t.panels.classes,
      value: clsStr,
      icon: <Layers size={13} />,
      editable: true,
    });
  }

  // Image-specific traits
  if (tagName === "img") {
    const attrs = component.getAttributes();
    fields.push(
      {
        key: "src",
        label: t.panels.source,
        value: attrs.src || "",
        icon: <Tag size={13} />,
        editable: true,
      },
      {
        key: "alt",
        label: t.panels.alt,
        value: attrs.alt || "",
        icon: <Tag size={13} />,
        editable: true,
      },
    );
  }

  // Text content for text components
  if (component.get("type") === "text") {
    const content = component.get("content") || el?.textContent || "";
    fields.push({
      key: "content",
      label: t.panels.content,
      value: typeof content === "string" ? content : String(content),
      icon: <Tag size={13} />,
      editable: true,
    });
  }

  // Component type
  fields.push({
    key: "type",
    label: t.panels.type,
    value: component.get("type") || "default",
    icon: <Tag size={13} />,
    editable: false,
  });

  return fields;
}

export function TraitsPanel({ editor }: Props) {
  const { t } = useLocale();
  const [fields, setFields] = useState<TraitField[]>([]);

  const sync = useCallback(() => {
    const selected = editor?.getSelected() ?? null;
    setFields(readTraits(selected, t));
  }, [editor, t]);

  useEffect(() => {
    if (!editor) return;
    sync();
    editor.on("component:selected", sync);
    editor.on("component:deselected", sync);
    editor.on("component:update", sync);
    return () => {
      editor.off("component:selected", sync);
      editor.off("component:deselected", sync);
      editor.off("component:update", sync);
    };
  }, [editor, sync]);

  const handleChange = useCallback(
    (field: TraitField, nextValue: string) => {
      if (!editor) return;
      const selected = editor.getSelected();
      if (!selected) return;

      if (field.key === "classes") {
        selected.set("classes", nextValue.split(/\s+/).filter(Boolean));
      } else if (field.key === "src" || field.key === "alt") {
        const attrs = selected.getAttributes();
        selected.addAttributes({ ...attrs, [field.key]: nextValue });
      } else if (field.key === "content") {
        selected.set("content", nextValue);
        // Also update the DOM element text
        const el = selected.getEl();
        if (el && el.textContent !== undefined) {
          el.textContent = nextValue;
        }
      }
    },
    [editor],
  );

  if (fields.length === 0) {
    return (
      <div className="inspector-empty">
        <p className="inspector-empty-text">{t.panels.selectElementToView}</p>
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 2, padding: "4px 0" }}>
      {fields.map((field) => (
        <div
          key={field.key}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 10,
            padding: "8px 10px",
            borderRadius: 8,
            transition: "background-color 120ms ease",
          }}
          onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = "var(--inspector-hover)")}
          onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = "transparent")}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 6,
              minWidth: 80,
              flexShrink: 0,
            }}
          >
            <span style={{ color: "var(--inspector-text-muted)", display: "flex" }}>
              {field.icon}
            </span>
            <span
              style={{
                fontSize: 11,
                fontWeight: 700,
                color: "var(--inspector-section-title)",
                textTransform: "uppercase",
                letterSpacing: "0.08em",
              }}
            >
              {field.label}
            </span>
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            {field.editable ? (
              <input
                type="text"
                value={field.value}
                onChange={(e) => {
                  const newValue = e.target.value;
                  setFields((prev) =>
                    prev.map((f) =>
                      f.key === field.key ? { ...f, value: newValue } : f,
                    ),
                  );
                }}
                onBlur={(e) => handleChange(field, e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.nativeEvent.isComposing) handleChange(field, (e.target as HTMLInputElement).value);
                }}
                spellCheck={false}
                style={{
                  width: "100%",
                  height: 32,
                  background: "var(--inspector-control-bg)",
                  border: "1px solid var(--inspector-control-border)",
                  borderRadius: 8,
                  padding: "0 10px",
                  fontSize: 13,
                  fontWeight: 500,
                  fontFamily: "'Inter', system-ui, sans-serif",
                  color: "var(--inspector-text)",
                  outline: "none",
                  transition: "border-color 220ms cubic-bezier(0.22, 1, 0.36, 1)",
                }}
                onFocus={(e) => (e.target.style.borderColor = "var(--inspector-control-focus)")}
                onBlurCapture={(e) => (e.target.style.borderColor = "var(--inspector-control-border)")}
              />
            ) : (
              <span
                style={{
                  fontSize: 12,
                  fontWeight: 500,
                  color: "var(--inspector-text-muted)",
                  fontFamily: "'SF Mono', 'Fira Code', monospace",
                  padding: "4px 0",
                  display: "block",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {field.value || "—"}
              </span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
