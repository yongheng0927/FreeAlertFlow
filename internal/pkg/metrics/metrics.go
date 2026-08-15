// Package metrics 存放 Prometheus 采集器（NFR-3）以及供 service 层记录
// 指标的辅助函数，所有辅助函数都可以安全地在测试中调用
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// AlertsReceived 按 source 统计已入库的 webhook 告警数
	AlertsReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "faf_alerts_received_total",
		Help: "Alerts accepted by the webhook endpoint.",
	}, []string{"source"})

	// AlertDisposition 按派发结果（delivered/deduped/unmatched）计数
	AlertDisposition = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "faf_alert_disposition_total",
		Help: "Alert dispatch outcomes.",
	}, []string{"disposition"})

	// Deliveries 按 channel 统计发送结果
	Deliveries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "faf_deliveries_total",
		Help: "Delivery results by channel and status.",
	}, []string{"channel", "status"})

	// DeliveryDuration 是按 channel 统计的发送耗时（秒）
	DeliveryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "faf_delivery_duration_seconds",
		Help:    "Delivery latency by channel.",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"channel"})

	// ManualResends 按结果统计手动重发次数（FR-2.6）
	ManualResends = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "faf_manual_resends_total",
		Help: "Manual resends by result.",
	}, []string{"status"})
)

// ObserveAlertsReceived 记录某个 source 接入的 n 条告警
func ObserveAlertsReceived(source string, n int) {
	AlertsReceived.WithLabelValues(source).Add(float64(n))
}

// IncDisposition 记录一次派发结果
func IncDisposition(disposition string) {
	AlertDisposition.WithLabelValues(disposition).Inc()
}

// ObserveDelivery 记录一次投递结果及其耗时（秒）
func ObserveDelivery(channel, status string, seconds float64) {
	Deliveries.WithLabelValues(channel, status).Inc()
	DeliveryDuration.WithLabelValues(channel).Observe(seconds)
}

// IncManualResend 按结果状态记录一次手动重发
func IncManualResend(status string) {
	ManualResends.WithLabelValues(status).Inc()
}
