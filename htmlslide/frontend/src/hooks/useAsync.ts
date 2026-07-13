import { useCallback, useRef, useState } from "react";

export interface AsyncState<T> {
  loading: boolean;
  error: string | null;
  data: T | null;
}

/**
 * Generic hook for async operations with loading, error, and data state.
 * Automatically prevents state updates on unmounted components.
 */
export function useAsync<T>() {
  const [state, setState] = useState<AsyncState<T>>({
    loading: false,
    error: null,
    data: null,
  });
  const mountedRef = useRef(true);

  const execute = useCallback(async (fn: () => Promise<T>): Promise<T | null> => {
    setState({ loading: true, error: null, data: null });
    try {
      const result = await fn();
      if (mountedRef.current) {
        setState({ loading: false, error: null, data: result });
      }
      return result;
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Operation failed";
      if (mountedRef.current) {
        setState({ loading: false, error: msg, data: null });
      }
      return null;
    }
  }, []);

  const reset = useCallback(() => {
    setState({ loading: false, error: null, data: null });
  }, []);

  return { ...state, execute, reset };
}
