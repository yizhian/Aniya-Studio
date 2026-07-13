/**
 * Centralized error logger.
 * Replaces the ad-hoc `catch { /* ignore *​/ }` pattern found across the codebase.
 *
 * In development, logs to console. In production, this can be extended to
 * send errors to a monitoring service (Sentry, Datadog, etc.).
 */

type ErrorContext = Record<string, unknown>;

function formatError(operation: string, error: unknown, context?: ErrorContext): string {
  const ctx = context ? ` ${JSON.stringify(context)}` : "";
  return `[aniya-studio] ${operation} failed:${ctx}`;
}

/**
 * Log an error that occurred during a named operation.
 * Use this instead of empty `catch {}` blocks.
 */
export function logError(operation: string, error: unknown, context?: ErrorContext): void {
  console.error(formatError(operation, error, context), error);
}

/**
 * Log a warning (non-critical issue that should be investigated).
 */
export function logWarn(operation: string, message: string, context?: ErrorContext): void {
  console.warn(formatError(operation, message, context));
}
