package main

import (
	"context"
	"fmt"
	"mime/multipart"
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

type uploadReq struct {
	Name string                `form:"name" binding:"required"`
	File *multipart.FileHeader `form:"file" binding:"required"`
}

func handleGreet(ctx context.Context, r *greetReq) (*greetRsp, error) {
	res := greetRsp{Greet: fmt.Sprintf("hello %s!", r.Name), Num1: r.Num1, Num2: r.Num2, Head: r.Head}
	return &res, nil
}

func handleGreetString(ctx context.Context, r *greetReq) (*ginx.StringRsp, error) {
	return ginx.StringResponse(http.StatusOK, "hello %s, num1: %d, num2: %d, head: %s", r.Name, r.Num1, r.Num2, r.Head), nil
}

func handleRedirect(_ context.Context, _ *struct{}) (*ginx.RedirectRsp, error) {
	return ginx.RedirectResponse(http.StatusFound, "/api/v1/test/world?num1=1&num2=2"), nil
}

func handleMapSlice(ctx context.Context, r *mapSlice) (*mapSlice, error) {
	return r, nil
}

func handleUpload(ctx context.Context, r *uploadReq) (*greetRsp, error) {
	return &greetRsp{Greet: r.Name + ":" + r.File.Filename}, nil
}

func main() {
	e := gin.Default()
	engine := ginx.New(
		ginx.WithInvalidArgCode(10001),
		ginx.WithSuccessHandler(func(ctx context.Context, data any) (int, any) {
			return http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data}
		}),
		ginx.WithErrorHandler(func(ctx context.Context, err error) (int, any) {
			return http.StatusInternalServerError, gin.H{"code": 10002, "msg": err.Error()}
		}),
	)

	g := engine.Wrap(e.Group("/api/v1"))
	ginx.GET(g, "/test/:name", handleGreet)
	ginx.GET(g, "/test_no_data_wrap/:name", handleGreet, ginx.NoDataWrap())
	ginx.GET(g, "/test_string/:name", handleGreetString)
	ginx.GET(g, "/redirect", handleRedirect)
	ginx.POST(g, "/test_map_slice", handleMapSlice)
	ginx.POST(g, "/upload", handleUpload)
	ginx.SSE(g, "/events", func(ctx context.Context, req *struct{}, send ginx.Sender) error {
		return send(ginx.Event{Event: "message", Data: gin.H{"hello": "world"}})
	})

	e.Run(":8081")
}
