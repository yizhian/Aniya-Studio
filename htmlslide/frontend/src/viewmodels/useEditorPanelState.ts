import { useState } from "react";

export function useEditorPanelState() {
  const [isTerminalOpen, setIsTerminalOpen] = useState(false);
  const [isVersionOpen, setIsVersionOpen] = useState(false);
  const [isAdvancedDesignOpen, setIsAdvancedDesignOpen] = useState(false);

  const [precipitateOpen, setPrecipitateOpen] = useState(false);
  const [precipitateHtml, setPrecipitateHtml] = useState("");
  const [imageDialogOpen, setImageDialogOpen] = useState(false);
  const [imageData, setImageData] = useState({ src: "", alt: "" });
  const [imageError, setImageError] = useState<string | null>(null);

  return {
    isTerminalOpen, setIsTerminalOpen,
    isVersionOpen, setIsVersionOpen,
    isAdvancedDesignOpen, setIsAdvancedDesignOpen,
    precipitateOpen, setPrecipitateOpen, precipitateHtml, setPrecipitateHtml,
    imageDialogOpen, setImageDialogOpen,
    imageData, setImageData,
    imageError, setImageError,
  };
}
