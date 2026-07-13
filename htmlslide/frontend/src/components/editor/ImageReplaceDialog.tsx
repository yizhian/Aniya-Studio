import React, { useEffect, useState } from "react";
import { Image as ImageIcon, Upload, X } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";

type Props = {
  isOpen: boolean;
  initialSrc: string;
  error: string | null;
  onClose: () => void;
  onSubmit: (payload: { src: string }) => void;
};

export function ImageReplaceDialog({
  isOpen,
  initialSrc,
  error,
  onClose,
  onSubmit,
}: Props) {
  const { t } = useLocale();
  const [src, setSrc] = useState(initialSrc);

  useEffect(() => {
    if (!isOpen) return;
    setSrc(initialSrc);
  }, [initialSrc, isOpen]);

  if (!isOpen) return null;

  return (
    <div className="absolute left-24 top-1/2 z-[60] w-[420px] -translate-y-1/2">
      <form
        className="rounded-lg border border-[var(--editor-border)] bg-[var(--editor-surface)] p-5 text-[var(--editor-text)] shadow-sm"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit({ src });
        }}
      >
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <ImageIcon size={18} className="text-[var(--editor-text-muted)]" />
            <h2 className="text-sm font-semibold">{t.imageReplace.replaceImage}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-[var(--editor-text-muted)] hover:bg-[var(--editor-control-hover)] hover:text-[var(--editor-text)]"
          >
            <X size={16} />
          </button>
        </div>
        {src && (
          <div className="mb-3 overflow-hidden rounded-lg border border-[var(--editor-border)] bg-[var(--editor-control)]">
            <img src={src} alt="" className="h-40 w-full object-cover" />
          </div>
        )}
        <label className="mb-3 flex cursor-pointer items-center justify-center gap-2 rounded-lg border border-dashed border-[var(--editor-border)] bg-[var(--editor-control)] px-4 py-3 text-sm font-semibold text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)]">
          <Upload size={16} />
          {t.imageReplace.uploadLocal}
          <input
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(event) => {
              const file = event.target.files?.[0];
              if (!file) return;
              const reader = new FileReader();
              reader.onload = () => setSrc(String(reader.result || ""));
              reader.onerror = () => setSrc("");
              reader.readAsDataURL(file);
            }}
          />
        </label>
        <label className="block text-xs font-semibold text-[var(--editor-text-muted)]">
          {t.imageReplace.imageUrl}
          <input
            value={src}
            onChange={(event) => setSrc(event.target.value)}
            className="mt-1 h-10 w-full rounded-lg border border-[var(--editor-border)] bg-[var(--editor-control)] px-3 text-sm text-[var(--editor-text)] outline-none focus:border-[var(--editor-text-muted)]"
            placeholder="https://example.com/image.png"
          />
        </label>
        {error && <p className="mt-3 text-sm text-[var(--editor-danger)]">{error}</p>}
        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg px-4 py-2 text-sm text-[var(--editor-text-muted)] hover:bg-[var(--editor-control-hover)]"
          >
            {t.imageReplace.cancel}
          </button>
          <button
            type="submit"
            className="rounded-lg bg-[var(--editor-accent)] px-4 py-2 text-sm font-semibold text-[var(--editor-accent-text)] hover:opacity-90"
          >
            {t.imageReplace.replace}
          </button>
        </div>
      </form>
    </div>
  );
}
