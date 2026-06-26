# ginx

`ginx` 是一个基于 Go 泛型的 [Gin](https://github.com/gin-gonic/gin) 类型安全 HTTP Handler 包装库。

它把 Gin Handler 收敛成统一的 RPC 风格签名：

```go
func(ctx context.Context, req *Req) (*Rsp, error)
```

`ginx` 负责请求绑定、默认值、校验、成功/失败响应、拦截器、非 JSON 响应、SSE，以及可选的 OpenAPI 代码生成。

这份 README 只保留主线用法。完整运行时参考见 [docs/RUNTIME_REFERENCE.md](docs/RUNTIME_REFERENCE.md)，OpenAPI 生成器见 [README_CODEGEN.md](README_CODEGEN.md)。

## 安装

```bash
go get github.com/chendefine/ginx
```

要求：

- Go 1.25+
- Gin 1.9+

## 快速开始

```go
package main

import (
	"context"
	"fmt"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

type HelloReq struct {
	Name string `uri:"name" binding:"required"`
}

type HelloRsp struct {
	Message string `json:"message"`
}

func main() {
	r := gin.Default()

	ginx.GET(r, "/hello/:name", func(ctx context.Context, req *HelloReq) (*HelloRsp, error) {
		return &HelloRsp{Message: fmt.Sprintf("hello %s", req.Name)}, nil
	})

	r.Run(":8080")
}
```

请求：

```bash
curl http://127.0.0.1:8080/hello/world
```

默认响应：

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "message": "hello world"
  }
}
```

## 推荐接入方式

中大型项目和测试较多的项目，推荐使用独立 `Engine`，并在注册路由前完成配置：

```go
engine := ginx.New(
	ginx.WithStrictJSONBody(true),
	ginx.WithExposeInternalError(false),
	ginx.WithInternalErrorMessage("internal error"),
)

r := gin.Default()
api := engine.Wrap(r.Group("/api/v1"))

