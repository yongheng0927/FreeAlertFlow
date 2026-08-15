import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 配置参考：https://vite.dev/config/
export default defineConfig({
  // base 用相对路径：子路径部署（FENGHUO_SERVER_ROOT_URL 带路径前缀）时，
  // index.html 中的 ./assets/* 会相对当前页面 URL 解析，无需按部署前缀
  // 重新构建；前端路由均为单层路径，相对资源在任何路由下都能正确解析
  base: './',
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/healthz': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
