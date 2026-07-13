import React from "react";
import { Globe } from "lucide-react";
import { motion } from "motion/react";
import { useLocale } from "../context/LocaleContext";
import { Locale } from "../i18n/translations";

export function LocaleSwitcher() {
  const { locale, toggleLocale } = useLocale();
  const label = locale === Locale.ZhCN ? "EN" : "中";

  return (
    <motion.button
      whileTap={{ scale: 0.92 }}
      type="button"
      onClick={toggleLocale}
      className="flex items-center gap-2 text-[13px] font-semibold text-[var(--editor-text)] hover:opacity-60 transition-opacity"
      title={locale === Locale.ZhCN ? "Switch to English" : "切换到中文"}
    >
      <Globe size={16} />
      <span>{label}</span>
    </motion.button>
  );
}
