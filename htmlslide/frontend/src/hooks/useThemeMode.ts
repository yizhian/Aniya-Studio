import { useCallback, useEffect } from "react";
import { ThemeMode, type ThemeModeValue } from "../models/editor";
import { useLocalStorage } from "./useLocalStorage";

const STORAGE_KEY = "html-editor-theme";

export function useThemeMode() {
  const [themeMode, setThemeMode] = useLocalStorage<ThemeModeValue>(STORAGE_KEY, ThemeMode.Dark);

  useEffect(() => {
    document.documentElement.classList.toggle("dark", themeMode === ThemeMode.Dark);
    document.documentElement.dataset.theme = themeMode;
  }, [themeMode]);

  const toggleThemeMode = useCallback(() => {
    setThemeMode((current) =>
      current === ThemeMode.Dark ? ThemeMode.Light : ThemeMode.Dark,
    );
  }, [setThemeMode]);

  return { themeMode, setThemeMode, toggleThemeMode };
}
