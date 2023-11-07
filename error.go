package ginx

import "fmt"

// ErrWrap 标准业务错误结构, JSON 序列化后即为统一响应体的 code/msg 字段.
// HttpCode 可选, 在 100~599 范围内才会作为实际 HTTP 状态码;
// 其它情况下由 Engine 的默认值或 ErrorHandler 决定.
type ErrWrap struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`

	HttpCode int `json:"-"`
}

// Error 构造一个 *ErrWrap. 默认不指定 HttpCode, 由 Engine 选择默认值(通常 500).
// 如需明确 HTTP 状态码, 请链式调用 .Status(n).
func Error(code int, msg string) *ErrWrap {
	return &ErrWrap{Code: code, Msg: msg}
}

// Status 设置 HTTP 状态码, 非 100~599 范围内的值被忽略. 返回新实例, 原对象不变.
func (e *ErrWrap) Status(httpCode int) *ErrWrap {
	cp := *e
	if httpCode > 100 && httpCode < 600 {
		cp.HttpCode = httpCode
	}
	return &cp
}

// Format 使用 fmt.Sprintf 语义将当前 Msg 作为模板填充参数, 返回新实例.
// 用于把 "user %s not found" 这类模板错误填充上下文.
func (e *ErrWrap) Format(args ...any) *ErrWrap {
	cp := *e
	cp.Msg = fmt.Sprintf(e.Msg, args...)
	return &cp
}

// Error 实现 error 接口.
func (e ErrWrap) Error() string { return e.Msg }

// Is 使得可以通过 errors.Is 判断两个业务错误 code 相同, 便于 sentinel 错误比较.
func (e *ErrWrap) Is(target error) bool {
	t, ok := target.(*ErrWrap)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
