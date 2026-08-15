package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/crypto"
)

// 飞书自定义机器人业务错误码（官方文档中的 webhook 错误码）
const (
	feishuCodeTokenInvalid = 19001 // webhook access token 无效
	feishuCodeSignFailed   = 19021 // 签名校验失败 / timestamp 过期
	feishuCodeIPNotAllowed = 19022 // IP 不在白名单
	feishuCodeKeywordMiss  = 19024 // 消息缺少关键词
	feishuCodeRateLimit    = 9499  // 请求过于频繁（限频）
)

// codeHints 为已知飞书错误码提供人类可读的排查提示
var codeHints = map[int]string{
	feishuCodeTokenInvalid: "webhook URL invalid or bot removed; check the bot webhook URL",
	feishuCodeSignFailed:   "signature check failed; check the sign secret",
	feishuCodeIPNotAllowed: "request IP not allowed; check the bot IP whitelist",
	feishuCodeKeywordMiss:  "required keyword missing in message; check channel keyword or template",
	feishuCodeRateLimit:    "rate limited by Feishu",
}

// SendResult 是一次发送尝试的结果
type SendResult struct {
	// Err 是传输层错误（网络失败、超时） 本地准备阶段的错误用 Code=-1 表示
	Err error
	// HTTPStatus 是飞书网关的 HTTP 状态码（未触达时为 0）
	HTTPStatus int
	// Code 是飞书业务 code（0 = 成功，-1 = 不可用/本地错误）
	Code int
	// Msg 是飞书返回的 msg 或本地错误描述
	Msg string
}

// Success 报告消息是否被飞书接收
func (r SendResult) Success() bool {
	return r.Err == nil && r.Code == 0 && r.HTTPStatus >= 200 && r.HTTPStatus < 300
}

// Retryable 实现 FR-2.4：只有瞬时错误才重试——传输错误、HTTP 5xx/429、
// 飞书限频 明确的业务错误（签名错误、缺关键词、机器人被移除等）永不重试，
// 重试也不会成功，交给人工排查
func (r SendResult) Retryable() bool {
	if r.Err != nil {
		return true
	}
	if r.HTTPStatus == http.StatusTooManyRequests || r.HTTPStatus >= 500 {
		return true
	}
	return r.Code == feishuCodeRateLimit
}

// Message 组装写入 deliveries 的人类可读的成功/失败信息
func (r SendResult) Message() string {
	if r.Err != nil {
		return "send error: " + r.Err.Error()
	}
	if hint, ok := codeHints[r.Code]; ok {
		return fmt.Sprintf("%s (feishu code %d: %s)", hint, r.Code, r.Msg)
	}
	return r.Msg
}

// Sender 把一条渲染好的消息发送到渠道
type Sender interface {
	Send(ctx context.Context, ch *model.Channel, payload []byte) SendResult
}

// FeishuSender 把渲染好的消息 POST 到飞书自定义机器人 webhook，负责解密
// 凭证并在配置时附加签名（FR-2.2）
type FeishuSender struct {
	client *http.Client
	cipher *crypto.Cipher
}

// NewFeishuSender 创建带指定请求超时的 FeishuSender
func NewFeishuSender(cipher *crypto.Cipher, timeout time.Duration) *FeishuSender {
	return &FeishuSender{
		client: &http.Client{Timeout: timeout},
		cipher: cipher,
	}
}

// Sign 计算飞书自定义机器人签名（FR-2.2）：stringToSign =
// timestamp + "\n" + secret，以它为 HMAC-SHA256 的密钥对空消息签名，
// 再 base64 编码
func Sign(secret string, timestamp int64) string {
	stringToSign := strconv.FormatInt(timestamp, 10) + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	mac.Write([]byte{})
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// localResult 为本地准备阶段的错误构造不可重试的结果
func localResult(format string, args ...any) SendResult {
	return SendResult{Code: -1, Msg: fmt.Sprintf(format, args...)}
}

func (s *FeishuSender) Send(ctx context.Context, ch *model.Channel, payload []byte) SendResult {
	url, err := s.cipher.Decrypt(ch.WebhookURLEncrypted)
	if err != nil {
		return localResult("decrypt webhook url: %v", err)
	}

	body := payload
	if ch.SecretEncrypted != nil && len(*ch.SecretEncrypted) > 0 {
		secret, err := s.cipher.Decrypt(*ch.SecretEncrypted)
		if err != nil {
			return localResult("decrypt sign secret: %v", err)
		}
		ts := time.Now().Unix()
		var m map[string]any
		if err := json.Unmarshal(payload, &m); err != nil {
			return localResult("payload is not valid JSON: %v", err)
		}
		m["timestamp"] = strconv.FormatInt(ts, 10)
		m["sign"] = Sign(string(secret), ts)
		body, err = json.Marshal(m)
		if err != nil {
			return localResult("marshal signed payload: %v", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, string(url), bytes.NewReader(body))
	if err != nil {
		return localResult("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.client.Do(req)
	if err != nil {
		return SendResult{Err: err}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return SendResult{Err: err, HTTPStatus: resp.StatusCode}
	}
	return parseFeishuResponse(resp.StatusCode, respBody)
}

// feishuResponse 同时兼容新版 {"code","msg"} 和旧版
// {"StatusCode","StatusMessage"} 两种机器人 webhook 响应结构
type feishuResponse struct {
	Code          *int   `json:"code"`
	Msg           string `json:"msg"`
	StatusCode    *int   `json:"StatusCode"`
	StatusMessage string `json:"StatusMessage"`
}

func parseFeishuResponse(httpStatus int, body []byte) SendResult {
	var fr feishuResponse
	if err := json.Unmarshal(body, &fr); err != nil {
		return SendResult{
			HTTPStatus: httpStatus,
			Code:       -1,
			Msg:        fmt.Sprintf("unparsable feishu response: %s", truncateRunes(string(body), 200)),
		}
	}
	res := SendResult{HTTPStatus: httpStatus, Code: -1, Msg: fr.Msg}
	if fr.Code != nil {
		res.Code = *fr.Code
	} else if fr.StatusCode != nil {
		res.Code = *fr.StatusCode
		res.Msg = fr.StatusMessage
	}
	return res
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
