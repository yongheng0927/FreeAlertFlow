// Package migrations 内嵌启动时执行的 SQL 迁移文件
package migrations

import "embed"

// FS 包含所有 *.sql 迁移文件（golang-migrate 的 iofs source）
//
//go:embed *.sql
var FS embed.FS
