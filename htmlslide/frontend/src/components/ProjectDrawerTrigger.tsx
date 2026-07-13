import React from "react";
import { FolderOpen } from "lucide-react";
import { motion } from "motion/react";
import { useLocale } from "../context/LocaleContext";

interface ProjectDrawerTriggerProps {
  onClick: () => void;
  projectCount: number;
  isOpen: boolean;
}

export function ProjectDrawerTrigger({ onClick, projectCount, isOpen }: ProjectDrawerTriggerProps) {
  const { t } = useLocale();
  return (
    <motion.button
      type="button"
      onClick={onClick}
      initial={{ opacity: 0, scale: 0.8 }}
      animate={{
        opacity: isOpen ? 0 : 1,
        scale: isOpen ? 0.8 : 1,
        pointerEvents: isOpen ? "none" : "auto",
      }}
      transition={{ duration: 0.25, ease: "easeOut" }}
      className="fixed left-4 top-1/2 -translate-y-1/2 z-30 w-11 h-11 rounded-full bg-[var(--editor-surface)] border border-[var(--editor-border)] shadow-sm backdrop-blur-md flex items-center justify-center hover:scale-105 active:scale-95 transition-transform cursor-pointer"
      title={t.drawer.projectList}
    >
      <FolderOpen size={20} className="text-[var(--editor-text)]" />
      {projectCount > 0 && (
        <span className="absolute -top-1 -right-1 min-w-[18px] h-[18px] rounded-full bg-[var(--editor-accent)] text-[var(--editor-accent-text)] text-[10px] font-semibold flex items-center justify-center px-1 leading-none">
          {projectCount > 99 ? "99+" : projectCount}
        </span>
      )}
    </motion.button>
  );
}