import type { InspectorProperty } from "../../../../config/editorFeatureMap";
import { normalizeColorToHex } from "../../../../utils/color";
import { ColorPicker } from "../../../common/ColorPicker";

type Props = {
  property: InspectorProperty;
  value: string;
  onChange: (value: string) => void;
};

export function ColorControl({ value, onChange }: Props) {
  const hex = normalizeColorToHex(value);

  return (
    <ColorPicker
      color={hex}
      onChange={onChange}
      className="inspector-color-swatch shrink-0 h-7 w-9"
    />
  );
}
