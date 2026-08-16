package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// Dispatcher 按渠道类型把消息分发到对应的 Sender
type Dispatcher struct {
	feishu   Sender
	dingtalk Sender
	wecom    Sender
}

// NewDispatcher 创建三路分发的 Dispatcher
func NewDispatcher(feishu, dingtalk, wecom Sender) *Dispatcher {
	return &Dispatcher{feishu: feishu, dingtalk: dingtalk, wecom: wecom}
}

// Send 按 ch.Type 选择具体渠道 sender；payload 是模板渲染出的该渠道完整
// 消息体（AtAll 由 Sender 层注入）
func (d *Dispatcher) Send(ctx context.Context, ch *model.Channel, payload []byte) SendResult {
	switch ch.Type {
	case model.ChannelTypeFeishu:
		return d.feishu.Send(ctx, ch, payload)
	case model.ChannelTypeDingTalk:
		return d.dingtalk.Send(ctx, ch, payload)
	case model.ChannelTypeWeCom:
		return d.wecom.Send(ctx, ch, payload)
	default:
		return localResult("unsupported channel type %q", ch.Type)
	}
}

// postJSON 向 webhook POST JSON 消息体，返回响应体与 HTTP 状态码；
// 传输层错误时 err 非 nil（状态码未触达时为 0）
func postJSON(ctx context.Context, client *http.Client, url string, payload []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// errcodeResponse 是钉钉/企微机器人 webhook 共用的响应结构
type errcodeResponse struct {
	ErrCode *int   `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// parseErrcodeResponse 解析 {"errcode":0,"errmsg":"ok"} 风格的响应
func parseErrcodeResponse(httpStatus int, body []byte) SendResult {
	var er errcodeResponse
	if err := json.Unmarshal(body, &er); err != nil {
		return SendResult{
			HTTPStatus: httpStatus,
			Code:       -1,
			Msg:        fmt.Sprintf("unparsable channel response: %s", truncateRunes(string(body), 200)),
		}
	}
	res := SendResult{HTTPStatus: httpStatus, Code: -1, Msg: er.ErrMsg}
	if er.ErrCode != nil {
		res.Code = *er.ErrCode
	}
	return res
}
