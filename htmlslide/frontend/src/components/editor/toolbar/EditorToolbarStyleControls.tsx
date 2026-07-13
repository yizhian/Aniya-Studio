import type { Editor } from "grapesjs";
import { EDITOR_TOOLBAR_STYLE_CONTROLS } from "./config";
import type { SelectedToolbarStyles } from "./types";
import { StylePxControl } from "./controls/StylePxControl";
import { FontFamilyControl } from "./controls/FontFamilyControl";
import { ColorSwatchControl } from "./controls/ColorControl";
import { ToggleBoldControl } from "./controls/ToggleBoldControl";
import { ToggleItalicControl } from "./controls/ToggleItalicControl";
import { AlignControl } from "./controls/AlignControl";

interface Props {
  editor: Editor | null;
  styles: SelectedToolbarStyles | null;
  onAfterAction: () => void;
}

export function EditorToolbarStyleControls({ editor, styles, onAfterAction }: Props) {
  const disabled = !editor || !styles;

  return (
    <>
      {EDITOR_TOOLBAR_STYLE_CONTROLS.map((control, index) => {
        switch (control.type) {
          case "stylePx":
            return (
              <StylePxControl
                key={control.id}
                control={control}
                editor={editor}
                styles={styles}
                disabled={disabled}
                index={index}
                onAfterAction={onAfterAction}
              />
            );

          case "fontFamily":
            return (
              <FontFamilyControl
                key={control.id}
                control={control}
                editor={editor}
                styles={styles}
                disabled={disabled}
                index={index}
                onAfterAction={onAfterAction}
              />
            );

          case "styleColor":
            return (
              <ColorSwatchControl
                key={control.id}
                control={control}
                editor={editor}
                styles={styles}
                disabled={disabled}
                index={index}
                onAfterAction={onAfterAction}
              />
            );

          case "toggleBold":
            return (
              <ToggleBoldControl
                key={control.id}
                control={control}
                editor={editor}
                styles={styles}
                disabled={disabled}
                index={index}
                onAfterAction={onAfterAction}
              />
            );

          case "toggleItalic":
            return (
              <ToggleItalicControl
                key={control.id}
                control={control}
                editor={editor}
                styles={styles}
                disabled={disabled}
                index={index}
                onAfterAction={onAfterAction}
              />
            );

          case "align":
            return (
              <AlignControl
                key={control.id}
                control={control}
                editor={editor}
                styles={styles}
                disabled={disabled}
                index={index}
                onAfterAction={onAfterAction}
              />
            );

          default:
            return null;
        }
      })}
    </>
  );
}
