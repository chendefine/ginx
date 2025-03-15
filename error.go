package ginx

import "fmt"

func Error(code int, msg string, httpCode ...int) *ErrWrap {
	ew := &ErrWrap{Code: code, Msg: msg}
	if len(httpCode) > 0 && httpCode[0] > 100 && httpCode[0] < 600 {
		ew.HttpCode = httpCode[0]
	}
	return ew
}

type ErrWrap struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`

	HttpCode int `json:"-"`
}

func (e ErrWrap) Error() string {
	return e.Msg
}

func (e *ErrWrap) Extend(vals ...any) *ErrWrap {
	msg := fmt.Sprintf(e.Msg, vals...)
	wrap := &ErrWrap{Code: e.Code, Msg: msg, HttpCode: e.HttpCode}
	return wrap
}
