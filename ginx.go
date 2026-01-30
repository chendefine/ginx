package ginx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type HandleOption int8

const (
	DataWrap   HandleOption = 1  // 如果handle正常返回, 则返回的数据结构包在"data"字段内, 还有code=0和msg=""
	NoDataWrap HandleOption = -1 // 如果handle正常返回, 则返回的数据结构不包在"data"字段内, 没有code和msg字段

	StatusCodeAlwaysOK HandleOption = -3 // http status code总是返回200 OK, 不管err返回的是nil还是error
)

var (
	defaultDataWrap = true // 默认返回的数据结构包在"data"字段内, 还有code=0和msg=""

	defaultInvalidArgumentCode     = 1
	defaultInternalServerErrorCode = 2
)

var EmptyHandler = func(context.Context, *struct{}) (*struct{}, error) { return nil, nil }

func SetDataWrap(b bool) {
	defaultDataWrap = b
}

func SetInvalidArgumentCode(n int) {
	defaultInvalidArgumentCode = n
}

func SetInternalServerErrorCode(n int) {
	defaultInternalServerErrorCode = n
}

func SetJsonDecoderUseNumber(b bool) {
	binding.EnableDecoderUseNumber = b
}

type handleConfig struct {
	dataWrap bool
	alwaysOK bool
}

func parseHandleOptions(opts []HandleOption) handleConfig {
	cfg := handleConfig{
		dataWrap: defaultDataWrap,
	}
	for _, opt := range opts {
		switch opt {
		case DataWrap:
			cfg.dataWrap = true
		case NoDataWrap:
			cfg.dataWrap = false
		case StatusCodeAlwaysOK:
			cfg.alwaysOK = true
		}
	}
	return cfg
}

type Any map[string]any

type rspWrap struct {
	ErrWrap
	Data any `json:"data,omitempty"`
}

func register[Req any, Rsp any](router gin.IRoutes, method string, path string, fn func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)
	handler := makeHandlerFunc(cfg, fn)
	router.Handle(method, path, handler)
}

func GET[Req any, Rsp any](router gin.IRoutes, path string, fn func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	register(router, http.MethodGet, path, fn, opts...)
}

func POST[Req any, Rsp any](router gin.IRoutes, path string, fn func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	register(router, http.MethodPost, path, fn, opts...)
}

func PUT[Req any, Rsp any](router gin.IRoutes, path string, fn func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	register(router, http.MethodPut, path, fn, opts...)
}

func PATCH[Req any, Rsp any](router gin.IRoutes, path string, fn func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	register(router, http.MethodPatch, path, fn, opts...)
}

func DELETE[Req any, Rsp any](router gin.IRoutes, path string, fn func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	register(router, http.MethodDelete, path, fn, opts...)
}

func makeHandlerFunc[Req any, Rsp any](cfg handleConfig, handle func(context.Context, *Req) (*Rsp, error)) func(c *gin.Context) {
	var handler = func(c *gin.Context) {
		var req Req
		statusCode := http.StatusOK
		c.ShouldBindHeader(&req)
		c.ShouldBindUri(&req)
		bindQueryErr := c.ShouldBindQuery(&req)
		if strings.Contains(c.ContentType(), "application/json") {
			if err := c.ShouldBindJSON(&req); err != nil {
				if !cfg.alwaysOK {
					statusCode = http.StatusBadRequest
				}
				c.AbortWithStatusJSON(statusCode, rspWrap{ErrWrap: ErrWrap{Code: defaultInvalidArgumentCode, Msg: err.Error()}, Data: nil})
				return
			}
		} else {
			if bindQueryErr != nil {
				if !cfg.alwaysOK {
					statusCode = http.StatusBadRequest
				}
				c.AbortWithStatusJSON(statusCode, rspWrap{ErrWrap: ErrWrap{Code: defaultInvalidArgumentCode, Msg: bindQueryErr.Error()}, Data: nil})
				return
			}
		}

		rsp, err := handle(c, &req)
		if c.IsAborted() {
			return
		} else if err != nil {
			var e *ErrWrap
			var ev ErrWrap
			if errors.As(err, &e) {
				if !cfg.alwaysOK {
					statusCode = http.StatusInternalServerError
					if e.HttpCode > 100 && e.HttpCode < 600 {
						statusCode = e.HttpCode
					}
				}
				c.AbortWithStatusJSON(statusCode, e)
			} else if errors.As(err, &ev) {
				if !cfg.alwaysOK {
					statusCode = http.StatusInternalServerError
					if ev.HttpCode > 100 && ev.HttpCode < 600 {
						statusCode = ev.HttpCode
					}
				}
				c.AbortWithStatusJSON(statusCode, &ev)
			} else {
				if !cfg.alwaysOK {
					statusCode = http.StatusInternalServerError
				}
				c.AbortWithStatusJSON(statusCode, rspWrap{ErrWrap: ErrWrap{Code: defaultInternalServerErrorCode, Msg: err.Error()}})
			}
		} else {
			if cfg.dataWrap {
				c.JSON(statusCode, rspWrap{Data: rsp})
			} else {
				c.JSON(statusCode, rsp)
			}
		}
	}
	return handler
}
