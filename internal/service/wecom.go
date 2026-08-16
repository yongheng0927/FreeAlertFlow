package service

import (
	"context"
	"net/http"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/crypto"
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
	cipher *crypto.Cipher
}

// NewWeComSender 创建带指定请求超时的 WeComSender
func NewWeComSender(cipher *crypto.Cipher, timeout time.Duration) *WeComSender {
	return &WeComSender{
		client: &http.Client{Timeout: timeout},
		cipher: cipher,
	}
}

func (s *WeComSender) Send(ctx context.Context, ch *model.Channel, payload []byte) SendResult {
	webhookURL, err := s.cipher.Decrypt(ch.WebhookURLEncrypted)
	if err != nil {
		return localResult("decrypt webhook url: %v", err)
	}
	respBody, httpStatus, err := postJSON(ctx, s.client, string(webhookURL), payload)
	if err != nil {
		return SendResult{Err: err, HTTPStatus: httpStatus}
	}
	return parseErrcodeResponse(httpStatus, respBody)
}
