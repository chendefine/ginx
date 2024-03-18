package ginx

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var EmptyHandler = func(context.Context, *struct{}) (*struct{}, error) { return nil, nil }

func parseHandleOptions(opts []HandleOption) handleConfig {
	cfg := globalConfig

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

type iRouter interface {
	gin.IRouter
	BasePath() string
}

type ErrWrap struct {
	HttpStatus int `json:"-"`

	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type rspWrap struct {
	ErrWrap
	Data any `json:"data,omitempty"`
}

func (e ErrWrap) Error() string {
	return fmt.Sprintf("code: %d, msg: %s", e.Code, e.Msg)
}

func GET[Req any, Rsp any](router gin.IRoutes, path string, handle func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)

	if cfg.pbParse {
		var req Req
		var rsp Rsp
		if r, ok := router.(*gin.Engine); ok {
			regist(http.MethodPost, r, path, handle, req, rsp)
		} else if g, ok := router.(*gin.RouterGroup); ok {
			regist(http.MethodPost, g, path, handle, req, rsp)
		}
	}

	var handler = func(c *gin.Context) {
		req := new(Req)
		statusCode := http.StatusOK
		c.ShouldBindUri(req)
		if err := c.ShouldBindQuery(req); err != nil {
			if !cfg.alwaysOK {
				statusCode = http.StatusBadRequest
			}
			c.AbortWithStatusJSON(statusCode, rspWrap{ErrWrap: ErrWrap{Code: cfg.invalidArgumentCode, Msg: err.Error()}, Data: nil})
			return
		}

		rsp, err := handle(c, req)
		if c.IsAborted() {
			return
		} else if err != nil {
			if !cfg.alwaysOK {
				statusCode = http.StatusInternalServerError
			}
			switch e := err.(type) {
			case ErrWrap:
				if !cfg.alwaysOK && e.HttpStatus > 0 {
					statusCode = e.HttpStatus
				}
				c.AbortWithStatusJSON(statusCode, e)
			case *ErrWrap:
				if !cfg.alwaysOK && e.HttpStatus > 0 {
					statusCode = e.HttpStatus
				}
				c.AbortWithStatusJSON(statusCode, e)
			default:
				c.AbortWithStatusJSON(statusCode, ErrWrap{Code: cfg.internalServerErrorCode, Msg: err.Error()})
			}
		} else {
			if cfg.dataWrap {
				c.JSON(statusCode, rspWrap{Data: rsp})
			} else {
				c.JSON(statusCode, rsp)
			}
		}
	}
	router.GET(path, handler)
}

func POST[Req any, Rsp any](router gin.IRoutes, path string, handle func(context.Context, *Req) (*Rsp, error), opts ...HandleOption) {
	cfg := parseHandleOptions(opts)

	if cfg.pbParse {
		var req Req
		var rsp Rsp
		if r, ok := router.(*gin.Engine); ok {
			regist(http.MethodPost, r, path, handle, req, rsp)
		} else if g, ok := router.(*gin.RouterGroup); ok {
			regist(http.MethodPost, g, path, handle, req, rsp)
		}
	}

	var handler = func(c *gin.Context) {
		req := new(Req)
		statusCode := http.StatusOK
		c.ShouldBindUri(req)
		c.ShouldBindQuery(req)
		if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "application/json") {
			if err := c.ShouldBindJSON(req); err != nil {
				if !cfg.alwaysOK {
					statusCode = http.StatusBadRequest
				}
				c.AbortWithStatusJSON(statusCode, ErrWrap{Code: cfg.internalServerErrorCode, Msg: err.Error()})
				return
			}
		}

		rsp, err := handle(c, req)
		if c.IsAborted() {
			return
		} else if err != nil {
			if !cfg.alwaysOK {
				statusCode = http.StatusInternalServerError
			}
			switch e := err.(type) {
			case ErrWrap:
				if !cfg.alwaysOK && e.HttpStatus > 0 {
					statusCode = e.HttpStatus
				}
				c.AbortWithStatusJSON(statusCode, e)
			case *ErrWrap:
				if !cfg.alwaysOK && e.HttpStatus > 0 {
					statusCode = e.HttpStatus
				}
				c.AbortWithStatusJSON(statusCode, e)
			default:
				c.AbortWithStatusJSON(statusCode, rspWrap{ErrWrap: ErrWrap{Code: cfg.internalServerErrorCode, Msg: err.Error()}})
			}
		} else {
			if cfg.dataWrap {
				c.JSON(statusCode, rspWrap{Data: rsp})
			} else {
				c.JSON(statusCode, rsp)
			}
		}
	}
	router.POST(path, handler)
}
