package render

import "time"

// SampleContext 返回一个有代表性的模板上下文，用于保存时校验，
// 也是预览的最终兜底
func SampleContext() *Context {
	return &Context{
		Version:  "4",
		Status:   "firing",
		Receiver: "freealertflow",
		GroupKey: "{}:{alertname=\"HighCPU\"}",
		GroupLabels: map[string]string{
			"alertname": "HighCPU",
		},
		CommonLabels: map[string]string{
			"alertname": "HighCPU",
			"severity":  "critical",
			"instance":  "10.0.0.1:9100",
			"namespace": "prod",
		},
		CommonAnnotations: map[string]string{
			"summary": "CPU usage above 90% for 5m",
		},
		ExternalURL: "http://alertmanager:9093",
		Alerts: []Alert{
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "HighCPU",
					"severity":  "critical",
					"instance":  "10.0.0.1:9100",
				},
				Annotations: map[string]string{
					"summary": "CPU usage above 90% for 5m",
				},
				StartsAt:    time.Date(2026, 8, 15, 2, 0, 0, 0, time.UTC),
				Fingerprint: "0123456789abcdef",
			},
		},
		SourceName: "生产环境 Prometheus",
		RootURL:    "https://alerts.example.com/",
		DetailURL:  "https://alerts.example.com/alerts/42",
	}
}
