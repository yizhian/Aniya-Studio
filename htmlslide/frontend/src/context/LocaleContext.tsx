import React, { createContext, useCallback, useContext } from "react";
import { Locale, translations, type LocaleValue } from "../i18n/translations";
import { useLocalStorage } from "../hooks/useLocalStorage";

const STORAGE_KEY = "html-editor-locale";

interface LocaleContextValue {
  locale: LocaleValue;
  setLocale: (locale: LocaleValue) => void;
  toggleLocale: () => void;
  t: typeof translations["zh-CN"];
}

const LocaleCtx = createContext<LocaleContextValue | null>(null);

export function LocaleProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useLocalStorage<LocaleValue>(STORAGE_KEY, Locale.ZhCN);

  const setLocale = useCallback((loc: LocaleValue) => {
    setLocaleState(loc);
  }, [setLocaleState]);

  const toggleLocale = useCallback(() => {
    setLocaleState((curr) => (curr === Locale.ZhCN ? Locale.EnUS : Locale.ZhCN));
  }, [setLocaleState]);

  const t = translations[locale];

  return (
    <LocaleCtx.Provider value={{ locale, setLocale, toggleLocale, t }}>
      {children}
    </LocaleCtx.Provider>
  );
}

export function useLocale(): LocaleContextValue {
  const ctx = useContext(LocaleCtx);
  if (!ctx) throw new Error("useLocale must be used within LocaleProvider");
  return ctx;
}
