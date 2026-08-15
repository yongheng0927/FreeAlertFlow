package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// 规范化序列化用的键值/键值对分隔符：选用 ASCII 控制字符，几乎不会出现在
// 真实 label 值里，即使出现也不会产生歧义
const (
	kvSep   = "\x1e"
	pairSep = "\x1f"
)

// canonical 把 label map 确定性序列化：key 排序后按 "k\x1ev" 形式用 "\x1f"
// 连接，保证同样的 map 永远得到同样的字符串
func canonical(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(pairSep)
		}
		b.WriteString(k)
		b.WriteString(kvSep)
		b.WriteString(m[k])
	}
	return b.String()
}

// FingerprintFromLabels 计算 64 字符的 SHA-256 指纹，用于 Alertmanager
// 未提供 fingerprint 的场景（DATABASE_DESIGN §8）
func FingerprintFromLabels(labels map[string]string) string {
	sum := sha256.Sum256([]byte(canonical(labels)))
	return hex.EncodeToString(sum[:])
}

// ContentHash 计算去重判定用的内容指纹（FR-1.3）：
// SHA-256(status + key 排序后的 labels + key 排序后的 annotations)
// 入库后去重只需比对定长字符串，无需反序列化比对完整 JSON
func ContentHash(status string, labels, annotations map[string]string) string {
	h := sha256.New()
	h.Write([]byte(status))
	h.Write([]byte(pairSep))
	h.Write([]byte(canonical(labels)))
	h.Write([]byte(pairSep))
	h.Write([]byte(canonical(annotations)))
	return hex.EncodeToString(h.Sum(nil))
}
