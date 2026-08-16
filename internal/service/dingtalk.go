package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/crypto"
)

// 钉钉自定义机器人业务错误码（官方文档中的 webhook 错误码，精简收录）
const (
	dingTalkCodeRejected  = 310000 // 关键词缺失 / 签名校验失败 / IP 不在白名单等
	dingTalkCodeRateLimit = 130101 // 发送频率超限（每分钟 20 条）
)

// DingTalkSender 把消息 POST 到钉钉自定义机器人 webhook（FR-2.2 扩展渠道）
type DingTalkSender struct {
	client *http.Client
	cipher *crypto.Cipher
}

// NewDingTalkSender 创建带指定请求超时的 DingTalkSender
func NewDingTalkSender(cipher *crypto.Cipher, timeout time.Duration) *DingTalkSender {
	return &DingTalkSender{
		client: &http.Client{Timeout: timeout},
		cipher: cipher,
	}
}

// DingTalkSign 计算钉钉自定义机器人加签：以 secret 为密钥对
// "timestamp(毫秒)\nsecret" 做 HMAC-SHA256，再 base64 编码（机器人 secret
// 以 SEC 开头，照签即可）
func DingTalkSign(secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "\n" + secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *DingTalkSender) Send(ctx context.Context, ch *model.Channel, payload []byte) SendResult {
	webhookURL, err := s.cipher.Decrypt(ch.WebhookURLEncrypted)
	if err != nil {
		return localResult("decrypt webhook url: %v", err)
	}

	rawURL := string(webhookURL)
	if ch.SecretEncrypted != nil && len(*ch.SecretEncrypted) > 0 {
		secret, err := s.cipher.Decrypt(*ch.SecretEncrypted)
		if err != nil {
			return localResult("decrypt sign secret: %v", err)
		}
		// 加签参数拼在 webhook URL 上：&timestamp=<毫秒>&sign=<urlencoded>
		ts := time.Now().UnixMilli()
		sign := url.QueryEscape(DingTalkSign(string(secret), ts))
		sep := "&"
		if !strings.Contains(rawURL, "?") {
			sep = "?"
		}
		rawURL += sep + "timestamp=" + strconv.FormatInt(ts, 10) + "&sign=" + sign
	}

	body := payload
	if ch.AtAll {
		// @所有人在消息体里以 at.isAtAll 表达，合并进渲染出的 payload
		var m map[string]any
		if err := json.Unmarshal(payload, &m); err != nil {
			return localResult("payload is not valid JSON: %v", err)
		}
		m["at"] = map[string]any{"isAtAll": true}
		body, err = json.Marshal(m)
		if err != nil {
			return localResult("marshal payload: %v", err)
		}
	}

	respBody, httpStatus, err := postJSON(ctx, s.client, rawURL, body)
	if err != nil {
		return SendResult{Err: err, HTTPStatus: httpStatus}
	}
	return parseErrcodeResponse(httpStatus, respBody)
}
