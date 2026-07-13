import { useState, useEffect } from "react";

interface Props {
  url: string;
}

export function TextFileViewer({ url }: Props) {
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetch(url)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.text();
      })
      .then((t) => {
        if (!cancelled) setText(t);
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      });

    return () => { cancelled = true; };
  }, [url]);

  if (error) return <div className="text-red-400 text-sm p-4">Failed to load: {error}</div>;
  if (text === null) return <div className="p-4 text-[var(--editor-text-muted)] text-sm">Loading...</div>;

  return (
    <pre className="p-6 text-[var(--editor-text)] text-sm leading-relaxed whitespace-pre-wrap font-sans select-text">
      {text}
    </pre>
  );
}
