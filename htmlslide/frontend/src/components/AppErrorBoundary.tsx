import React from "react";
import { isRouteErrorResponse, useRouteError } from "react-router";
import { useLocale } from "../context/LocaleContext";

export function AppErrorBoundary() {
  const error = useRouteError();
  const { t } = useLocale();
  const message = isRouteErrorResponse(error)
    ? `${error.status} ${error.statusText}`
    : error instanceof Error
      ? error.message
      : t.errorBoundary.unknownError;

  return (
    <div className="min-h-screen bg-[var(--editor-bg)] text-[var(--editor-text)] flex items-center justify-center p-6">
      <div className="border border-[var(--editor-border)] bg-[var(--editor-surface)] max-w-lg rounded-lg p-6 shadow-sm">
        <h1 className="text-xl font-semibold">{t.errorBoundary.pageError}</h1>
        <p className="mt-3 text-sm leading-6 text-[var(--editor-text-muted)]">
          {t.errorBoundary.errorMessage}
        </p>
        <pre className="mt-4 max-h-40 overflow-auto rounded-lg bg-[var(--editor-control)] p-3 text-xs text-[var(--editor-danger)]">
          {message}
        </pre>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="mt-5 rounded-[50px] bg-[var(--editor-accent)] px-4 py-2 text-sm font-medium text-[var(--editor-accent-text)]"
        >
          {t.errorBoundary.refreshPage}
        </button>
      </div>
    </div>
  );
}
