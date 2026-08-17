package service

import (
	"context"
	"net/http"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// 企业微信群机器人业务错误码（官方文档中的 webhook 错误码，精简收录）
const (
	weComCodeRateLimit = 45009 // 接口调用频率超限
)

// WeComSender 把消息 POST 到企业微信群机器人 webhook。企微机器人无加签
// 机制，仅凭 webhook URL 中的 key 鉴权；企微机器人 markdown 也不支持
// @人，ch.AtAll 无从表达，直接忽略
type WeComSender struct {
	client *http.Client
}

// NewWeComSender 创建带指定请求超时的 WeComSender
func NewWeComSender(timeout time.Duration) *WeComSender {
	return &WeComSender{
		client: &http.Client{Timeout: timeout},
	}
}

func (s *WeComSender) Send(ctx context.Context, ch *model.Channel, payload []byte) SendResult {
	respBody, httpStatus, err := postJSON(ctx, s.client, ch.WebhookURL, payload)
	if err != nil {
		return SendResult{Err: err, HTTPStatus: httpStatus}
	}
	return parseErrcodeResponse(httpStatus, respBody)
}
