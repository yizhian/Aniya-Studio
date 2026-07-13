import { useState } from "react";
import type { SelectedComponentContext } from "../models/editor";

export function useEditorSelection() {
  const [selectedContext, setSelectedContext] = useState<SelectedComponentContext | null>(null);
  return { selectedContext, setSelectedContext };
}
