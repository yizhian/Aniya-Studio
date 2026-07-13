import React, { useCallback } from "react";
import type { InspectorProperty } from "../../../../config/editorFeatureMap";

type Props = {
  property: InspectorProperty;
  value: string;
  onChange: (value: string) => void;
};

export function SelectControl({ property, value, onChange }: Props) {
  const { options } = property;
  if (!options || options.length === 0) return null;

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLSelectElement>) => {
      onChange(e.target.value);
    },
    [onChange],
  );

  return (
    <select
      className="inspector-select"
      value={value}
      onChange={handleChange}
    >
      {options.map((opt) => (
        <option key={opt.value} value={opt.value}>
          {opt.label}
        </option>
      ))}
    </select>
  );
}
