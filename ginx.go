package ginx

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type HandleOption int8

const (
	DataWrap   HandleOption = 1  // 如果handle正常返回, 则返回的数据结构包在"data"字段内, 还有code=0和msg=""
	NoDataWrap HandleOption = -1 // 如果handle正常返回, 则返回的数据结构不包在"data"字段内, 没有code和msg字段

	NoPbParse HandleOption = -2 // 跳过pb注册

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
	dataWrap  bool
	noPbParse bool
	alwaysOK  bool
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

		case NoPbParse:
			cfg.noPbParse = true

		case StatusCodeAlwaysOK:
			cfg.alwaysOK = true
		}
	}
	return cfg
}

type iRouter interface {
	gin.IRouter
	BasePath() string
}

type Any map[string]any

type ErrWrap struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type rspWrap struct {
	ErrWrap
	Data any `json:"data,omitempty"`
}

func (e ErrWrap) Error() string {
	return e.Msg
}

func GET[Req any, Rsp any](router gin.IRoutes, path string, handle func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)
	registFuncMetadata(router, path, http.MethodGet, handle, cfg)
	var handler = parseFormHandler(cfg, handle)
	router.GET(path, handler)
}

func POST[Req any, Rsp any](router gin.IRoutes, path string, handle func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)
	registFuncMetadata(router, path, http.MethodPost, handle, cfg)
	var handler = parseBodyHandler(cfg, handle)
	router.POST(path, handler)
}

func PUT[Req any, Rsp any](router gin.IRoutes, path string, handle func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)
	registFuncMetadata(router, path, http.MethodPut, handle, cfg)
	var handler = parseBodyHandler(cfg, handle)
	router.PUT(path, handler)
}

func PATCH[Req any, Rsp any](router gin.IRoutes, path string, handle func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)
	registFuncMetadata(router, path, http.MethodPatch, handle, cfg)
	var handler = parseBodyHandler(cfg, handle)
	router.PATCH(path, handler)
}

func DELETE[Req any, Rsp any](router gin.IRoutes, path string, handle func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)
	registFuncMetadata(router, path, http.MethodDelete, handle, cfg)
	var handler = parseFormHandler(cfg, handle)
	router.DELETE(path, handler)
}

func parseFormHandler[Req any, Rsp any](cfg handleConfig, handle func(context.Context, *Req) (*Rsp, error)) func(c *gin.Context) {
	var handler = func(c *gin.Context) {
		var req Req
		statusCode := http.StatusOK
		c.ShouldBindUri(&req)
		if err := c.ShouldBindQuery(&req); err != nil {
			if !cfg.alwaysOK {
				statusCode = http.StatusBadRequest
			}
			c.AbortWithStatusJSON(statusCode, rspWrap{ErrWrap: ErrWrap{Code: defaultInvalidArgumentCode, Msg: err.Error()}, Data: nil})
			return
		}

		rsp, err := handle(c, &req)
		if c.IsAborted() {
			return
		} else if err != nil {
			if !cfg.alwaysOK {
				statusCode = http.StatusInternalServerError
			}
			if e, ok := err.(ErrWrap); ok {
				c.AbortWithStatusJSON(statusCode, e)
			} else {
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

func parseBodyHandler[Req any, Rsp any](cfg handleConfig, handle func(context.Context, *Req) (*Rsp, error)) func(c *gin.Context) {
	var handler = func(c *gin.Context) {
		var req Req
		statusCode := http.StatusOK
		c.ShouldBindUri(&req)
		c.ShouldBindQuery(&req)
		if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "application/json") {
			if err := c.ShouldBindJSON(&req); err != nil {
				if !cfg.alwaysOK {
					statusCode = http.StatusBadRequest
				}
				c.AbortWithStatusJSON(statusCode, rspWrap{ErrWrap: ErrWrap{Code: defaultInvalidArgumentCode, Msg: err.Error()}, Data: nil})
				return
			}
		}

		rsp, err := handle(c, &req)
		if c.IsAborted() {
			return
		} else if err != nil {
			if !cfg.alwaysOK {
				statusCode = http.StatusInternalServerError
			}
			if e, ok := err.(ErrWrap); ok {
				c.AbortWithStatusJSON(statusCode, e)
			} else {
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

func registFuncMetadata[Req any, Rsp any](router gin.IRoutes, path string, method string, handle func(context.Context, *Req) (*Rsp, error), cfg handleConfig) {
	if !cfg.noPbParse {
		var req Req
		var rsp Rsp
		if r, ok := router.(*gin.Engine); ok {
			regist(method, r, path, handle, req, rsp, cfg.dataWrap)
		} else if g, ok := router.(*gin.RouterGroup); ok {
			regist(method, g, path, handle, req, rsp, cfg.dataWrap)
		}
	}
}
