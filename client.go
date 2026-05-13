package ginx

import "encoding/json"

type dataWrapper struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// ParseResponse 解析 HTTP 响应体, 兼容 DataWrap 和 NoDataWrap 两种模式.
//   - HTTP 错误 + 空 body → 返回 *ErrWrap{HttpCode}
//   - body 为 {code, msg, data} 格式且 code != 0 → 返回 *ErrWrap 业务错误
//   - body 为 {code:0, data:...} 格式 → 从 data 字段反序列化 result
//   - body 非 wrapper 格式 + HTTP 错误 → 返回 *ErrWrap{HttpCode, Msg: body}
//   - body 非 wrapper 格式 + HTTP 成功 → 直接反序列化 body 到 result
func ParseResponse(statusCode int, body []byte, result any) error {
	if len(body) == 0 {
		if statusCode >= 400 {
			return &ErrWrap{HttpCode: statusCode}
		}
		return nil
	}

	var wrapper dataWrapper
	if err := json.Unmarshal(body, &wrapper); err == nil && (wrapper.Code != 0 || wrapper.Data != nil) {
		if wrapper.Code != 0 {
			return &ErrWrap{Code: wrapper.Code, Msg: wrapper.Msg, HttpCode: statusCode}
		}
		if result != nil && wrapper.Data != nil {
			return json.Unmarshal(wrapper.Data, result)
		}
		return nil
	}

	if statusCode >= 400 {
		return &ErrWrap{Code: -1, Msg: string(body), HttpCode: statusCode}
	}
	if result != nil {
		return json.Unmarshal(body, result)
	}
	return nil
}
