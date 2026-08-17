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
)

// 飞书自定义机器人业务错误码（官方文档中的 webhook 错误码）
const (
	feishuCodeTokenInvalid = 19001 // webhook access token 无效
	feishuCodeSignFailed   = 19021 // 签名校验失败 / timestamp 过期
	feishuCodeIPNotAllowed = 19022 // IP 不在白名单
	feishuCodeKeywordMiss  = 19024 // 消息缺少关键词
	feishuCodeRateLimit    = 9499  // 请求过于频繁（限频）
)

// codeHints 为已知渠道错误码提供人类可读的排查提示（渠道间错误码区间
// 不同，共用一张表）
var codeHints = map[int]string{
	feishuCodeTokenInvalid: "webhook URL invalid or bot removed; check the bot webhook URL",
	feishuCodeSignFailed:   "signature check failed; check the sign secret",
	feishuCodeIPNotAllowed: "request IP not allowed; check the bot IP whitelist",
	feishuCodeKeywordMiss:  "required keyword missing in message; check channel keyword or template",
	feishuCodeRateLimit:    "rate limited by Feishu",
	dingTalkCodeRejected:   "dingtalk robot rejected the message (keyword missing, sign failed or IP not in whitelist); check errmsg",
	dingTalkCodeRateLimit:  "rate limited by DingTalk",
	weComCodeRateLimit:     "rate limited by WeCom",
}

// retryableCodes 是各渠道限频等瞬时业务错误码（可重试）
var retryableCodes = map[int]bool{
	feishuCodeRateLimit:   true,
	dingTalkCodeRateLimit: true,
	weComCodeRateLimit:    true,
}

// SendResult 是一次发送尝试的结果
type SendResult struct {
	// Err 是传输层错误（网络失败、超时） 本地准备阶段的错误用 Code=-1 表示
	Err error
	// HTTPStatus 是渠道网关的 HTTP 状态码（未触达时为 0）
	HTTPStatus int
	// Code 是渠道业务 code（0 = 成功，-1 = 不可用/本地错误）
	Code int
	// Msg 是渠道返回的消息或本地错误描述
	Msg string
}

// Success 报告消息是否被渠道接收
func (r SendResult) Success() bool {
	return r.Err == nil && r.Code == 0 && r.HTTPStatus >= 200 && r.HTTPStatus < 300
}

// Retryable 实现 FR-2.4：只有瞬时错误才重试——传输错误、HTTP 5xx/429、
// 渠道限频 明确的业务错误（签名错误、缺关键词、机器人被移除等）永不重试，
// 重试也不会成功，交给人工排查
func (r SendResult) Retryable() bool {
	if r.Err != nil {
		return true
	}
	if r.HTTPStatus == http.StatusTooManyRequests || r.HTTPStatus >= 500 {
		return true
	}
	return retryableCodes[r.Code]
}

// Message 组装写入 deliveries 的人类可读的成功/失败信息
func (r SendResult) Message() string {
	if r.Err != nil {
		return "send error: " + r.Err.Error()
	}
	if hint, ok := codeHints[r.Code]; ok {
		return fmt.Sprintf("%s (code %d: %s)", hint, r.Code, r.Msg)
	}
	return r.Msg
}

// Sender 把一条渲染好的消息发送到渠道
type Sender interface {
	Send(ctx context.Context, ch *model.Channel, payload []byte) SendResult
}

// FeishuSender 把渲染好的消息 POST 到飞书自定义机器人 webhook，并在配置时
// 附加签名（FR-2.2）
type FeishuSender struct {
	client *http.Client
}

// NewFeishuSender 创建带指定请求超时的 FeishuSender
func NewFeishuSender(timeout time.Duration) *FeishuSender {
	return &FeishuSender{
		client: &http.Client{Timeout: timeout},
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
	body := payload
	hasSecret := ch.Secret != ""
	if ch.AtAll || hasSecret {
		// AtAll 注入和加签都要改写消息体：一次 unmarshal/marshal 完成，
		// 先注入 @所有人，再加时间戳和签名
		var m map[string]any
		if err := json.Unmarshal(payload, &m); err != nil {
			return localResult("payload is not valid JSON: %v", err)
		}
		if ch.AtAll {
			injectFeishuAtAll(m)
		}
		if hasSecret {
			ts := time.Now().Unix()
			m["timestamp"] = strconv.FormatInt(ts, 10)
			m["sign"] = Sign(ch.Secret, ts)
		}
		var err error
		body, err = json.Marshal(m)
		if err != nil {
			return localResult("marshal payload: %v", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ch.WebhookURL, bytes.NewReader(body))
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

// feishuAtAllText 是飞书 @所有人的 lark_md 标记
const feishuAtAllText = `<at user_id="all">所有人</at>`

// injectFeishuAtAll 按 msg_type 把 @所有人注入消息体：text 消息往
// content.text 末尾追加；interactive 卡片往 card.elements 追加一个
// lark_md div；其他消息类型（post/image/share_chat）没有通用的注入位置，
// 不处理
func injectFeishuAtAll(m map[string]any) {
	switch m["msg_type"] {
	case "text":
		content, _ := m["content"].(map[string]any)
		if content == nil {
			return
		}
		text, _ := content["text"].(string)
		if text != "" {
			text += "\n"
		}
		content["text"] = text + feishuAtAllText
	case "interactive":
		card, _ := m["card"].(map[string]any)
		if card == nil {
			return
		}
		elements, _ := card["elements"].([]any)
		card["elements"] = append(elements, map[string]any{
			"tag":  "div",
			"text": map[string]any{"tag": "lark_md", "content": feishuAtAllText},
		})
	}
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
