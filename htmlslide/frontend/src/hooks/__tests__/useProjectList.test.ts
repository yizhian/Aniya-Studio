import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useProjectList } from '../useProjectList';

describe('useProjectList', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches projects on mount', async () => {
    const mockProjects = [
      { id: 'proj-1', name: 'Project 1', created_at: '2024-01-01' },
      { id: 'proj-2', name: 'Project 2', created_at: '2024-01-02' },
    ];
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockProjects),
    } as Response);

    const { result } = renderHook(() => useProjectList());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.projects).toEqual(mockProjects);
    expect(result.current.error).toBeNull();
  });

  it('sets error on fetch failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'));

    const { result } = renderHook(() => useProjectList());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.error).toBeTruthy();
    expect(result.current.projects).toEqual([]);
  });

  it('sets error on non-ok response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: false,
      status: 500,
    } as Response);

    const { result } = renderHook(() => useProjectList());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.error).toBeTruthy();
  });

  it('starts with loading true', () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([]),
    } as Response);

    const { result } = renderHook(() => useProjectList());
    expect(result.current.loading).toBe(true);
  });

  it('deleteProject removes project from list', async () => {
    const mockProjects = [
      { id: 'proj-1', name: 'Project 1', created_at: '2024-01-01' },
    ];
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockProjects),
    } as Response);

    const { result } = renderHook(() => useProjectList());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Now mock DELETE request.
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    } as Response);

    await act(async () => {
      await result.current.deleteProject('proj-1');
    });

    expect(result.current.projects).toEqual([]);
  });

  it('deleteProject sets deleteError on failure', async () => {
    vi.spyOn(globalThis, 'fetch')
      // Initial GET succeeds with empty list.
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve([]) } as Response)
      // DELETE fails.
      .mockResolvedValueOnce({
        ok: false,
        status: 404,
        json: () => Promise.resolve({ detail: 'Not found' }),
      } as Response);

    const { result } = renderHook(() => useProjectList());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await expect(async () => {
      await act(async () => {
        await result.current.deleteProject('proj-nonexistent');
      });
    }).rejects.toThrow('Not found');
  });

  it('clearDeleteError clears the delete error state', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([]),
    } as Response);

    const { result } = renderHook(() => useProjectList());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.clearDeleteError();
    });
    expect(result.current.deleteError).toBeNull();
  });

  it('refetch reloads projects', async () => {
    const projects1 = [{ id: 'proj-1', name: 'Project 1', created_at: '2024-01-01' }];
    const projects2 = [
      { id: 'proj-1', name: 'Project 1', created_at: '2024-01-01' },
      { id: 'proj-2', name: 'Project 2', created_at: '2024-01-02' },
    ];

    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(projects1) } as Response)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(projects2) } as Response);

    const { result } = renderHook(() => useProjectList());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.projects.length).toBe(1);

    await act(async () => {
      await result.current.refetch();
    });

    await waitFor(() => {
      expect(result.current.projects.length).toBe(2);
    });
  });
});
