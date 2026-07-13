import React, { useCallback, useState } from "react";
import { ChevronDown } from "lucide-react";
import type { Editor } from "grapesjs";
import type { InspectorSection as SectionConfig, InspectorProperty } from "../../../config/editorFeatureMap";
import { FontFamilyControl } from "./controls/FontFamilyControl";
import { NumberControl } from "./controls/NumberControl";
import { ColorControl } from "./controls/ColorControl";
import { SegmentedControl } from "./controls/SegmentedControl";
import { SelectControl } from "./controls/SelectControl";

type Props = {
  section: SectionConfig;
  editor: Editor | null;
  styles: Record<string, string> | null;
  onStyleChange: (key: string, value: string) => void;
};

export function InspectorSection({
  section,
  editor,
  styles,
  onStyleChange,
}: Props) {
  const [collapsed, setCollapsed] = useState(false);
  const Icon = section.icon;

  const toggleCollapse = useCallback(() => {
    setCollapsed((p) => !p);
  }, []);

  const handlePropertyChange = useCallback(
    (prop: InspectorProperty, value: string) => {
      onStyleChange(prop.styleKey, value);
    },
    [onStyleChange],
  );

  return (
    <div>
      <button
        type="button"
        className="inspector-section-header"
        onClick={toggleCollapse}
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          padding: "8px 0",
          background: "none",
          border: "none",
          width: "100%",
        }}
      >
        <Icon size={13} style={{ color: "var(--inspector-section-title)" }} />
        <span>{section.label}</span>
        <ChevronDown
          size={12}
          className={`inspector-chevron ${collapsed ? "closed" : "open"}`}
          style={{
            color: "var(--inspector-text-muted)",
            marginLeft: "auto",
          }}
        />
      </button>
      <div
        className={`inspector-section-content ${collapsed ? "collapsed" : "expanded"}`}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 10, paddingBottom: 8 }}>
          {section.properties.map((prop) => {
            const value = styles?.[prop.styleKey] ?? "";
            const onChange = (v: string) => handlePropertyChange(prop, v);

            const Icon = prop.icon;
            return (
              <div key={prop.id} className="inspector-property-row">
                <label className="inspector-property-label" style={{ display: "flex", alignItems: "center", gap: 5 }}>
                  {Icon && <Icon size={12} style={{ color: "var(--inspector-text-muted)", flexShrink: 0 }} />}
                  {prop.label}
                </label>
                <div className="inspector-property-control">
                  {renderControl(prop, value, onChange, editor)}
                </div>
              </div>
            );
          })}
        </div>
      </div>
      <div className="inspector-divider" />
    </div>
  );
}

function renderControl(
  prop: InspectorProperty,
  value: string,
  onChange: (value: string) => void,
  editor: Editor | null,
): React.ReactNode {
  switch (prop.type) {
    case "fontFamily":
      return (
        <FontFamilyControl
          editor={editor}
          property={prop}
          value={value}
          onChange={onChange}
        />
      );
    case "number":
      return (
        <NumberControl property={prop} value={value} onChange={onChange} />
      );
    case "color":
      return (
        <ColorControl property={prop} value={value} onChange={onChange} />
      );
    case "segmented":
      return (
        <SegmentedControl
          property={prop}
          value={value}
          onChange={onChange}
        />
      );
    case "select":
      return (
        <SelectControl property={prop} value={value} onChange={onChange} />
      );
    default:
      return null;
  }
}
