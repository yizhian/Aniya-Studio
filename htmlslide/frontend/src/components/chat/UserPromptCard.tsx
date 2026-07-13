import React, { useState, useRef, useEffect, useMemo, useCallback } from "react";
import { Check, ChevronDown, Copy, FileText, MousePointer2 } from "lucide-react";
import type { AttachmentMeta, DomContext } from "../../models/chat";
import { useLocale } from "../../context/LocaleContext";

interface Props {
  content: string;
  domContext?: DomContext | null;
  attachments?: Array<AttachmentMeta | string>;
  defaultExpanded?: boolean;
}

/** Parse out DOM context info that the backend may concatenate into the message content */
function parseUserMessage(content: string, domContext?: DomContext | null): {
  displayText: string;
  domTag: string | null;
  domText: string | null;
} {
  // Backend may prepend DOM context like:
  // 【DOM 元素选中】\n- CSS 路径: ...\n- 标签: div\n- 文字内容: ...\n\n用户指令：actual prompt
  if (content.includes("【DOM 元素选中】")) {
    const parts = content.split("用户指令：");
    const userPart = parts.length > 1 ? parts[1].trim() : "";
    const domPart = parts[0] || "";

    if (domContext) {
      return {
        displayText: userPart || content,
        domTag: domContext.tag,
        domText: domContext.text?.slice(0, 80) || domContext.css_path?.slice(0, 80) || null,
      };
    }

    // Fallback: parse from the text
    const domLines = domPart.split("\n").filter((l) => l.startsWith("-"));
    const tag = domLines.find((l) => l.includes("标签:"))?.split("标签:")[1]?.trim();
    const text = domLines.find((l) => l.includes("文字内容:"))?.split("文字内容:")[1]?.trim();
    return {
      displayText: userPart || content,
      domTag: tag || null,
      domText: text?.slice(0, 80) || null,
    };
  }

  // No DOM context in content, use structured domContext if available
  if (domContext) {
    return {
      displayText: content,
      domTag: domContext.tag,
      domText: domContext.text?.slice(0, 80) || domContext.css_path?.slice(0, 80) || null,
    };
  }

  return { displayText: content, domTag: null, domText: null };
}

export function UserPromptCard({ content, domContext, attachments, defaultExpanded = false }: Props) {
  const { t } = useLocale();
  const [expanded, setExpanded] = useState(defaultExpanded);
  const contentRef = useRef<HTMLParagraphElement>(null);
  const [needsExpand, setNeedsExpand] = useState(false);
  const [copied, setCopied] = useState(false);

  const { displayText, domTag, domText } = useMemo(
    () => parseUserMessage(content, domContext),
    [content, domContext],
  );

  useEffect(() => {
    const el = contentRef.current;
    if (el) {
      setNeedsExpand(el.scrollWidth > el.clientWidth + 2);
    }
  }, [displayText]);

  const handleToggle = useCallback(() => {
    // Don't toggle if the user was selecting text
    if (needsExpand) {
      const sel = window.getSelection();
      if (sel && sel.toString().trim().length > 0) return;
      setExpanded(!expanded);
    }
  }, [needsExpand, expanded]);

  const handleCopy = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(displayText);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Fallback
      const ta = document.createElement("textarea");
      ta.value = displayText;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    }
  }, [displayText]);

  if (!displayText) return null;

  return (
    <div className="shrink-0 px-3 pt-1 pb-3">
      <div
        className={`group/prompt border border-[var(--editor-border)] rounded-2xl backdrop-blur-sm bg-[var(--editor-control)]/60 transition-all duration-300 ${
          needsExpand ? "cursor-pointer" : ""
        } ${expanded ? "px-4 pt-2.5 pb-3" : "px-4 pt-2.5 pb-2.5"}`}
        onClick={handleToggle}
      >
        {attachments && attachments.length > 0 && (
          <div className="mb-1.5 flex flex-wrap items-center gap-1.5">
            {attachments.map((item) => {
              const name = typeof item === "string" ? item : item.original_name;
              return (
              <div
                key={name}
                className="flex max-w-[160px] items-center gap-1 rounded-md border border-green-500/15 bg-green-500/[0.04] px-2 py-0.5 text-[10px] text-[var(--editor-text-muted)]"
              >
                <FileText size={10} className="shrink-0 text-green-500/80" />
                <span className="truncate font-medium text-[var(--editor-text)]">{name}</span>
              </div>
              );
            })}
          </div>
        )}

        {/* DOM context badge (if applicable) */}
        {domTag && (
          <div className="mb-1.5 flex items-center gap-1.5">
            <div className="flex items-center gap-1 px-2 py-0.5 rounded-full bg-[var(--editor-accent)]/5 border border-[var(--editor-accent)]/15 text-[10px] text-[var(--editor-accent)]">
              <MousePointer2 size={10} />
              <code className="font-mono">&lt;{domTag}&gt;</code>
            </div>
            {domText && (
              <span className="text-[10px] text-[var(--editor-text-muted)] truncate max-w-[180px]">
                {domText}
              </span>
            )}
          </div>
        )}

        <div className="flex items-start gap-2">
          {/* Prompt text */}
          <div className="flex-1 min-w-0">
            <p
              ref={contentRef}
              className={`text-sm leading-relaxed text-[var(--editor-text-muted)] group-hover/prompt:text-[var(--editor-text)] transition-colors duration-300 m-0 select-text ${
                expanded ? "whitespace-pre-wrap max-h-48 overflow-y-auto thin-scrollbar pr-1" : "truncate"
              }`}
            >
              {displayText}
            </p>
          </div>

          {/* Expand button */}
          {needsExpand && (
            <button
              className="shrink-0 w-5 h-5 flex items-center justify-center rounded-md text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] cursor-pointer bg-transparent border-0 p-0 transition-all opacity-0 group-hover/prompt:opacity-100"
              onClick={(e) => {
                e.stopPropagation();
                const sel = window.getSelection();
                if (sel && sel.toString().trim().length > 0) return;
                setExpanded(!expanded);
              }}
              title={expanded ? t.chat.collapse : t.chat.expand}
            >
              <ChevronDown
                size={14}
                className={`transition-transform duration-200 ${expanded ? "rotate-180" : ""}`}
              />
            </button>
          )}
        </div>

        {/* Copy button — only when expanded */}
        {expanded && needsExpand && (
          <div className="flex justify-end mt-2">
            <button
              type="button"
              onClick={handleCopy}
              className="flex items-center gap-1.5 text-[11px] text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] transition-colors cursor-pointer bg-transparent border-0 p-0"
            >
              {copied ? <Check size={12} /> : <Copy size={12} />}
              <span>{copied ? t.chat.copied : t.chat.copy}</span>
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
