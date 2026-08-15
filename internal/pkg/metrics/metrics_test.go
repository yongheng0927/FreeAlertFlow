package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestMetricsRegistered 向每个采集器写入数据，并验证这些指标族都出现在
// 默认 registry 中（NFR-3）
func TestMetricsRegistered(t *testing.T) {
	ObserveAlertsReceived("prod", 3)
	IncDisposition("deduped")
	IncDisposition("unmatched")
	ObserveDelivery("值班群", "success", 0.42)
	ObserveDelivery("值班群", "failed", 1.5)
	IncManualResend("success")

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	seen := map[string]bool{}
	for _, f := range families {
		seen[f.GetName()] = true
	}
	for _, name := range []string{
		"faf_alerts_received_total",
		"faf_alert_disposition_total",
		"faf_deliveries_total",
		"faf_delivery_duration_seconds",
		"faf_manual_resends_total",
	} {
		if !seen[name] {
			t.Errorf("metric %s not registered", name)
		}
	}

	// 抽查 deliveries 计数器上的标签值
	var found bool
	for _, f := range families {
		if f.GetName() != "faf_deliveries_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			var labelText strings.Builder
			for _, l := range m.GetLabel() {
				labelText.WriteString(l.GetName() + "=" + l.GetValue() + ";")
			}
			if strings.Contains(labelText.String(), "channel=值班群") && strings.Contains(labelText.String(), "status=success") {
				found = true
				if m.GetCounter().GetValue() < 1 {
					t.Error("counter must be >= 1 after ObserveDelivery")
				}
			}
		}
	}
	if !found {
		t.Error("deliveries counter with channel/status labels not found")
	}
}
