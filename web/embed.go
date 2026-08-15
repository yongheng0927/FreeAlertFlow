// Package web 将构建好的前端资源（web/dist）嵌入二进制文件
// 仓库中提交的 dist/index.html 是占位文件，保证干净的检出（此时还没有
// 真实的 vite 构建产物）也能编译通过；`npm run build` 会用真实产物覆盖它
package web

import "embed"

// DistFS 包含 SPA 构建产物（index.html + assets/）
//
//go:embed dist
var DistFS embed.FS
