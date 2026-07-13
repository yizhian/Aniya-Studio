export type UploadStatus = "idle" | "uploading" | "parsing" | "done" | "error";

const UPLOAD_TIMEOUT_MS = 120_000;

export interface ParsedFile {
  original_name: string;
  type: string;
  saved_path_rel: string;
  original_path_rel?: string;
  char_count?: number;
  pages?: number;
  width?: number;
  height?: number;
  format?: string;
  summary?: string;
  error?: string;
}

interface ParseStats {
  total_files: number;
  succeeded: number;
  unsupported: number;
  errors: number;
  total_duration_ms: number;
}

export interface UploadResponse {
  upload_id: string;
  session_id: string;
  files: ParsedFile[];
  summary_text: string;
  parse_stats: ParseStats;
}

export function uploadToAgentgo(
  files: File[],
  baseUrl: string,
  onProgress: (pct: number) => void,
  onStatus: (status: UploadStatus) => void,
  projectId: string,
): Promise<UploadResponse> {
  return new Promise((resolve, reject) => {
    const formData = new FormData();
    files.forEach((f) => formData.append("files", f));
    formData.append("project_id", projectId);

    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${baseUrl}/upload`);
    xhr.timeout = UPLOAD_TIMEOUT_MS;

    xhr.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable) {
        onProgress(Math.round((event.loaded / event.total) * 100));
      }
    });

    xhr.addEventListener("load", () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          const data = JSON.parse(xhr.responseText);
          resolve(data);
        } catch {
          reject(new Error("Server returned unparseable data"));
        }
      } else {
        let message: string;
        try {
          const body = JSON.parse(xhr.responseText);
          message = body?.message || body?.error || `Server error (HTTP ${xhr.status})`;
        } catch {
          message = `Server error (HTTP ${xhr.status})`;
        }
        reject(new Error(message));
      }
    });

    xhr.addEventListener("error", () => {
      reject(new Error("Cannot connect to file parsing service — ensure agentgo is running (port 8080)"));
    });

    xhr.addEventListener("timeout", () => {
      reject(new Error(`File parsing timeout (>${UPLOAD_TIMEOUT_MS / 1000}s). Try smaller files or fewer files.`));
    });

    xhr.addEventListener("abort", () => {
      reject(new Error("Upload cancelled"));
    });

    xhr.upload.addEventListener("load", () => {
      onStatus("parsing");
    });

    onStatus("uploading");
    xhr.send(formData);
  });
}
