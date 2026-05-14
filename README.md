# ginx

基于 Go 泛型的 [Gin](https://github.com/gin-gonic/gin) 类型安全 HTTP Handler 包装库。

它把 HTTP Handler 收敛成统一的 RPC 风格签名：

```go
func(ctx context.Context, req *Req) (*Rsp, error)
```

`ginx` 负责：

- 多源请求绑定（header / uri / query / form / multipart / json）
- 默认值填充与参数校验
- 统一成功 / 失败响应
- 自定义成功包装、错误处理、校验错误处理、JSON 渲染
- 路由级与 Engine 级拦截器
- 非 JSON 响应（文本、原始字节、文件、重定向）
- SSE
- Engine 级配置隔离

---

## 安装

```bash
go get github.com/chendefine/ginx
```

要求：

- Go 1.25+
- Gin 1.9+

---

## 1. 快速开始

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

---

## 2. Handler 签名

所有业务 Handler 都使用统一签名：

```go
func(ctx context.Context, req *Req) (*Rsp, error)
```

含义：

- `ctx`：标准 `context.Context`，原始入参可通过 `ginx.GinContext(ctx)` 取回底层 `*gin.Context`
- `req`：自动绑定并校验后的请求结构体
- `rsp`：成功响应对象
- `error`：失败时返回错误

`GinContext(ctx)` 对 ginx 传入 handler 的原始 `ctx` 生效；基于该 `ctx` 再通过 `context.WithValue`、`context.WithCancel`、`context.WithTimeout`、`context.WithDeadline` 派生出的标准 context，通常也仍可取回 `*gin.Context`。但如果业务代码改用了无关的新 context，则无法回取。

`ctx` 本身不会因为 ginx 内部复用而在请求结束后失效；不过 Gin 的 `*gin.Context` 和响应写入能力仍然只应在请求生命周期内使用。涉及 Gin 专属能力时，仍建议在 handler 边界尽早取出并向下游传递所需值，而不是在深层 service 中回取。

如果你的 service 层已经采用 `context.Context + *Req -> (*Rsp, error)` 形式，接入成本会很低。

---

## 3. 路由注册

支持这些注册函数：

```go
ginx.GET(router, path, handler, opts...)
ginx.POST(router, path, handler, opts...)
ginx.PUT(router, path, handler, opts...)
ginx.PATCH(router, path, handler, opts...)
ginx.DELETE(router, path, handler, opts...)
ginx.HEAD(router, path, handler, opts...)
ginx.OPTIONS(router, path, handler, opts...)
ginx.Any(router, path, handler, opts...)
ginx.Handle(router, []string{...}, path, handler, opts...)
ginx.SSE(router, path, handler, opts...)
```

示例：

```go
ginx.Any(r, "/ping", func(ctx context.Context, req *struct{}) (*struct {
	Message string `json:"message"`
}, error) {
	return &struct {
		Message string `json:"message"`
	}{Message: "pong"}, nil
})
```

`router` 可以是：

- `*gin.Engine`
- `*gin.RouterGroup`
- `engine.Wrap(...)` 返回的 `*ginx.Router`
- `engine.Group(...)` 返回的 `*ginx.Router`

---

## 4. 请求绑定

### 4.1 支持的来源

同一个 `Req` 可以同时从多个来源绑定：

- `header:"X-Token"`
- `uri:"id"`
- `form:"page"`（query / x-www-form-urlencoded / multipart）
- `json:"name"`

示例：

```go
type SearchReq struct {
	UserID int64  `uri:"user_id" binding:"required"`
	Token  string `header:"X-Token" binding:"required"`
	Page   int    `form:"page" binding:"gt=0"`
	Size   int    `form:"size" binding:"gt=0,max=100"`
	Word   string `json:"word"`
}

ginx.POST(r, "/users/:user_id/search", func(ctx context.Context, req *SearchReq) (*SearchRsp, error) {
	return doSearch(ctx, req), nil
})
```

绑定顺序是：

1. header
2. uri
3. query（带 `form` tag 的字段）
4. body
   - `application/json` / `application/*+json`
   - `application/x-www-form-urlencoded`
   - `multipart/form-data`

校验（`binding` tag）统一在所有绑定完成后执行，确保多源字段都能被校验到。

### 4.2 JSON Content-Type

ginx 支持：

- `application/json`
- `application/json; charset=utf-8`
- `application/*+json`，例如：
  - `application/merge-patch+json`
  - `application/problem+json`

### 4.3 multipart 上传

只要在结构体中声明 `*multipart.FileHeader` 即可：

```go
type UploadReq struct {
	Name string                `form:"name" binding:"required"`
	File *multipart.FileHeader `form:"file" binding:"required"`
}

ginx.POST(r, "/upload", func(ctx context.Context, req *UploadReq) (*UploadRsp, error) {
	return &UploadRsp{Filename: req.File.Filename}, nil
})
```

### 4.4 默认值

如果请求结构体使用了 `default` tag，ginx 会在绑定前应用默认值：

```go
type ListReq struct {
	Page int `form:"page" default:"1" binding:"required"`
	Size int `form:"size" default:"20"`
}
```

---

## 5. 参数校验

ginx 使用 Gin 的 binding + validator 机制。

```go
type CreateUserReq struct {
	Name  string `json:"name" binding:"required,max=100"`
	Email string `json:"email" binding:"required,email"`
	Age   int    `json:"age" binding:"gt=0,lt=150"`
}
```

### 5.1 默认校验错误响应

默认会返回业务码 `1`、HTTP 400，并对 validator 错误做脱敏：

```json
{
  "code": 1,
  "msg": "name is required"
}
```

说明：

- 默认脱敏文案追求"可读且稳定"，不是完整的字段映射或 i18n 方案
- 字段名优先使用请求 tag 名（顺序：`json` > `form` > `uri` > `header`），无 tag 时回退到 Go 结构体字段名
- 已内置覆盖绝大多数常用 validator tag 的可读文案
- 未覆盖的规则回退为 `xxx is invalid`
- 多个字段校验失败时，错误信息以 `; ` 拼接
- 对复杂 API、嵌套结构、多语言场景，建议使用 `WithValidationErrorHandler(...)` 完全接管

内置覆盖的校验类别（非完整列举）：

| 类别 | 示例文案 |
|------|---------|
| 必填 | `xxx is required` |
| 比较 | `xxx must be greater than n` / `must be less than n` / `must be at least n` / `must be at most n` |
| 枚举 | `xxx must be one of ...` |
| 字符串格式 | `xxx must contain only letters` / `must be lowercase` / `must start with ...` 等 |
| 格式校验 | `xxx must be a valid email` / `valid URL` / `valid UUID` / `valid IP address` 等 |
| 网络地址 | `IPv4` / `IPv6` / `CIDR` / `MAC` / `hostname` / `host:port` |
| 编码 | `base64` / `JSON` / `MD5` / `SHA256` |
| 其它 | `xxx is invalid`（兜底） |

### 5.2 自定义校验错误处理

可以用 `WithValidationErrorHandler` 完全接管：

```go
engine := ginx.New(
	ginx.WithValidationErrorHandler(func(ctx context.Context, err error) (int, any) {
		return http.StatusBadRequest, gin.H{
			"code": 4001,
			"msg":  "invalid request",
		}
	}),
)
```

如果返回的 `httpStatus <= 0`，会回退到默认处理。

---

## 6. 成功响应

### 6.1 默认包装

默认成功响应（`dataWrap=true`）为：

```json
{
  "code": 0,
  "msg": "",
  "data": ...
}
```

即使 `rsp == nil`，也仍然会返回：

```json
{
  "code": 0,
  "msg": "",
  "data": null
}
```

`ginx.EmptyHandler` 是一个内置空 Handler：

```go
ginx.GET(r, "/healthz", ginx.EmptyHandler)
```

### 6.2 关闭包装

使用 `NoDataWrap()`：

```go
ginx.GET(r, "/raw", GetUser, ginx.NoDataWrap())
```

响应会直接输出业务结构体：

```json
{
  "id": 1,
  "name": "alice"
}
```

### 6.3 自定义成功响应包装

使用 `WithSuccessHandler`：

```go
engine := ginx.New(
	ginx.WithSuccessHandler(func(ctx context.Context, data any) (int, any) {
		return http.StatusOK, gin.H{
			"code":    0,
			"msg":     "ok",
			"payload": data,
		}
	}),
)
```

说明：

- 仅在 `dataWrap=true` 时生效
- 如果返回的 `httpStatus <= 0`，最终会按 200 处理

---

## 7. 错误处理

### 7.1 结构化错误 `ErrWrap`

```go
return nil, ginx.Error(1001, "user not found")
```

默认输出：

```json
{
  "code": 1001,
  "msg": "user not found"
}
```

默认 HTTP 状态码是 500。

### 7.2 指定 HTTP 状态码

```go
return nil, ginx.Error(1001, "user not found").Status(http.StatusNotFound)
```

仅当状态码在 `101~599` 范围内时才会生效。

### 7.3 格式化错误消息

```go
return nil, ginx.Error(1002, "user %s not found").Format(req.Name)
```

### 7.4 普通 `error`

如果返回普通 `error`：

```go
return nil, errors.New("boom")
```

默认输出：

```json
{
  "code": 2,
  "msg": "boom"
}
```

默认 HTTP 状态码为 500。

注意：普通 `error` 的默认响应会直接包含 `err.Error()`。公网 API 或可能携带内部细节的服务，建议使用 `WithErrorHandler(...)` 统一做脱敏和错误码映射。

### 7.5 Sentinel 错误比较

`*ErrWrap` 实现了 `errors.Is`，以 `Code` 字段作为相等判断依据，便于定义 sentinel 错误并在调用链中比较：

```go
var ErrNotFound = ginx.Error(1001, "user %s not found")

// 某处返回
return nil, ErrNotFound.Format(req.Name)

// 某处判断
if errors.Is(err, ErrNotFound) {
    // code == 1001
}
```

### 7.6 自定义错误处理

```go
engine := ginx.New(
	ginx.WithErrorHandler(func(ctx context.Context, err error) (int, any) {
		return http.StatusConflict, gin.H{
			"code": 2001,
			"msg":  err.Error(),
		}
	}),
)
```

说明：

- `*ErrWrap` 优先走内置处理，不经过 `WithErrorHandler`
- 只有普通 `error` 才会进入 `WithErrorHandler`
- 如果返回的 `httpStatus <= 0`，会回退到默认处理

---

## 8. Route 选项

### 8.1 `WrapData()`

强制当前路由开启 `{code,msg,data}` 包装，即使 Engine 默认关闭了包装。

### 8.2 `NoDataWrap()`

强制当前路由关闭成功响应包装。

### 8.3 `AlwaysOK()`

无论校验失败、业务失败还是普通错误，HTTP 状态总是 200：

```go
ginx.POST(r, "/mobile/login", Login, ginx.AlwaysOK())
```

适用于：

- 历史移动端协议
- 某些网关 / 客户端只能解析 200 的场景

错误信息仍然会在 body 中返回。

### 8.4 `RouteInterceptor(...)`

给单个路由增加拦截器：

```go
ginx.GET(r, "/users/:id", GetUser,
	ginx.RouteInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		start := time.Now()
		rsp, err := next()
		_ = start
		return rsp, err
	}),
)
```

说明：

- interceptor 的 `req` / `rsp` 是类型擦除的 `any`
- 返回值必须与当前 handler 声明的响应类型一致，也就是返回 `*Rsp` 或 `nil`
- 如果返回了不匹配的类型，ginx 会以 panic 抛出，便于在开发 / 测试阶段尽早暴露编程错误
- 更适合日志、审计、鉴权、包裹 `next()` 前后的通用逻辑，而不是改写成任意响应类型

---

## 9. Engine 级配置

推荐在实际项目中优先使用独立 `Engine`，避免全局默认配置互相污染。

推荐：

- 中大型项目、测试较多的项目，优先使用 `ginx.New(...)` 创建独立 `Engine`
- `engine.Wrap(...)` / `engine.Group(...)` 是更推荐的注册入口
- 包级 `ginx.GET/POST/...` + `Set*` 更适合快速接入或在启动阶段统一配置

```go
engine := ginx.New(
	ginx.WithInvalidArgCode(10001),
	ginx.WithInternalErrorCode(10002),
)

r := gin.Default()
api := engine.Wrap(r.Group("/api/v1"))

ginx.GET(api, "/users/:id", GetUser)
```

也可以直接基于 `*gin.Engine` 或 `*gin.RouterGroup` 创建分组：

```go
api := engine.Group(r, "/api/v1")
```

### 9.1 可用的 Engine 选项

- `WithDataWrap(bool)`：控制成功响应是否包装，默认 `true`
- `WithInvalidArgCode(int)`：参数校验失败的业务 code，默认 `1`
- `WithInternalErrorCode(int)`：普通 error 的业务 code，默认 `2`
- `WithErrorHandler(...)`
- `WithValidationErrorHandler(...)`
- `WithSuccessHandler(...)`
- `WithJSONRenderer(...)`
- `WithInterceptor(...)`
- `WithOnRegister(...)`
- `WithJsonDecoderUseNumber(bool)`

### 9.2 包级默认 Engine

如果你不想手动创建 `Engine`，也可以直接使用包级默认实例：

```go
ginx.SetDataWrap(false)
ginx.SetInvalidArgumentCode(4001)
ginx.SetInternalServerErrorCode(5001)
ginx.SetJsonDecoderUseNumber(true)
```

这些配置会影响直接调用 `ginx.GET/POST/...` 时使用的默认 Engine。

注意：

- `Set*` 修改的是进程内共享默认 Engine
- 推荐只在启动期集中设置一次，不建议在运行期或不同测试用例中交叉修改
- 如果希望不同模块拥有不同配置，请显式创建多个 `Engine`

### 9.3 `UseNumber`

开启 `WithJsonDecoderUseNumber(true)` 或 `SetJsonDecoderUseNumber(true)` 后，JSON body 解码会启用 `json.Decoder.UseNumber()`。

适合需要保留数值精度、并在请求结构体中使用 `any` / `interface{}` 接收数字的场景。

说明：

- `ginx` 在 JSON body 路径中使用自己的 `json.Decoder`，而不是依赖 Gin 的全局 `binding.EnableDecoderUseNumber`
- 这样做是为了让 `UseNumber` 成为 Engine 级配置，避免不同模块或测试之间互相污染
- 这是有意识的实例级隔离取舍，而不是简单沿用框架全局默认值

---

## 10. 自定义 JSON 渲染

有些项目希望统一追加 header、打日志，或者替换 JSON 输出方式，可以使用 `WithJSONRenderer`：

```go
engine := ginx.New(
	ginx.WithJSONRenderer(func(c *gin.Context, status int, body any) {
		c.Header("X-Renderer", "custom")
		c.JSON(status, body)
	}),
)
```

这个 renderer 会统一作用于：

- 成功 JSON 响应
- 校验错误响应
- 普通错误响应
- `ErrWrap` 错误响应
- `NoDataWrap()` 路由返回的 JSON

非 JSON 响应（如文件下载、重定向、文本、原始字节、SSE）不会走它。

---

## 11. Interceptor

Interceptor 是 ginx 层的中间件，能拿到已经绑定完成的 `req`：

```go
engine := ginx.New(
	ginx.WithInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		ginx.SetHeader(ctx, "X-Ginx", "enabled")
		return next()
	}),
)
```

执行顺序：

- Engine 级 interceptor 在外层
- Route 级 interceptor 在内层
- 最里层才是业务 handler

约束：

- interceptor 应返回 `next()` 的原始结果，或与 handler 声明的 `*Rsp` 类型一致的值（或 `nil`）
- 如果返回值类型与 handler 声明的 `*Rsp` 不匹配，ginx 会以 panic 抛出，便于在开发 / 测试阶段尽早暴露编程错误

适合：

- 打点
- 审计
- 统一 header 注入
- 基于已绑定请求结构体做鉴权或日志

---

## 12. 非 JSON 响应

如果 handler 返回实现了 `ginx.Response` 的类型，会跳过 JSON 包装，直接调用 `WriteTo()`。

内置支持：

### 12.1 文本响应

```go
return ginx.StringResponse(http.StatusOK, "hello %s", req.Name), nil
```

### 12.2 原始字节响应

```go
return ginx.DataResponse(http.StatusOK, "application/octet-stream", data), nil
```

### 12.3 文件下载

```go
return ginx.FileResponse("/tmp/report.pdf", "report.pdf"), nil
```

### 12.4 重定向

```go
return ginx.RedirectResponse(http.StatusFound, "/login"), nil
```

如果 `WriteTo()` 返回错误，ginx 会把错误记录进 `gin.Context.Errors`，方便上层 Gin middleware 统一处理。

---

## 13. SSE

ginx 内置的是基础、轻量的 Server-Sent Events 支持：

```go
ginx.SSE(r, "/events", func(ctx context.Context, req *struct{}, send ginx.Sender) error {
	if err := send(ginx.Event{
		Event: "message",
		Data:  gin.H{"hello": "world"},
	}); err != nil {
		return err
	}
	return nil
})
```

返回示例：

```text
event:message
data:{"hello":"world"}
```

`Event` 结构体字段：

```go
type Event struct {
	ID    string
	Event string
	Data  any
	Retry uint
}
```

提供：

- 自动设置常见 SSE header（`Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`）
- 提供事件编码与 flush
- 默认不走 JSON dataWrap

不覆盖：

- 心跳保活策略
- 断线恢复或 replay
- 连接治理与广播抽象

如果你的场景需要完整流式基础设施，建议在此基础上自行扩展。

---

## 14. Context 使用方式

业务 handler 接收的是标准 `context.Context`，ginx 提供一组辅助函数访问底层 `*gin.Context` 能力。

### 14.1 获取底层 `*gin.Context`

```go
gc, ok := ginx.GinContext(ctx)
if !ok {
	return nil, errors.New("missing gin context")
}
```

`GinContext(ctx)` 对 ginx 传入 handler 的原始 `ctx` 生效；基于该 `ctx` 再派生出的标准 context，通常也仍可回取到底层 `*gin.Context`。但如果你改用了无关的新 context，则无法回取。

`ctx` 可以安全地按标准 `context.Context` 语义继续派生和传递；但 `*gin.Context` 本身仍然代表当前 HTTP 请求，不建议在请求结束后异步访问或写响应。

### 14.2 常用辅助函数

```go
ginx.Get(ctx, "uid")
ginx.Set(ctx, "uid", int64(1))
ginx.MustGet(ctx, "uid")
ginx.GetHeader(ctx, "X-Token")
ginx.SetHeader(ctx, "X-Trace-ID", "abc")
ginx.AddHeader(ctx, "Set-Cookie", "a=1")
ginx.ClientIP(ctx)
ginx.Request(ctx)
ginx.Cookie(ctx, "sid")
ginx.SetCookie(ctx, "sid", "xxx", 3600, "/", "", false, true)
```

### 14.3 泛型取值

```go
uid, ok := ginx.GetValue[int64](ctx, "uid")
```

---

## 15. OnRegister Hook

如果你想在注册路由时收集元信息（例如未来生成 OpenAPI），可以用 `WithOnRegister`：

```go
engine := ginx.New(
	ginx.WithOnRegister(func(info ginx.RegisterInfo) {
		fmt.Println(info.Method, info.Path, info.ReqType, info.RspType)
	}),
)
```

在每次 `GET/POST/...` 注册时都会回调。

---

## 16. 典型接入方式

推荐实践：

- 业务层优先使用标准 `context.Context`
- 需要 Gin 专属能力时，在 handler 边界尽早取出
- 中大型项目优先使用独立 `Engine`
- 复杂错误映射与国际化场景通过自定义 handler 接管
- SSE 应视为基础能力，而不是完整流式基础设施

推荐把 HTTP 层写成轻量适配层：

```go
func HTTPCreateUser(ctx context.Context, req *CreateUserReq) (*CreateUserRsp, error) {
	return userService.CreateUser(ctx, req)
}
```

这样：

- HTTP handler 层只负责协议适配
- service 层保持 RPC 风格
- ginx 负责绑定、校验和响应输出

---

## 17. demo

仓库中的 `examples/basic/main.go` 展示了：

- Engine 级配置
- 普通 JSON 路由
- `NoDataWrap()`
- 文本响应
- 重定向
- multipart 上传
- SSE
- 自定义成功 / 错误处理

运行：

```bash
go run ./examples/basic
```

服务默认监听 `:8081`。

---

## 18. API 速查

### 注册函数

- `GET`
- `POST`
- `PUT`
- `PATCH`
- `DELETE`
- `HEAD`
- `OPTIONS`
- `Any`
- `Handle`
- `SSE`

### 核心类型

- `HandlerFunc[Req, Rsp]` — RPC 风格 handler 签名
- `SSEHandler[Req]` — SSE handler 签名
- `Sender` — SSE 事件推送函数
- `Event` — SSE 事件结构体
- `Response` — 非 JSON 响应接口
- `Interceptor` — 拦截器签名
- `RegisterInfo` — 路由注册元信息
- `RegisterHook` — 路由注册回调签名
- `ErrorHandler` — 自定义错误处理签名
- `ValidationErrorHandler` — 自定义校验错误处理签名
- `SuccessHandler` — 自定义成功响应处理签名
- `JSONRenderer` — 自定义 JSON 渲染签名

### EngineOption

- `WithDataWrap`
- `WithInvalidArgCode`
- `WithInternalErrorCode`
- `WithErrorHandler`
- `WithValidationErrorHandler`
- `WithSuccessHandler`
- `WithJSONRenderer`
- `WithInterceptor`
- `WithOnRegister`
- `WithJsonDecoderUseNumber`

### RouteOption

- `WrapData()`
- `NoDataWrap()`
- `AlwaysOK()`
- `RouteInterceptor(...)`

### Response helper

- `StringResponse`
- `DataResponse`
- `FileResponse`
- `RedirectResponse`

### Error helper

- `Error(code, msg)`
- `(*ErrWrap).Status(code)`
- `(*ErrWrap).Format(args...)`
- `(*ErrWrap).Is(target)` — 支持 `errors.Is` 按 Code 比较

### Context helper

- `GinContext`
- `Get`
- `Set`
- `MustGet`
- `GetHeader`
- `SetHeader`
- `AddHeader`
- `ClientIP`
- `Request`
- `Cookie`
- `SetCookie`
- `GetValue[T]`

### 类型别名

- `AnyMap` — `map[string]any` 的便捷别名

### Engine

- `New`
- `(*Engine).Wrap`
- `(*Engine).Group`
- `Default`
- `Configure`

### 其它

- `EmptyHandler`
- `SetDataWrap`
- `SetInvalidArgumentCode`
- `SetInternalServerErrorCode`
- `SetJsonDecoderUseNumber`

---

## 19. 当前设计边界

ginx 当前聚焦于：

- Handler 适配
- 请求绑定与校验
- 响应协议包装
- 可插拔扩展点

它不负责：

- 路由文档自动生成
- 依赖注入容器
- 认证鉴权框架
- ORM / 数据库抽象
- tracing / metrics SDK 集成

这些能力可以通过 `Interceptor`、`WithOnRegister`、Gin middleware 或你自己的上层框架组合实现。
