import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useThemeMode } from '../useThemeMode';
import { ThemeMode } from '../../models/editor';

const STORAGE_KEY = 'html-editor-theme';

describe('useThemeMode', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.classList.remove('dark');
    delete document.documentElement.dataset.theme;
  });

  it('returns dark as default theme', () => {
    const { result } = renderHook(() => useThemeMode());
    expect(result.current.themeMode).toBe(ThemeMode.Dark);
  });

  it('reads saved light theme from localStorage', () => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(ThemeMode.Light));
    const { result } = renderHook(() => useThemeMode());
    expect(result.current.themeMode).toBe(ThemeMode.Light);
  });

  it('defaults to dark for invalid storage value', () => {
    window.localStorage.setItem(STORAGE_KEY, 'bad json');
    const { result } = renderHook(() => useThemeMode());
    expect(result.current.themeMode).toBe(ThemeMode.Dark);
  });

  it('toggleThemeMode switches from dark to light', () => {
    const { result } = renderHook(() => useThemeMode());
    expect(result.current.themeMode).toBe(ThemeMode.Dark);

    act(() => {
      result.current.toggleThemeMode();
    });

    expect(result.current.themeMode).toBe(ThemeMode.Light);
  });

  it('toggleThemeMode switches from light to dark', () => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(ThemeMode.Light));
    const { result } = renderHook(() => useThemeMode());

    act(() => {
      result.current.toggleThemeMode();
    });

    expect(result.current.themeMode).toBe(ThemeMode.Dark);
  });

  it('setThemeMode explicitly changes theme', () => {
    const { result } = renderHook(() => useThemeMode());

    act(() => {
      result.current.setThemeMode(ThemeMode.Light);
    });

    expect(result.current.themeMode).toBe(ThemeMode.Light);
  });

  it('persists theme to localStorage on change', () => {
    const { result } = renderHook(() => useThemeMode());

    act(() => {
      result.current.toggleThemeMode();
    });

    expect(window.localStorage.getItem(STORAGE_KEY)).toBe(JSON.stringify(ThemeMode.Light));
  });

  it('sets dark class on document by default', () => {
    renderHook(() => useThemeMode());
    // The useEffect should apply the dark class.
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('removes dark class when toggled to light', () => {
    const { result } = renderHook(() => useThemeMode());
    // Default is dark.
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    act(() => {
      result.current.toggleThemeMode();
    });

    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('sets data-theme attribute on document', () => {
    renderHook(() => useThemeMode());
    expect(document.documentElement.dataset.theme).toBe(ThemeMode.Dark);
  });
});
