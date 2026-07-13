import { useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router";
import { FileCode2, Search, X } from "lucide-react";
import { useProjectList } from "../hooks/useProjectList";
import { useLocale } from "../context/LocaleContext";
import { AnimatedPanel } from "./common/AnimatedPanel";
import { LoadingState, ErrorState, EmptyState } from "./common/StateRenderer";
import { ProjectCard } from "./home/ProjectCard";

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export function ProjectDrawer({ isOpen, onClose }: Props) {
  const navigate = useNavigate();
  const { t, locale } = useLocale();
  const { projects, loading, error, deleteProject, deleteError, clearDeleteError, refetch } =
    useProjectList();
  const [search, setSearch] = useState("");
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  const filtered = useMemo(() => {
    if (!search.trim()) return projects;
    const q = search.toLowerCase();
    return projects.filter((p) => p.name.toLowerCase().includes(q));
  }, [projects, search]);

  const handleDelete = useCallback(
    async (projectId: string) => {
      clearDeleteError();
      setDeletingId(projectId);
      setConfirmDeleteId(null);
      try {
        await deleteProject(projectId);
      } catch {
        // error surfaced via deleteError from hook
      } finally {
        setDeletingId(null);
      }
    },
    [deleteProject, clearDeleteError],
  );

  const handleCardClick = useCallback(
    (projectId: string) => {
      navigate(`/editor/${projectId}`);
      onClose();
    },
    [navigate, onClose],
  );

  return (
    <AnimatedPanel isOpen={isOpen} onClose={onClose} position="left" width={320}>
      {/* Header */}
      <div className="shrink-0 px-4 pt-5 pb-3 border-b border-[var(--editor-border)]">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2.5">
            <span
              className="w-1.5 h-1.5 rounded-full"
              style={{ backgroundColor: "var(--editor-text)" }}
            />
            <h2 className="text-xs font-bold tracking-[0.2em] text-[var(--editor-text)] uppercase">
              {t.drawer.projects}
            </h2>
            <span className="text-[11px] text-[var(--editor-text-muted)] font-mono tabular-nums">
              {projects.length}
            </span>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-[var(--editor-text-muted)] hover:text-[var(--editor-text)] hover:bg-[var(--editor-control-hover)] transition-colors"
          >
            <X size={16} />
          </button>
        </div>

        {projects.length > 0 && (
          <div className="relative">
            <Search
              size={13}
              className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--editor-text-muted)]"
            />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t.drawer.searchPlaceholder}
              className="w-full h-8 rounded-lg border border-[var(--editor-border)] bg-[var(--editor-control)] pl-7 pr-2.5 text-xs text-[var(--editor-text)] outline-none placeholder:text-[var(--editor-text-muted)] focus:border-[var(--editor-text-muted)] transition-colors"
            />
          </div>
        )}
      </div>

      {/* Body */}
      <div
        className="flex-1 overflow-y-auto px-3 py-3 thin-scrollbar"
        style={{
          maskImage:
            "linear-gradient(black calc(100% - 40px), rgba(0,0,0,0.4) calc(100% - 16px), transparent)",
          WebkitMaskImage:
            "linear-gradient(black calc(100% - 40px), rgba(0,0,0,0.4) calc(100% - 16px), transparent)",
        }}
      >
        {loading && <LoadingState />}

        {!loading && error && (
          <ErrorState message={error} onRetry={refetch} retryLabel={t.drawer.retry} />
        )}

        {!loading && !error && projects.length === 0 && (
          <EmptyState icon={FileCode2} title={t.drawer.noProjects} subtitle={t.drawer.noProjectsHint} />
        )}

        {!loading && !error && projects.length > 0 && filtered.length === 0 && (
          <EmptyState icon={Search} title={`${t.drawer.noMatchPrefix}${search}${t.drawer.noMatchSuffix}`} />
        )}

        {!loading &&
          filtered.map((project, idx) => (
            <ProjectCard
              key={project.id}
              project={project}
              index={idx}
              onClick={() => handleCardClick(project.id)}
              onDelete={() => handleDelete(project.id)}
              isConfirming={confirmDeleteId === project.id}
              onRequestConfirm={() => {
                setConfirmDeleteId(project.id);
                clearDeleteError();
              }}
              onCancelConfirm={() => {
                setConfirmDeleteId(null);
                clearDeleteError();
              }}
              isDeleting={deletingId === project.id}
              deleteError={deleteError}
              locale={locale}
              deleteLabel={t.drawer.deleteProject}
              confirmLabel={t.drawer.confirm}
              cancelLabel={t.drawer.cancel}
            />
          ))}
      </div>
    </AnimatedPanel>
  );
}
