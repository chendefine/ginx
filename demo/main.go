package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

type greetReq struct {
	Name string `uri:"name" binding:"required"`
	Head string `header:"X-Head" binding:"required"`
	Num1 int    `form:"num1" binding:"gt=0"`
	Num2 int    `form:"num2" binding:"gt=0"`
}

type greetRsp struct {
	Greet string `json:"greet"`
	Head  string `json:"head"`
	Num1  int    `json:"num1"`
	Num2  int    `json:"num2"`
}

type mapSlice struct {
	MapSlice map[int][]string `json:"map_slice"`
}

func handleGreet(ctx context.Context, r *greetReq) (*greetRsp, error) {
	res := greetRsp{Greet: fmt.Sprintf("hello %s!", r.Name), Num1: r.Num1, Num2: r.Num2, Head: r.Head}
	return &res, nil
}

func handleGreetWithGinContext(ctx context.Context, r *greetReq) (*greetRsp, error) {
	c, ok := ctx.(*gin.Context)
	if !ok {
		return nil, ginx.ErrWrap{Code: 1001, Msg: "invalid handle option"}
	}

	c.String(http.StatusOK, "hello %s, num1: %d, num2: %d, head: %s", r.Name, r.Num1, r.Num2, r.Head)
	c.Abort()

	return nil, nil
}

func handleMapSlice(ctx context.Context, r *mapSlice) (*mapSlice, error) {
	return r, nil
}

func main() {
	e := gin.Default()

	g := e.Group("/api/v1")
	ginx.GET(g, "/test/:name", handleGreet)
	ginx.GET(g, "/test_no_data_wrap/:name", handleGreet, ginx.NoDataWrap)
	ginx.GET(g, "/test_gin_context/:name", handleGreetWithGinContext)
	ginx.POST(g, "/test_map_slice", handleMapSlice)
	e.Run(":8081")
}
