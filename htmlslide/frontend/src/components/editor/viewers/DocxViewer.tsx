import { useState, useEffect } from "react";

interface Props {
  url: string;
}

export function DocxViewer({ url }: Props) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    fetch(url)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.arrayBuffer();
      })
      .then((buf) => import("mammoth").then((m) => m.default.convertToHtml({ arrayBuffer: buf })))
      .then((result) => {
        if (cancelled) return;
        const html = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><style>
  body { font-family: system-ui, sans-serif; line-height: 1.6; padding: 2rem; color: #1a1a1a; overflow-x: hidden; word-wrap: break-word; }
  img, table { max-width: 100%; height: auto; }
</style></head><body>${result.value}</body></html>`;
        const blob = new Blob([html], { type: "text/html;charset=utf-8" });
        setBlobUrl(URL.createObjectURL(blob));
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      });

    return () => { cancelled = true; };
  }, [url]);

  useEffect(() => {
    return () => { if (blobUrl) URL.revokeObjectURL(blobUrl); };
  }, [blobUrl]);

  if (error) return <div className="text-red-400 text-sm p-4">Failed to load DOCX: {error}</div>;
  if (!blobUrl) return <div className="p-4 text-[var(--editor-text-muted)] text-sm">Loading...</div>;

  return (
    <iframe
      src={blobUrl}
      className="w-full h-full"
      title="DOCX Viewer"
    />
  );
}
