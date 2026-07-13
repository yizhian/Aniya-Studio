import { Moon, Sun, Globe } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { useThemeMode } from "../../hooks/useThemeMode";
import { ThemeMode } from "../../models/editor";

export function AppearanceTab() {
  const { t, locale, setLocale } = useLocale();
  const { themeMode, toggleThemeMode } = useThemeMode();
  const isLight = themeMode === ThemeMode.Light;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <label className="text-[11px] font-semibold uppercase tracking-wider text-[var(--editor-text-muted)]">
          {t.settings.theme}
        </label>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => !isLight && toggleThemeMode()}
            className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-sm font-medium border transition-all ${
              isLight
                ? "bg-[var(--editor-accent)] text-[var(--editor-accent-text)] border-[var(--editor-accent)]"
                : "bg-[var(--editor-control)]/40 text-[var(--editor-text-muted)] border-[var(--editor-border)]"
            }`}
          >
            <Sun size={14} />
            {t.theme.light}
          </button>
          <button
            type="button"
            onClick={() => isLight && toggleThemeMode()}
            className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-sm font-medium border transition-all ${
              !isLight
                ? "bg-[var(--editor-accent)] text-[var(--editor-accent-text)] border-[var(--editor-accent)]"
                : "bg-[var(--editor-control)]/40 text-[var(--editor-text-muted)] border-[var(--editor-border)]"
            }`}
          >
            <Moon size={14} />
            {t.theme.dark}
          </button>
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <label className="text-[11px] font-semibold uppercase tracking-wider text-[var(--editor-text-muted)]">
          {t.settings.language}
        </label>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setLocale("zh-CN")}
            className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-sm font-medium border transition-all ${
              locale === "zh-CN"
                ? "bg-[var(--editor-accent)] text-[var(--editor-accent-text)] border-[var(--editor-accent)]"
                : "bg-[var(--editor-control)]/40 text-[var(--editor-text-muted)] border-[var(--editor-border)]"
            }`}
          >
            <Globe size={14} />
            中文
          </button>
          <button
            type="button"
            onClick={() => setLocale("en-US")}
            className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-sm font-medium border transition-all ${
              locale === "en-US"
                ? "bg-[var(--editor-accent)] text-[var(--editor-accent-text)] border-[var(--editor-accent)]"
                : "bg-[var(--editor-control)]/40 text-[var(--editor-text-muted)] border-[var(--editor-border)]"
            }`}
          >
            <Globe size={14} />
            English
          </button>
        </div>
      </div>
    </div>
  );
}
