import { FileText, Image, X } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";

export interface FileChipFile {
  name: string;
  type?: string;
  pages?: number;
  charCount?: number;
  error?: string;
  status: "uploading" | "parsing" | "done" | "error";
}

interface FileChipProps {
  file: FileChipFile;
  onRemove?: () => void;
}

const STATUS_COLORS: Record<string, string> = {
  uploading: "var(--editor-accent)",
  parsing: "var(--editor-accent)",
  done: "#22c55e",
  error: "#ef4444",
};

const STATUS_ANIMATION: Record<string, string> = {
  uploading: "animate-pulse",
  parsing: "animate-pulse",
  done: "",
  error: "",
};

function formatMeta(file: FileChipFile, t: any): string | null {
  const parts: string[] = [];
  if (file.type && file.type !== "unsupported") parts.push(file.type);
  if (file.pages) parts.push(`${file.pages}${t.formatting.pages}`);
  if (file.charCount) parts.push(`${file.charCount.toLocaleString()}${t.formatting.characters}`);
  return parts.length > 0 ? parts.join(" · ") : null;
}

export function FileChip({ file, onRemove }: FileChipProps) {
  const { t } = useLocale();
  const meta = formatMeta(file, t);
  const statusLabel =
    file.status === "uploading" ? t.upload.uploading
    : file.status === "parsing" ? t.upload.parsing
    : file.status === "error" ? t.upload.parseFailed
    : null;
  const isImage = file.type === "image";

  return (
    <div className="flex items-center gap-1.5 rounded-md border border-[var(--editor-border)] bg-[var(--editor-control)] px-2.5 py-1 text-[11px] text-[var(--editor-text-muted)] font-mono">
      <span
        className={`h-1.5 w-1.5 rounded-full shrink-0 ${STATUS_ANIMATION[file.status]}`}
        style={{ backgroundColor: STATUS_COLORS[file.status] }}
      />
      {isImage ? <Image size={12} /> : <FileText size={12} />}
      <span className="truncate max-w-[140px] font-medium text-[var(--editor-text)]">
        {file.name}
      </span>
      {statusLabel && <span className="text-[10px] opacity-60">{statusLabel}</span>}
      {meta && file.status === "done" && (
        <span className="text-[10px] opacity-60">{meta}</span>
      )}
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          className="p-0.5 rounded-full hover:opacity-60 transition-opacity"
        >
          <X size={12} />
        </button>
      )}
    </div>
  );
}

export function FileChipGroup({
  files,
  onRemove,
}: {
  files: FileChipFile[];
  onRemove?: (i: number) => void;
}) {
  if (files.length === 0) return null;
  return (
    <div className="mb-2 flex flex-wrap items-center gap-2">
      {files.map((f, i) => (
        <FileChip key={`${f.name}-${i}`} file={f} onRemove={onRemove ? () => onRemove(i) : undefined} />
      ))}
    </div>
  );
}
