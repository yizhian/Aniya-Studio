import { useState, useEffect } from "react";
import { marked } from "marked";

interface Props {
  url: string;
}

export function MarkdownViewer({ url }: Props) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetch(url)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.text();
      })
      .then((md) => {
        if (cancelled) return;
        const body = marked.parse(md) as string;
        const html = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><style>
  body { font-family: system-ui, sans-serif; line-height: 1.6; padding: 2rem; color: #1a1a1a; overflow-x: hidden; word-wrap: break-word; }
  h1 { font-size: 1.25rem; font-weight: 700; margin: 1.5rem 0 0.75rem; }
  h2 { font-size: 1.1rem; font-weight: 600; margin: 1.25rem 0 0.5rem; }
  h3 { font-size: 1rem; font-weight: 600; margin: 1rem 0 0.5rem; }
  p { margin: 0.5rem 0; }
  ul, ol { padding-left: 1.5rem; margin: 0.5rem 0; }
  li { margin: 0.25rem 0; }
  strong { font-weight: 600; }
  code { background: #e5e5e5; padding: 0.125rem 0.25rem; border-radius: 0.25rem; font-size: 0.85em; }
  pre { background: #e5e5e5; padding: 1rem; border-radius: 0.5rem; overflow-x: auto; }
  pre code { background: none; padding: 0; }
  a { color: #2563eb; text-decoration: underline; }
  blockquote { border-left: 2px solid #d4d4d4; padding-left: 1rem; color: #737373; margin: 0.5rem 0; }
  hr { border: none; border-top: 1px solid #d4d4d4; margin: 1.5rem 0; }
  img, table { max-width: 100%; height: auto; }
</style></head><body>${body}</body></html>`;
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

  if (error) return <div className="text-red-400 text-sm p-4">Failed to load: {error}</div>;
  if (!blobUrl) return <div className="p-4 text-[var(--editor-text-muted)] text-sm">Loading...</div>;

  return (
    <iframe
      src={blobUrl}
      className="w-full h-full"
      title="Markdown Viewer"
    />
  );
}
