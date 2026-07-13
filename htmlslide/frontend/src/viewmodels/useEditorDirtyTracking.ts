import { useCallback, useRef, useState } from "react";
import type { Editor } from "grapesjs";

export function useEditorDirtyTracking() {
  const isDirtyRef = useRef(false);
  const lastSavedHtmlRef = useRef("");
  const [dirtyTick, setDirtyTick] = useState(0);

  const markDirty = useCallback(() => {
    if (!isDirtyRef.current) {
      isDirtyRef.current = true;
      setDirtyTick((tick) => tick + 1);
    }
  }, []);

  const captureCleanSnapshot = useCallback(
    (ed: Editor | null) => {
      if (!ed) return;
      const wasDirty = isDirtyRef.current;
      isDirtyRef.current = false;
      lastSavedHtmlRef.current = ed.getHtml() + ":::" + ed.getCss();
      if (wasDirty) setDirtyTick((tick) => tick + 1);
    },
    [],
  );

  const checkAndUpdateDirty = useCallback((ed: Editor | null) => {
    if (!ed || !isDirtyRef.current) return;
    const current = ed.getHtml() + ":::" + ed.getCss();
    if (current === lastSavedHtmlRef.current) {
      isDirtyRef.current = false;
      setDirtyTick((tick) => tick + 1);
    }
  }, []);

  return { isDirtyRef, dirtyTick, markDirty, captureCleanSnapshot, checkAndUpdateDirty };
}
