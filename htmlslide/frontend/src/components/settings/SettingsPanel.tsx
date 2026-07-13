import { useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { X, Cpu, Palette, Layers } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { ProviderConfigForm } from "./ProviderConfigForm";
import { AppearanceTab } from "./AppearanceTab";
import { SkillsTab } from "./SkillsTab";
import { Portal } from "../common/Portal";
import { Z_INDEX } from "../../constants/zIndex";

type Tab = "models" | "appearance" | "skills";

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

const TABS: { key: Tab; icon: typeof Cpu; labelKey: "models" | "appearance" | "skills" }[] = [
  { key: "models", icon: Cpu, labelKey: "models" },
  { key: "appearance", icon: Palette, labelKey: "appearance" },
  { key: "skills", icon: Layers, labelKey: "skills" },
];

export function SettingsPanel({ isOpen, onClose }: Props) {
  const { t } = useLocale();
  const [activeTab, setActiveTab] = useState<Tab>("models");

  return (
    <Portal>
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div
            key="backdrop"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.16 }}
            className="fixed inset-0 bg-black/25 backdrop-blur-[1px]"
            style={{ zIndex: Z_INDEX.OVERLAY_BACKDROP }}
            onClick={onClose}
          />

          <motion.div
            key="panel"
            initial={{ x: "100%" }}
            animate={{ x: 0 }}
            exit={{ x: "100%" }}
            transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            className="fixed right-0 top-0 bottom-0 w-[min(400px,100vw)] flex flex-col bg-[var(--editor-surface)] border-l border-[var(--editor-border)]"
            style={{ zIndex: Z_INDEX.OVERLAY }}
          >
            <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--editor-border)] shrink-0">
              <h2 className="text-sm font-semibold text-[var(--editor-text)]">
                {t.settings.title}
              </h2>
              <button
                type="button"
                onClick={onClose}
                className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control)] transition-colors"
              >
                <X size={16} />
              </button>
            </div>

            <div className="flex gap-1 px-4 py-2 border-b border-[var(--editor-border)] shrink-0">
              {TABS.map(({ key, icon: Icon, labelKey }) => (
                <button
                  key={key}
                  type="button"
                  onClick={() => setActiveTab(key)}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                    activeTab === key
                      ? "bg-[var(--editor-control)] text-[var(--editor-text)]"
                      : "text-[var(--editor-text-muted)] hover:text-[var(--editor-text)]"
                  }`}
                >
                  <Icon size={13} />
                  {t.settings[labelKey]}
                </button>
              ))}
            </div>

            <div className="flex-1 overflow-y-auto thin-scrollbar px-5 py-5">
              {activeTab === "models" && <ProviderConfigForm />}
              {activeTab === "appearance" && <AppearanceTab />}
              {activeTab === "skills" && <SkillsTab />}
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
    </Portal>
  );
}
