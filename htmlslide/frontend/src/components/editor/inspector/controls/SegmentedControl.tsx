import React, { useCallback } from "react";
import type { InspectorProperty } from "../../../../config/editorFeatureMap";

type Props = {
  property: InspectorProperty;
  value: string;
  onChange: (value: string) => void;
};

export function SegmentedControl({ property, value, onChange }: Props) {
  const { options } = property;
  if (!options || options.length === 0) return null;

  const handleClick = useCallback(
    (optValue: string) => {
      onChange(optValue);
    },
    [onChange],
  );

  return (
    <div className="inspector-segmented">
      {options.map((opt) => {
        const isActive = value.toLowerCase() === opt.value.toLowerCase();
        const Icon = opt.icon;
        return (
          <button
            key={opt.value}
            type="button"
            className={`inspector-segmented-btn${isActive ? " active" : ""}`}
            onClick={() => handleClick(opt.value)}
            title={opt.label || opt.value}
          >
            {Icon ? <Icon size={13} /> : opt.label}
          </button>
        );
      })}
    </div>
  );
}
