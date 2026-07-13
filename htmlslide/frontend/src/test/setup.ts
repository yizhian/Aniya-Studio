import '@testing-library/jest-dom';
import { vi } from 'vitest';

// Polyfill localStorage and sessionStorage for jsdom (missing in some versions).
const storage = new Map<string, string>();
const storageMock: Storage = {
  getItem: (key: string) => storage.get(key) ?? null,
  setItem: (key: string, value: string) => { storage.set(key, value); },
  removeItem: (key: string) => { storage.delete(key); },
  clear: () => { storage.clear(); },
  get length() { return storage.size; },
  key: (index: number) => [...storage.keys()][index] ?? null,
};
Object.defineProperty(window, 'localStorage', { value: storageMock, writable: true });
Object.defineProperty(window, 'sessionStorage', { value: storageMock, writable: true });

vi.mock('../context/LocaleContext', () => {
  // Proxy-based mock so any t.* key returns a readable string.
  const tProxy = new Proxy({} as Record<string, any>, {
    get(_target, prop: string) {
      return new Proxy({} as Record<string, any>, {
        get(_inner, innerProp: string) {
          return `${String(prop)}.${innerProp}`;
        },
      });
    },
  });

  return {
    useLocale: () => ({
      locale: 'zh-CN',
      setLocale: vi.fn(),
      toggleLocale: vi.fn(),
      t: tProxy,
    }),
    LocaleProvider: ({ children }: { children: React.ReactNode }) => children,
  };
});
