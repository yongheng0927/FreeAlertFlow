package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrBadPayload 表示无法解析或不支持的 webhook 消息体（HTTP 400）
var ErrBadPayload = errors.New("invalid Alertmanager webhook payload")

// AMWebhook 是 Alertmanager v4 webhook 消息（FR-1.1）
type AMWebhook struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []AMAlert         `json:"alerts"`
}

// AMAlert 是 v4 webhook 消息中的单条告警
type AMAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// ParseAMWebhook 解析并校验 Alertmanager v4 webhook 消息体
func ParseAMWebhook(data []byte) (*AMWebhook, error) {
	var msg AMWebhook
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadPayload, err)
	}
	if msg.Version != "4" {
		return nil, fmt.Errorf("%w: unsupported version %q", ErrBadPayload, msg.Version)
	}
	if len(msg.Alerts) == 0 {
		return nil, fmt.Errorf("%w: empty alerts", ErrBadPayload)
	}
	return &msg, nil
}
