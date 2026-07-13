import React, { useCallback, useEffect, useMemo, useState } from "react";
import type { Editor, Component } from "grapesjs";
import {
  INSPECTOR_SECTIONS,
  type InspectorProperty,
  type InspectorSection as InspectorSectionConfig,
} from "../../../config/editorFeatureMap";
import * as api from "../../../services/editorApi";
import { InspectorSection } from "./InspectorSection";
import { useLocale } from "../../../context/LocaleContext";

import { normalizeColorToHex } from "../../../utils/color";

type Props = {
  editor: Editor | null;
};

function readAllStyles(
  editor: Editor | null,
  selected: Component | null,
): Record<string, string> | null {
  if (!editor || !selected) return null;

  const inlineStyle = (selected.getStyle() || {}) as Record<string, unknown>;
  const el = selected.getEl();
  const view = el?.ownerDocument.defaultView ?? null;
  const computed = el && view ? view.getComputedStyle(el) : null;

  const result: Record<string, string> = {};

  for (const section of INSPECTOR_SECTIONS) {
    for (const prop of section.properties) {
      const key = prop.styleKey;
      const inlineVal = inlineStyle[key];
      if (typeof inlineVal === "string" && inlineVal.trim()) {
        const raw = inlineVal.trim();
        result[key] = prop.type === "color" ? normalizeColorToHex(raw) : raw;
      } else if (computed) {
        const compVal = computed.getPropertyValue(key);
        // Skip transparent/none for non-color props, but for colors accept any visible color
        if (compVal) {
          if (prop.type === "color") {
            const hex = normalizeColorToHex(compVal);
            if (hex !== "#000000" || compVal !== "rgba(0, 0, 0, 0)") {
              result[key] = hex;
            }
          } else if (compVal !== "rgba(0, 0, 0, 0)" && compVal !== "none") {
            result[key] = typeof compVal === "string" ? compVal : String(compVal);
          }
        }
      }
    }
  }

  return result;
}

function sanitizeStyleValue(
  prop: InspectorProperty,
  value: string,
): string | undefined {
  const { constraints, type } = prop;

  switch (type) {
    case "number": {
      if (constraints?.unit) {
        const withoutUnit = value.replace(/px|em|rem|%|vh|vw/g, "").trim();
        const parsed = parseFloat(withoutUnit);
        if (!Number.isFinite(parsed)) return undefined;
        let clamped = parsed;
        if (constraints.min !== undefined) clamped = Math.max(constraints.min, clamped);
        if (constraints.max !== undefined) clamped = Math.min(constraints.max, clamped);
        return `${Math.round(clamped)}${constraints.unit}`;
      }
      const parsed = parseFloat(value);
      if (!Number.isFinite(parsed)) return undefined;
      let clamped = parsed;
      if (constraints.min !== undefined) clamped = Math.max(constraints.min, clamped);
      if (constraints.max !== undefined) clamped = Math.min(constraints.max, clamped);
      return String(clamped);
    }
    case "color": {
      const v = value.trim().toLowerCase();
      if (/^#[0-9a-f]{6}$/.test(v)) return v;
      if (/^#[0-9a-f]{3}$/.test(v)) {
        const r = v[1], g = v[2], b = v[3];
        return `#${r}${r}${g}${g}${b}${b}`;
      }
      if (/^rgba?\(/.test(v)) return v;
      return v;
    }
    default:
      return value.trim();
  }
}

export function InspectorPanel({ editor }: Props) {
  const { t } = useLocale();
  const [styles, setStyles] = useState<Record<string, string> | null>(null);

  const syncStyles = useCallback(() => {
    const selected = editor?.getSelected() ?? null;
    setStyles(readAllStyles(editor, selected));
  }, [editor]);

  useEffect(() => {
    if (!editor) return;
    syncStyles();
    editor.on("component:selected", syncStyles);
    editor.on("component:deselected", syncStyles);
    editor.on("component:styleUpdate", syncStyles);
    editor.on("component:update", syncStyles);
    return () => {
      editor.off("component:selected", syncStyles);
      editor.off("component:deselected", syncStyles);
      editor.off("component:styleUpdate", syncStyles);
      editor.off("component:update", syncStyles);
    };
  }, [editor, syncStyles]);

  const handleStyleChange = useCallback(
    (key: string, value: string) => {
      if (!editor) return;

      // Find the property definition to get constraints for sanitization
      let propDef: InspectorProperty | undefined;
      for (const section of INSPECTOR_SECTIONS) {
        const found = section.properties.find((p) => p.styleKey === key);
        if (found) {
          propDef = found;
          break;
        }
      }

      const sanitized = propDef ? sanitizeStyleValue(propDef, value) : value;
      if (sanitized === undefined) return;

      api.setSelectedStyle(editor, key, sanitized);
    },
    [editor],
  );

  const hasSelection = styles !== null;

  const translatedSections = useMemo<InspectorSectionConfig[]>(() => {
    return INSPECTOR_SECTIONS.map((section) => ({
      ...section,
      label: (t.inspector.sections as Record<string, string>)[section.id] ?? section.label,
      properties: section.properties.map((prop) => ({
        ...prop,
        label: (t.inspector.properties as Record<string, string>)[prop.id] ?? prop.label,
        options: prop.options?.map((opt) => ({
          ...opt,
          label: opt.label
            ? ((t.inspector.displayOptions as Record<string, string>)[opt.value] ?? opt.label)
            : opt.label,
        })),
      })),
    }));
  }, [t]);

  if (!hasSelection) {
    return (
      <div className="inspector-empty">
        <p className="inspector-empty-text">
          {t.panels.selectElementToEdit}
        </p>
      </div>
    );
  }

  return (
    <div style={{ padding: "0 4px" }}>
      {translatedSections.map((section) => (
        <InspectorSection
          key={section.id}
          section={section}
          editor={editor}
          styles={styles}
          onStyleChange={handleStyleChange}
        />
      ))}
    </div>
  );
}
