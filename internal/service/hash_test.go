package service

import (
	"testing"
)

func TestContentHashDeterministicAndOrderIndependent(t *testing.T) {
	l1 := map[string]string{"a": "1", "b": "2", "c": "3"}
	l2 := map[string]string{"c": "3", "a": "1", "b": "2"}
	a1 := map[string]string{"x": "9"}
	a2 := map[string]string{"x": "9"}

	h1 := ContentHash("firing", l1, a1)
	h2 := ContentHash("firing", l2, a2)
	if h1 != h2 {
		t.Fatal("content hash must be independent of map iteration order")
	}
	if len(h1) != 64 {
		t.Fatalf("hash length = %d, want 64", len(h1))
	}
}

func TestContentHashSensitivity(t *testing.T) {
	base := ContentHash("firing", map[string]string{"a": "1"}, map[string]string{"x": "9"})
	cases := map[string]string{
		"status changed":      ContentHash("resolved", map[string]string{"a": "1"}, map[string]string{"x": "9"}),
		"label value changed": ContentHash("firing", map[string]string{"a": "2"}, map[string]string{"x": "9"}),
		"label key added":     ContentHash("firing", map[string]string{"a": "1", "b": "2"}, map[string]string{"x": "9"}),
		"annotation changed":  ContentHash("firing", map[string]string{"a": "1"}, map[string]string{"x": "8"}),
		"annotations empty":   ContentHash("firing", map[string]string{"a": "1"}, nil),
	}
	for name, h := range cases {
		if h == base {
			t.Errorf("%s must change the content hash", name)
		}
	}
}

func TestFingerprintFromLabels(t *testing.T) {
	f1 := FingerprintFromLabels(map[string]string{"a": "1", "b": "2"})
	f2 := FingerprintFromLabels(map[string]string{"b": "2", "a": "1"})
	if f1 != f2 {
		t.Fatal("fingerprint must be independent of key order")
	}
	if len(f1) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(f1))
	}
	f3 := FingerprintFromLabels(map[string]string{"a": "1", "b": "3"})
	if f1 == f3 {
		t.Fatal("different labels must produce different fingerprints")
	}
}

func TestParseAMWebhook(t *testing.T) {
	msg, err := ParseAMWebhook([]byte(sampleWebhookJSON))
	if err != nil {
		t.Fatalf("ParseAMWebhook: %v", err)
	}
	if msg.Version != "4" || msg.Status != "firing" {
		t.Fatalf("unexpected message: %+v", msg)
	}
	if len(msg.Alerts) != 1 {
		t.Fatalf("alerts = %d, want 1", len(msg.Alerts))
	}
	a := msg.Alerts[0]
	if a.Labels["alertname"] != "HighCPU" || a.Annotations["summary"] == "" {
		t.Fatalf("unexpected alert: %+v", a)
	}
	if a.StartsAt.IsZero() {
		t.Fatal("startsAt must be parsed")
	}
	if !a.EndsAt.IsZero() {
		t.Fatal("endsAt 0001-01-01 must stay zero (stored as NULL)")
	}
}

func TestParseAMWebhookRejects(t *testing.T) {
	if _, err := ParseAMWebhook([]byte("not json")); err == nil {
		t.Fatal("invalid JSON must fail")
	}
	if _, err := ParseAMWebhook([]byte(`{"version":"3","alerts":[{}]}`)); err == nil {
		t.Fatal("unsupported version must fail")
	}
	if _, err := ParseAMWebhook([]byte(`{"version":"4","alerts":[]}`)); err == nil {
		t.Fatal("empty alerts must fail")
	}
}

// sampleWebhookJSON 是一个有代表性的 Alertmanager v4 webhook 消息体
const sampleWebhookJSON = `{
  "version": "4",
  "groupKey": "{}:{alertname=\"HighCPU\"}",
  "truncatedAlerts": 0,
  "status": "firing",
  "receiver": "freealertflow",
  "groupLabels": {"alertname": "HighCPU"},
  "commonLabels": {
    "alertname": "HighCPU",
    "severity": "critical",
    "instance": "10.0.0.1:9100",
    "namespace": "prod"
  },
  "commonAnnotations": {
    "summary": "CPU usage above 90% for 5m",
    "description": "Node 10.0.0.1 CPU is on fire"
  },
  "externalURL": "http://alertmanager:9093",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighCPU",
        "severity": "critical",
        "instance": "10.0.0.1:9100",
        "namespace": "prod"
      },
      "annotations": {
        "summary": "CPU usage above 90% for 5m",
        "description": "Node 10.0.0.1 CPU is on fire"
      },
      "startsAt": "2026-08-15T02:00:00.000Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus:9090/graph?g0.expr=up",
      "fingerprint": "0123456789abcdef"
    }
  ]
}`
