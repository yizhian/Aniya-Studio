import { useState, useCallback } from "react";

export function useLocalStorage<T>(
  key: string,
  defaultValue: T,
): [T, (value: T | ((prev: T) => T)) => void] {
  const [storedValue, setStoredValue] = useState<T>(() => {
    try {
      const item = window.localStorage.getItem(key);
      return item ? (JSON.parse(item) as T) : defaultValue;
    } catch {
      return defaultValue;
    }
  });

  const setValue = useCallback(
    (value: T | ((prev: T) => T)) => {
      setStoredValue((prev) => {
        const next = typeof value === "function"
          ? (value as (prev: T) => T)(prev)
          : value;
        try {
          window.localStorage.setItem(key, JSON.stringify(next));
        } catch {
          // Storage quota exceeded or access denied — silently fail
        }
        return next;
      });
    },
    [key],
  );

  return [storedValue, setValue];
}
