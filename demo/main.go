package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

var cnt int

type greetReq struct {
	Name string `uri:"name" binding:"required"`
	Num  int    `form:"num" binding:"gt=0"`
}

type greetRsp struct {
	Greet string `json:"greet"`
	Count int    `json:"count"`
}

type mapSlice struct {
	MapSlice map[int][]string `json:"map_slice"`
}

func handleGreet(ctx context.Context, r *greetReq) (*greetRsp, error) {
	if r.Num > 10 {
		return nil, ginx.ErrWrap{Code: 1001, Msg: "request num should not greater than 10"}
	}

	cnt += r.Num
	res := greetRsp{Greet: fmt.Sprintf("hello %s!", r.Name), Count: cnt}
	return &res, nil
}

func handleGreetWithGinContext(ctx context.Context, r *greetReq) (*greetRsp, error) {
	if r.Num > 10 {
		return nil, ginx.ErrWrap{Code: 1001, Msg: "request num should not greater than 10"}
	}

	c, ok := ctx.(*gin.Context)
	if !ok {
		return nil, ginx.ErrWrap{Code: 1001, Msg: "invalid handle option"}
	}

	c.String(http.StatusOK, "hello %s, count: %d", r.Name, cnt)
	c.Abort()

	return nil, nil
}

func handleMapSlice(ctx context.Context, r *mapSlice) (*mapSlice, error) {
	return r, nil
}

func main() {
	e := gin.Default()
	ginx.SetServeDoc(true, "test_http")
	ginx.ServeDoc(e)

	g := e.Group("/api/v1")
	ginx.GET(g, "/test/:name", handleGreet)
	ginx.GET(g, "/test_no_data_wrap/:name", handleGreet, ginx.NoDataWrap)
	ginx.GET(g, "/test_gin_context/:name", handleGreetWithGinContext)
	ginx.POST(g, "/test_map_slice", handleMapSlice, ginx.NoPbParse)
	e.Run(":8081")
}
