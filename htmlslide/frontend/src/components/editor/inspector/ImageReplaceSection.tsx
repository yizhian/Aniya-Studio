import { ImageIcon, Upload } from "lucide-react";
import { useLocale } from "../../../context/LocaleContext";

interface Props {
  imageSrc: string;
  imageError: string | null;
  onImageSrcChange: (src: string) => void;
  onReplace: () => void;
}

export function ImageReplaceSection({ imageSrc, imageError, onImageSrcChange, onReplace }: Props) {
  const { t } = useLocale();

  return (
    <div
      style={{
        marginTop: 16,
        padding: 14,
        borderTop: "1px solid var(--inspector-border)",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 10 }}>
        <ImageIcon size={13} style={{ color: "var(--inspector-text-muted)" }} />
        <span
          style={{
            fontSize: 11,
            fontWeight: 700,
            color: "var(--inspector-section-title)",
            letterSpacing: "0.14em",
            textTransform: "uppercase",
          }}
        >
          {t.panels.image}
        </span>
      </div>
      {imageSrc && (
        <div
          style={{
            marginBottom: 10,
            overflow: "hidden",
            borderRadius: 10,
            border: "1px solid var(--inspector-border)",
          }}
        >
          <img
            src={imageSrc}
            alt=""
            style={{ height: 100, width: "100%", objectFit: "cover" }}
          />
        </div>
      )}
      <label
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 6,
          borderRadius: 10,
          border: "1px dashed var(--inspector-border)",
          padding: "7px 12px",
          fontSize: 12,
          fontWeight: 600,
          color: "var(--inspector-text)",
          cursor: "pointer",
          marginBottom: 8,
          transition: "background-color 150ms ease",
        }}
        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = "var(--inspector-hover)")}
        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = "transparent")}
      >
        <Upload size={12} />
        {t.panels.uploadLocal}
        <input
          type="file"
          accept="image/*"
          style={{ display: "none" }}
          onChange={(evt) => {
            const f = evt.target.files?.[0];
            if (f) {
              const r = new FileReader();
              r.onload = () => onImageSrcChange(String(r.result || ""));
              r.onerror = () => onImageSrcChange("");
              r.readAsDataURL(f);
            }
          }}
        />
      </label>
      <input
        value={imageSrc}
        onChange={(e) => onImageSrcChange(e.target.value)}
        className="inspector-number-input"
        placeholder="https://example.com/image.png"
        style={{ textAlign: "left", marginBottom: 8 }}
      />
      {imageError && (
        <p style={{ fontSize: 11, color: "var(--editor-danger)", marginBottom: 8 }}>{imageError}</p>
      )}
      <button
        type="button"
        onClick={onReplace}
        style={{
          width: "100%",
          borderRadius: 10,
          background: "var(--inspector-text)",
          color: "var(--inspector-bg)",
          padding: "9px 16px",
          fontSize: 12,
          fontWeight: 600,
          border: "none",
          cursor: "pointer",
          fontFamily: "'Inter', system-ui, sans-serif",
          transition: "opacity 150ms ease",
        }}
        onMouseEnter={(e) => (e.currentTarget.style.opacity = "0.9")}
        onMouseLeave={(e) => (e.currentTarget.style.opacity = "1")}
      >
        {t.imageReplace.replaceImage}
      </button>
    </div>
  );
}
