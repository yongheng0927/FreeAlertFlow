package service

import (
	"encoding/json"
	"testing"

	"github.com/yongheng0927/fenghuo/internal/model"
)

func rule(id int64, priority int, match string, cont bool) model.RoutingRule {
	return model.RoutingRule{
		ID:               id,
		Priority:         priority,
		MatchLabels:      json.RawMessage(match),
		ContinueMatching: cont,
		Enabled:          true,
	}
}

func TestMatchRulesFirstHitStops(t *testing.T) {
	rules := []model.RoutingRule{
		rule(1, 10, `{"severity":"critical"}`, false),
		rule(2, 20, `{"namespace":"prod"}`, false),
	}
	labels := map[string]string{"severity": "critical", "namespace": "prod"}
	matched := MatchRules(rules, labels)
	if len(matched) != 1 || matched[0].ID != 1 {
		t.Fatalf("matched = %+v, want only rule 1", matched)
	}
}

func TestMatchRulesContinueMatching(t *testing.T) {
	rules := []model.RoutingRule{
		rule(1, 10, `{"severity":"critical"}`, true),
		rule(2, 20, `{"namespace":"prod"}`, false),
		rule(3, 30, `{}`, false), // 兜底规则，永远到不了：rule 2 会终止匹配
	}
	labels := map[string]string{"severity": "critical", "namespace": "prod"}
	matched := MatchRules(rules, labels)
	if len(matched) != 2 || matched[0].ID != 1 || matched[1].ID != 2 {
		t.Fatalf("matched = %+v, want rules 1 and 2", matched)
	}
}

func TestMatchRulesDefaultFallback(t *testing.T) {
	rules := []model.RoutingRule{
		rule(1, 10, `{"severity":"critical"}`, false),
		rule(2, 100, `{}`, false),
	}
	labels := map[string]string{"severity": "info"}
	matched := MatchRules(rules, labels)
	if len(matched) != 1 || matched[0].ID != 2 {
		t.Fatalf("matched = %+v, want fallback rule 2", matched)
	}
}

func TestMatchRulesANDSemantics(t *testing.T) {
	rules := []model.RoutingRule{
		rule(1, 10, `{"severity":"critical","namespace":"prod"}`, false),
	}
	if got := MatchRules(rules, map[string]string{"severity": "critical"}); len(got) != 0 {
		t.Fatal("partial match must not hit")
	}
	if got := MatchRules(rules, map[string]string{"severity": "critical", "namespace": "prod"}); len(got) != 1 {
		t.Fatal("full match must hit")
	}
}

func TestMatchRulesSkipsDisabledAndMalformed(t *testing.T) {
	disabled := rule(1, 10, `{}`, false)
	disabled.Enabled = false
	malformed := rule(2, 20, `{not json`, false)
	fallback := rule(3, 30, `{}`, false)
	matched := MatchRules([]model.RoutingRule{disabled, malformed, fallback}, map[string]string{})
	if len(matched) != 1 || matched[0].ID != 3 {
		t.Fatalf("matched = %+v, want only rule 3", matched)
	}
}

func TestMatchRulesNoMatch(t *testing.T) {
	rules := []model.RoutingRule{rule(1, 10, `{"severity":"critical"}`, false)}
	if got := MatchRules(rules, map[string]string{"severity": "info"}); len(got) != 0 {
		t.Fatalf("matched = %+v, want none", got)
	}
}
