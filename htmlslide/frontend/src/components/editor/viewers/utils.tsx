import { FileText, FileImage } from "lucide-react";
import type { ParsedFile } from "../../../services/upload";

export type ViewerDoc = ParsedFile;

export type FileType = "pdf" | "docx" | "image" | "markdown" | "text" | "other";

export function detectFileType(doc: ViewerDoc): FileType {
  const t = doc.type?.toLowerCase() ?? "";
  if (t === "pdf") return "pdf";
  if (t === "docx") return "docx";
  if (t === "image" || t === "png" || t === "jpeg" || t === "jpg" || t === "gif" || t === "webp" || t === "svg") return "image";
  if (t === "markdown") return "markdown";
  if (t === "text") return "text";
  return "other";
}

export function fileTypeIcon(doc: ViewerDoc) {
  const ft = detectFileType(doc);
  if (ft === "pdf") return <FileText size={14} className="text-red-400 shrink-0" />;
  if (ft === "docx") return <FileText size={14} className="text-blue-400 shrink-0" />;
  if (ft === "image") return <FileImage size={14} className="text-green-400 shrink-0" />;
  if (ft === "markdown") return <FileText size={14} className="text-purple-400 shrink-0" />;
  return <FileText size={14} className="text-[var(--editor-text-muted)] shrink-0" />;
}

export function fileUrl(agentgoBaseUrl: string, projectId: string, doc: ViewerDoc): string {
  if (doc.original_path_rel) {
    return `${agentgoBaseUrl}/files/${projectId}/originals/${encodeURIComponent(doc.original_name)}`;
  }
  const safeName = doc.saved_path_rel?.split('/').pop() ?? doc.original_name;
  return `${agentgoBaseUrl}/files/${projectId}/docs/${encodeURIComponent(safeName)}`;
}
