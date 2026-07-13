import { useCallback, useEffect, useState } from "react";
import type { ProjectResponse } from "../models/chat";
import { useLocale } from "../context/LocaleContext";
import { extractErrorMessage } from "../models/apiResponses";

export function useProjectList() {
  const { t } = useLocale();
  const [projects, setProjects] = useState<ProjectResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const fetchProjects = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/v1/projects");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: ProjectResponse[] = await res.json();
      setProjects(data);
    } catch {
      setError(t.errors.loadProjectsFailed);
    } finally {
      setLoading(false);
    }
  }, [t]);

  const deleteProject = useCallback(
    async (projectId: string) => {
      setDeleteError(null);
      const res = await fetch(`/api/v1/projects/${projectId}`, {
        method: "DELETE",
      });
      if (!res.ok) {
        const msg = await extractErrorMessage(res);
        setDeleteError(msg);
        throw new Error(msg);
      }
      setProjects((prev) => prev.filter((p) => p.id !== projectId));
    },
    [t],
  );

  const clearDeleteError = useCallback(() => setDeleteError(null), []);

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  return { projects, loading, error, deleteProject, deleteError, clearDeleteError, refetch: fetchProjects };
}
