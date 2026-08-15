package service

import (
	"encoding/json"

	"github.com/yongheng0927/FreeAlertFlow/internal/model"
)

// MatchRules 把某个接入源的路由规则应用到告警 labels 上（FR-3.2）：规则按
// 传入顺序（priority 升序）逐条评估，match_labels 中所有 key=value 全部
// 命中才算匹配（AND 语义），空的 match_labels {} 即默认兜底规则 命中后
// 停止匹配，除非该规则开了 continue_matching（一条告警可发多个渠道）
func MatchRules(rules []model.RoutingRule, labels map[string]string) []model.RoutingRule {
	var matched []model.RoutingRule
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if !matchLabels(r.MatchLabels, labels) {
			continue
		}
		matched = append(matched, r)
		if !r.ContinueMatching {
			break
		}
	}
	return matched
}

// matchLabels 判断规则的 match_labels 中所有键值对是否都出现在 labels
// 中 JSON 非法时视为永不匹配
func matchLabels(raw json.RawMessage, labels map[string]string) bool {
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	for k, v := range m {
		if labels[k] != v {
			return false
		}
	}
	return true
}
