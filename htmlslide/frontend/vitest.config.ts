import { defineConfig } from 'vitest/config';
import path from 'path';
import react from '@vitejs/plugin-react';
const mockDir = path.resolve(__dirname, 'src/test/__mocks__');

// EditorContext.tsx imports graphejs (2.7 MB) and its CSS. Tests mock
// useEditor (the only runtime consumer of EditorContext), but Vite's
// dependency optimizer walks the raw import chain before vi.mock()
// can intercept. The resolveId hook below prevents the 2.7 MB module
// from being loaded, avoiding OOM in vitest workers.

export default defineConfig({
  plugins: [{
    name: 'grapesjs-mock',
    resolveId(id) {
      if (id === 'grapesjs') return path.join(mockDir, 'grapesjs.ts');
      if (id === 'grapesjs-parser-postcss') return path.join(mockDir, 'grapesjs-parser-postcss.ts');
      if (id === 'grapesjs-custom-code') return path.join(mockDir, 'grapesjs-custom-code.ts');
      if (id === 'grapesjs/dist/css/grapes.min.css') return path.join(mockDir, 'grapesjs-css.css');
      return null;
    },
  }, react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
  },
});
