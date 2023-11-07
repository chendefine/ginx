package ginx

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type HandleOption int8

const (
	DataWrap   HandleOption = 1  // 如果handle正常返回, 则返回的数据结构包在"data"字段内, 还有code=0和msg=""
	NoDataWrap HandleOption = -1 // 如果handle正常返回, 则返回的数据结构不包在"data"字段内, 没有code和msg字段

	NoPbParse HandleOption = -2 // 跳过pb注册
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

type handleConfig struct {
	dataWrap  bool
	noPbParse bool
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
		}
	}
	return cfg
}

type iRouter interface {
	gin.IRouter
	BasePath() string
}

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

	if !cfg.noPbParse {
		var req Req
		var rsp Rsp
		if r, ok := router.(*gin.Engine); ok {
			regist(http.MethodPost, r, path, handle, req, rsp)
		} else if g, ok := router.(*gin.RouterGroup); ok {
			regist(http.MethodPost, g, path, handle, req, rsp)
		}
	}

	var handler = func(c *gin.Context) {
		var req Req
		c.ShouldBindUri(&req)
		if err := c.ShouldBindQuery(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, rspWrap{ErrWrap: ErrWrap{Code: defaultInvalidArgumentCode, Msg: err.Error()}, Data: nil})
			return
		}

		rsp, err := handle(c, &req)
		if c.IsAborted() {
			return
		} else if err != nil {
			if e, ok := err.(ErrWrap); ok {
				c.AbortWithStatusJSON(http.StatusInternalServerError, e)
			} else {
				c.AbortWithStatusJSON(http.StatusInternalServerError, rspWrap{ErrWrap: ErrWrap{Code: defaultInternalServerErrorCode, Msg: err.Error()}})
			}
		} else {
			if cfg.dataWrap {
				c.JSON(http.StatusOK, rspWrap{Data: rsp})
			} else {
				c.JSON(http.StatusOK, rsp)
			}
		}
	}
	router.GET(path, handler)
}

func POST[Req any, Rsp any](router gin.IRoutes, path string, handle func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)

	if !cfg.noPbParse {
		var req Req
		var rsp Rsp
		if r, ok := router.(*gin.Engine); ok {
			regist(http.MethodPost, r, path, handle, req, rsp)
		} else if g, ok := router.(*gin.RouterGroup); ok {
			regist(http.MethodPost, g, path, handle, req, rsp)
		}
	}

	var handler = func(c *gin.Context) {
		var req Req
		c.ShouldBindUri(&req)
		c.ShouldBindQuery(&req)
		if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "application/json") {
			if err := c.ShouldBindJSON(&req); err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, rspWrap{ErrWrap: ErrWrap{Code: defaultInvalidArgumentCode, Msg: err.Error()}, Data: nil})
				return
			}
		}

		rsp, err := handle(c, &req)
		if c.IsAborted() {
			return
		} else if err != nil {
			if e, ok := err.(ErrWrap); ok {
				c.AbortWithStatusJSON(http.StatusInternalServerError, e)
			} else {
				c.AbortWithStatusJSON(http.StatusInternalServerError, rspWrap{ErrWrap: ErrWrap{Code: defaultInternalServerErrorCode, Msg: err.Error()}})
			}
		} else {
			if cfg.dataWrap {
				c.JSON(http.StatusOK, rspWrap{Data: rsp})
			} else {
				c.JSON(http.StatusOK, rsp)
			}
		}
	}
	router.POST(path, handler)
}