ginx.POST(api, "/users", CreateUser)
```

关键规则：

- `Engine` 配置会在路由注册时生成快照；注册后再修改只影响后续路由。
- 包级 `ginx.GET/POST/...` 和 `ginx.Set*` 使用共享默认 `Engine`，适合 demo 或启动期一次性配置。
- 公网 API 建议开启 `WithStrictJSONBody(true)`，并关闭普通 `error` 的原始信息暴露。
- 需要多个响应/错误策略时，创建多个独立 `Engine`，不要在运行期动态切换全局配置。

## Handler 模型

业务 Handler 使用标准 `context.Context`：

```go
func CreateUser(ctx context.Context, req *CreateUserReq) (*CreateUserRsp, error) {
	return userService.CreateUser(ctx, req)
}
```

`ginx` 会自动完成：

1. 创建并绑定 `Req`
2. 应用 `default` tag
3. 执行 `binding` 校验
4. 调用业务 Handler
5. 写出成功或失败响应

需要访问底层 Gin 能力时，在 HTTP 边界调用：

```go
gc, ok := ginx.GinContext(ctx)
```

`*gin.Context` 仍然只应在当前请求生命周期内使用；service 层建议继续保持标准 `context.Context`。

## 路由注册

常用注册函数：

```go
ginx.GET(router, path, handler, opts...)
ginx.POST(router, path, handler, opts...)
ginx.PUT(router, path, handler, opts...)
ginx.PATCH(router, path, handler, opts...)
ginx.DELETE(router, path, handler, opts...)
ginx.Any(router, path, handler, opts...)
ginx.SSE(router, path, handler, opts...)
```

`router` 可以是 `*gin.Engine`、`*gin.RouterGroup`、`engine.Wrap(...)` 或 `engine.Group(...)` 返回的 `*ginx.Router`。

## 请求绑定

一个 `Req` 可以同时从 path、query、header、cookie、form、multipart、JSON body 绑定字段：

```go
type SearchReq struct {
	UserID int64  `uri:"user_id" binding:"required"`
	Token  string `header:"X-Token" binding:"required"`
	SID    string `cookie:"sid"`
	Page   int    `form:"page" default:"1" binding:"gt=0"`
	Size   int    `form:"size" default:"20" binding:"gt=0,max=100"`
	Word   string `json:"word"`
}
```

常用 tag：

| 来源 | tag |
|---|---|
| 路径参数 | `uri:"id"` |
| query / form / multipart 普通字段 | `form:"name"` |
| 请求头 | `header:"X-Token"` |
| Cookie | `cookie:"sid"` |
| JSON body | `json:"name"` |
| 默认值 | `default:"1"` |
| 校验 | `binding:"required,email,max=100"` |

multipart 文件上传：

```go
type UploadReq struct {
	Name string                `form:"name" binding:"required"`
	File *multipart.FileHeader `form:"file" binding:"required"`
}
```

## 响应与错误

默认成功响应包装为：

```json
{
  "code": 0,
  "msg": "",
  "data": {}
}
```

按路由调整包装：

```go
ginx.GET(r, "/raw", GetUser, ginx.NoDataWrap())
ginx.GET(r, "/wrapped", GetUser, ginx.WrapData())
ginx.POST(r, "/legacy", Login, ginx.AlwaysOK())
```

业务错误使用 `ginx.Error`：

```go
return nil, ginx.Error(1001, "user not found").Status(http.StatusNotFound)
```

普通 `error` 默认会以内部错误返回。为了避免公网服务暴露 `err.Error()`，推荐：

```go
engine := ginx.New(
	ginx.WithExposeInternalError(false),
	ginx.WithInternalErrorMessage("internal error"),
)
```

需要统一业务错误体、校验错误体或成功响应体时，使用：

- `WithErrorHandler(...)`
- `WithValidationErrorHandler(...)`
- `WithSuccessHandler(...)`
- `WithJSONRenderer(...)`

## 拦截器

Gin middleware 适合处理原始 HTTP 传输层逻辑；`ginx.Interceptor` 适合处理已经绑定完成的类型化请求。

```go
engine := ginx.New(
	ginx.WithInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		ginx.SetHeader(ctx, "X-Ginx", "enabled")
		return next()
	}),
)
```

约束：

- Engine 级 interceptor 在外层，Route 级 interceptor 在内层。
- `next()` 在单次请求中只能调用一次。
- 返回值必须是 `next()` 原始结果、`nil`，或当前 handler 声明的响应指针类型。

## 非 JSON 响应与 SSE

handler 返回实现了 `ginx.Response` 的类型时，会跳过 JSON 包装：

```go
return ginx.StringResponse(http.StatusOK, "ok"), nil
return ginx.DataResponse(http.StatusOK, "application/octet-stream", data), nil
return ginx.FileResponse("/tmp/report.pdf", "report.pdf"), nil
return ginx.RedirectResponse(http.StatusFound, "/login"), nil
```

SSE 使用独立注册函数：

```go
ginx.SSE(r, "/events", func(ctx context.Context, req *struct{}, send ginx.Sender) error {
	return send(ginx.Event{
		Event: "message",
		Data:  gin.H{"hello": "world"},
	})
})
```

内置 SSE 只提供基础事件编码、header 和 flush；心跳、断线恢复、广播和连接治理应由上层实现。

## OpenAPI Codegen

如果项目以 OpenAPI 为契约，优先使用 `oapi-ginx` 生成类型、服务接口、路由注册和可选客户端 SDK。

安装 CLI（需要 Go 1.25+）。`go install` 会把二进制装到 `$GOPATH/bin`（未设置 `GOBIN` 时默认为 `~/go/bin`），确认该目录已加入 `PATH` 后即可全局调用：

```bash
go install github.com/chendefine/ginx/cmd/oapi-ginx@latest
```

也可以不安装、直接在项目内用 `go run` 运行，版本随当前模块依赖走，适合 CI 或锁定版本：

```bash
go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml
```

需要从源码构建时，clone 本仓库后执行 `go build -o oapi-ginx ./cmd/oapi-ginx`。

安装后生成示例配置，再生成代码：

```bash
oapi-ginx -init > oapi-ginx.yaml
oapi-ginx -c oapi-ginx.yaml
```

生成后实现 `ServerInterface`，并在启动代码中调用 `RegisterRoutes(r, svc, opts...)`。

完整配置、命令行参数和生成规则见 [README_CODEGEN.md](README_CODEGEN.md) 和 [docs/CODEGEN_REFERENCE.md](docs/CODEGEN_REFERENCE.md)。

## AI Agent 接入

面向 AI Agent 的最短入口：

- [AGENTS.md](AGENTS.md)：仓库级 AI 工作流入口
- [skills/ginx-http-backend/SKILL.md](skills/ginx-http-backend/SKILL.md)：可复制到 Codex skill registry 的精简技能包

建议 Agent 先读 skill，再按任务需要打开本 README、[README_CODEGEN.md](README_CODEGEN.md) 或 `docs/` 参考手册。

## 运行示例与测试

```bash
go run ./examples/basic
go test ./...
```

修改 codegen 模板、OpenAPI fixture 或生成器行为后，还应运行：

```bash
./scripts/test-codegen-e2e.sh
```

## 当前边界

`ginx` 聚焦于 Handler 适配、请求绑定校验、响应协议包装和可插拔扩展点。它不内置认证鉴权框架、DI 容器、ORM、tracing/metrics SDK 或完整 OpenAPI 文档站点；这些能力建议通过 Gin middleware、`Interceptor`、`WithOnRegister` 或上层工程模板组合实现。
