import { defineConfig } from 'vite'
import path from 'path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'


function figmaAssetResolver() {
  return {
    name: 'figma-asset-resolver',
    resolveId(id) {
      if (id.startsWith('figma:asset/')) {
        const filename = id.replace('figma:asset/', '')
        return path.resolve(__dirname, 'src/assets', filename)
      }
    },
  }
}

export default defineConfig({
  plugins: [
    figmaAssetResolver(),
    // The React and Tailwind plugins are both required for Make, even if
    // Tailwind is not being actively used – do not remove them
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      // Alias @ to the src directory
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': {
        // 使用 127.0.0.1 而非 localhost，避开 Node.js 把 localhost 优先解析为 ::1 (IPv6)
        // 而 docker compose 仅把端口发布在 IPv4 (0.0.0.0:8000) 上导致的 ECONNREFUSED
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
        // 后端冷启动期间前端抢先发起的请求会被代理报错，加重试避免误报
        retry: 5,
        retryDelay: 500,
        // SSE 端点（/chat）是长连接，模型生成 tool call 时可能 5s+ 不发 text 事件，
        // 导致 socket 超时断开。设 10 分钟覆盖整个会话周期。
        timeout: 600000,
      },
    },
  },

  // File types to support raw imports. Never add .css, .tsx, or .ts files to this.
  assetsInclude: ['**/*.svg', '**/*.csv'],
})
