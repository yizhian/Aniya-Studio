import React from "react";

function inline(text: string): React.ReactNode {
  const cleaned = text.replace(/<br\s*\/?>/gi, "\n");
  const parts = cleaned.split(/(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`)/g);
  return parts.map((part, i) => {
    if (/^\*\*.*\*\*$/.test(part)) return <strong key={i} className="font-semibold">{part.slice(2, -2)}</strong>;
    if (/^\*.*\*$/.test(part)) return <em key={i} className="italic">{part.slice(1, -1)}</em>;
    if (/^`.*`$/.test(part)) return <code key={i} className="px-1 py-0.5 text-xs font-mono bg-[var(--editor-control)] rounded">{part.slice(1, -1)}</code>;
    if (part.includes("\n")) return <span key={i}>{part.split("\n").map((seg, j) => <React.Fragment key={j}>{j > 0 && <br />}{seg}</React.Fragment>)}</span>;
    return part;
  });
}

export function InlineMarkdown({ text }: { text: string }) {
  if (!text) return null;

  const lines = text.split("\n");
  const elements: React.ReactNode[] = [];
  let i = 0;
  let key = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Table
    if (line.trim().startsWith("|") && line.trim().endsWith("|")) {
      const rows: string[][] = [];
      let hasSep = false;
      while (i < lines.length && lines[i].trim().startsWith("|") && lines[i].trim().endsWith("|")) {
        const r = lines[i].trim();
        if (/^\|[\s\-:]+\|[\s\-:]+\|?$/.test(r.replace(/\|/g, "|"))) { hasSep = true; i++; continue; }
        rows.push(r.split("|").slice(1, -1).map((c) => c.trim()));
        i++;
      }
      if (rows.length > 0) {
        elements.push(
          <div key={++key} className="my-2 overflow-x-auto rounded-lg border border-[var(--editor-border)]">
            <table className="w-full text-xs">
              <tbody>
                {rows.map((row, ri) => (
                  <tr key={ri} className={ri === 0 && hasSep ? "bg-[var(--editor-control)] font-semibold" : ri % 2 === 0 ? "bg-[var(--editor-control)]/30" : ""}>
                    {row.map((cell, ci) => (
                      <td key={ci} className="px-2 py-1 border-r border-[var(--editor-border)] last:border-r-0 text-[var(--editor-text)]">{inline(cell)}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>,
        );
        continue;
      }
    }

    // Code block
    if (line.trim().startsWith("```")) {
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith("```")) { codeLines.push(lines[i]); i++; }
      i++;
      if (codeLines.length > 0) {
        elements.push(
          <pre key={++key} className="my-2 p-2.5 rounded-lg bg-[var(--editor-control)] border border-[var(--editor-border)] overflow-x-auto">
            <code className="text-xs font-mono text-[var(--editor-text)] whitespace-pre-wrap">{codeLines.join("\n")}</code>
          </pre>,
        );
      }
      continue;
    }

    // Headings
    if (/^###\s/.test(line)) { elements.push(<h3 key={++key} className="text-sm font-semibold mt-3 mb-1">{inline(line.replace(/^###\s/, ""))}</h3>); i++; continue; }
    if (/^##\s/.test(line)) { elements.push(<h2 key={++key} className="text-sm font-bold mt-4 mb-1.5 pb-1 border-b border-[var(--editor-border)]/40">{inline(line.replace(/^##\s/, ""))}</h2>); i++; continue; }

    // Lists
    if (/^[\-\*]\s/.test(line.trim())) {
      const items: string[] = [];
      while (i < lines.length && /^[\-\*]\s/.test(lines[i].trim())) { items.push(lines[i].trim().replace(/^[\-\*]\s/, "")); i++; }
      elements.push(<ul key={++key} className="my-1 space-y-0.5 list-disc list-outside ml-4">{items.map((it, idx) => <li key={idx} className="text-sm">{inline(it)}</li>)}</ul>);
      continue;
    }
    if (/^\d+\.\s/.test(line.trim())) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s/.test(lines[i].trim())) { items.push(lines[i].trim().replace(/^\d+\.\s/, "")); i++; }
      elements.push(<ol key={++key} className="my-1 space-y-0.5 list-decimal list-outside ml-4">{items.map((it, idx) => <li key={idx} className="text-sm">{inline(it)}</li>)}</ol>);
      continue;
    }

    // Empty
    if (line.trim() === "") { i++; continue; }

    // Paragraph
    const para: string[] = [];
    while (i < lines.length && lines[i].trim() !== "" && !lines[i].trim().startsWith("```") && !/^#{1,3}\s/.test(lines[i]) && !/^[\-\*]\s/.test(lines[i].trim()) && !/^\d+\.\s/.test(lines[i].trim()) && !(lines[i].trim().startsWith("|") && lines[i].trim().endsWith("|"))) {
      para.push(lines[i]); i++;
    }
    elements.push(<p key={++key} className="text-sm my-1.5">{inline(para.join("\n"))}</p>);
  }

  return <div className="markdown">{elements}</div>;
}
