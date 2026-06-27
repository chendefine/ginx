# oapi-ginx

`oapi-ginx` 是 `github.com/chendefine/ginx` 的 OpenAPI 3.0 代码生成器。它从 OpenAPI spec 生成：

- Go 请求/响应类型
- `ServerInterface`
- `RegisterRoutes`
- 可选 resty 客户端 SDK
- 可选压缩内嵌 spec

这份文档只保留主线工作流。完整类型映射、验证规则、响应分类和长示例见 [docs/CODEGEN_REFERENCE.md](docs/CODEGEN_REFERENCE.md)。

## 安装

```bash
go install github.com/chendefine/ginx/cmd/oapi-ginx@latest
```

也可以在项目内用 `go run` 固定到当前模块依赖：

```bash
go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml
```

## 快速开始

生成示例配置：

```bash
oapi-ginx -init > oapi-ginx.yaml
```

最小配置：

```yaml
package: api
spec: ./openapi.yaml
output:
  types: types.gen.go
  server: server.gen.go
generate_directive: "go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml"
output_options:
  generate_server: true
```

生成代码：

```bash
oapi-ginx -c oapi-ginx.yaml
# 或
go generate ./...
```

## 命令行

```bash
oapi-ginx spec.yaml
oapi-ginx -o api.gen.go spec.yaml
oapi-ginx -p api -o api.gen.go spec.yaml
oapi-ginx -c oapi-ginx.yaml
oapi-ginx -init
```

| 参数 | 简写 | 说明 |
|---|---|---|
| `--output` | `-o` | 输出文件路径，默认 stdout |
| `--package` | `-p` | Go 包名，默认从输出目录推导 |
| `--config` | `-c` | YAML 配置文件 |
| `--init` | - | 输出示例配置 |

## 配置主线

推荐使用多文件输出，方便 review 和按需生成：

```yaml
package: api
spec: ./openapi.yaml
output:
  types: types.gen.go
  server: server.gen.go
  client: client.gen.go
  spec: spec.gen.go
server_name: pet_store
include_tags: [pets]
exclude_tags: [internal]
generate_directive: "go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml"
output_options:
  generate_server: true
  generate_client: true
  skip_fmt: false
  unwrap_envelope: true  # 自动解包 spec 中误写的 {code,msg,data} 响应封装（默认 true）
```

输出字段：

| 字段 | 说明 |
|---|---|
| `types` | schema、请求、响应、枚举等类型 |
| `server` | `ServerInterface` 和 `RegisterRoutes` |
| `client` | HTTP 客户端 SDK，指定路径即启用 |
| `spec` | 压缩内嵌 OpenAPI spec，提供 `GetSwaggerSpec()` |

`server_name` 用于同一个 package 内生成多份 API，避免命名冲突。例如 `pet_store` 会生成 `PetStoreServerInterface`、`RegisterPetStoreRoutes`、`NewPetStoreClient`。

兼容说明：顶层 `generate_server` 仍可读取，但已废弃；新配置请使用 `output_options.generate_server`。

## 从 Spec 到服务

OpenAPI operation 会生成 `{OperationName}Req`、`{OperationName}Rsp`、接口方法和路由注册函数：

```go
type GetPetReq struct {
	PetID int64 `uri:"pet_id" binding:"required"`
}

type GetPetRsp = Pet

type ServerInterface interface {
	// GET /pets/:pet_id
	GetPet(ctx context.Context, req *GetPetReq) (*GetPetRsp, error)
}

func RegisterRoutes(r gin.IRoutes, s ServerInterface, opts ...ginx.RouteOption) {
	ginx.GET(r, "/pets/:pet_id", s.GetPet, opts...)
}
```

业务侧只实现接口：

```go
type PetService struct{}

func (s *PetService) GetPet(ctx context.Context, req *api.GetPetReq) (*api.GetPetRsp, error) {
	pet, err := loadPet(ctx, req.PetID)
	if err != nil {
		return nil, ginx.Error(1002, "pet not found").Status(http.StatusNotFound)
	}
	return pet, nil
}
```

启动时注册：

```go
r := gin.Default()
engine := ginx.New(
	ginx.WithStrictJSONBody(true),
	ginx.WithExposeInternalError(false),
	ginx.WithInternalErrorMessage("internal error"),
)

api.RegisterRoutes(engine.Wrap(r.Group("/api/v1")), &PetService{})
r.Run(":8080")
```

## 请求生成规则

每个 operation 生成一个 `{OperationName}Req`，合并 path/query/header/cookie/body：

| OpenAPI 参数位置 | Go struct tag |
|---|---|
| `in: path` | `uri:"name"`，始终 required |
| `in: query` | `form:"name"` |
| `in: header` | `header:"name"` |
| `in: cookie` | `cookie:"name"` |
| JSON body object | `json:"name"` |
| form-urlencoded / multipart | `form:"name"` |

常见请求体行为：

- JSON `$ref` 会以内嵌类型生成到 `Req`。
- inline JSON object 会平铺为 `Req` 字段。
- 非 object JSON body 会生成 `Body` 字段。
- multipart 文件字段生成 `*multipart.FileHeader` 或 `[]*multipart.FileHeader`。
- OpenAPI `default` 会生成 `default` tag。
- OpenAPI schema 约束会尽量转换为 Gin validator 的 `binding` tag。

## 响应生成规则

生成器默认选择成功响应：

- 优先选择最小的 `2xx` 响应。
- 多个 `2xx` JSON schema 不一致时会失败，避免误选。
- 可用 `x-ginx-primary-response: true` 明确主成功响应。
- `204` 或无 content 的 `2xx` 生成 `struct{}`。
- 只有 `3xx` 且无 `2xx` 时生成 `ginx.RedirectRsp`。

常见响应映射：

| 响应 | 生成类型 |
|---|---|
| JSON object / array / `$ref` | `{OperationName}Rsp` 或类型别名 |
| `application/octet-stream` | `ginx.FileRsp` |
| `text/plain` | `ginx.StringRsp` |
| `text/event-stream` | SSE handler |
| 仅重定向响应 | `ginx.RedirectRsp` |

**响应封装自动解包**：ginx 运行时会自动把成功响应包装成 `{"code":0,"msg":"","data":{...}}`，所以 spec 的 response 只需描述 `data` 里的业务数据。若 spec 误把 response 写成整层 `{code,msg,data}`，codegen 默认（`output_options.unwrap_envelope: true`）会识别并只取 `data` 子 schema 生成 `Rsp`，避免运行时双壳封装。判定为严格匹配：对象且**恰好**三字段 `code`(integer)/`msg`(string)/`data`。若业务响应本身就是这三字段结构，设 `unwrap_envelope: false` 关闭。详见 [docs/CODEGEN_REFERENCE.md](docs/CODEGEN_REFERENCE.md#响应封装自动解包unwrap_envelope)。

当 content type 无法表达意图时，使用 `x-ginx-response`：

```yaml
responses:
  "200":
    x-ginx-response: data
    content:
      application/octet-stream:
        schema:
          type: string
          format: binary
```

支持值：`file`、`string`、`data`、`redirect`。

## SSE

满足任一条件会生成 SSE operation：

- operation 设置 `x-ginx-sse: true`
- 成功响应 content type 为 `text/event-stream`

生成的服务端签名：

```go
ListEvents(ctx context.Context, req *ListEventsReq, send ginx.Sender) error
```

注册时使用 `ginx.SSE`。如果生成 client，SSE 客户端方法返回 `*ginx.SSEStream`，调用方使用 `Recv()` 拉取事件并在结束时调用 `Close()`。

## JSON Lines / NDJSON（OpenAPI 3.2）

满足任一条件会生成 JSON Lines 流式 operation：

- operation 设置 `x-ginx-jsonl: true`
- 成功响应 content type 为 `application/jsonl` 或 `application/x-ndjson`

```go
TailLogs(ctx context.Context, req *TailLogsReq, send ginx.JSONLinesSender) error
```

注册时使用 `ginx.JSONLines`；每个 item 经 `send` 写为紧凑 JSON + `\n` 并 flush。客户端方法返回 `*ginx.JSONLinesStream`，`Recv()` 逐行返回 `json.RawMessage`（item 类型为 `any`，因 3.2 `itemSchema` 被解析库丢弃）。详见 `docs/CODEGEN_REFERENCE.md`。

> 3.1 新增：`const`→`oneof`、`prefixItems` 元组→`[]any`、可空 type 数组、`webhooks` 入站处理器、数值 exclusive 边界。3.2 的 `in: querystring` 归一化为普通 query 参数；`itemSchema`/`QUERY`/`additionalOperations` 等结构化字段受 kin-openapi v0.140.0 限制会在校验阶段清晰报错（库限制，待上游支持）。

## 客户端 SDK

启用方式：

```yaml
output:
  types: types.gen.go
  server: server.gen.go
  client: client.gen.go
output_options:
  generate_client: true
```

使用方式：

```go
client := api.NewClient("http://localhost:8080",
	func(c *resty.Client) {
		c.SetTimeout(10 * time.Second)
		c.SetAuthToken("token")
	},
)

rsp, err := client.GetPet(context.Background(), &api.GetPetReq{PetID: 1})
```

客户端复用服务端 `Req` 类型：

- `uri` 字段设置 path param。
- query `form` 字段设置 query param。
- form-urlencoded `form` 字段设置 form data。
- `header` 字段设置 header。
- `cookie` 字段设置 cookie。
- `json` 字段作为 JSON body。
- 指针可选字段为 `nil` 时跳过。

当前限制：服务端 multipart 文件上传类型已支持，但客户端 SDK 暂不生成文件上传方法。启用 client 且 spec 含文件上传字段时，生成器会直接失败并指出 operation 名称。

## 常用扩展

| 扩展 | 位置 | 作用 |
|---|---|---|
| `x-ginx-sse: true` | operation | 强制生成 SSE handler/client |
| `x-ginx-jsonl: true` | operation | 强制生成 JSON Lines 流式 handler/client |
| `x-ginx-response: file\|string\|data\|redirect` | operation 或 response | 覆盖非 JSON 响应分类 |
| `x-ginx-primary-response: true` | response | 多个成功响应中选择主响应 |
| `x-binding: "..."` | schema/property | 追加自定义 validator 规则 |

## AI / Code Agent 工作流

让 AI Agent 接入 codegen 时，推荐顺序：

1. 先读 [AGENTS.md](AGENTS.md) 和 [skills/ginx-http-backend/SKILL.md](skills/ginx-http-backend/SKILL.md)。
2. 读取 `oapi-ginx.yaml` 和 OpenAPI spec，确认 `package`、`output`、`server_name`、tag 过滤和 client 开关。
3. 运行 `go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml` 或项目约定的 `go generate ./...`。
4. 实现生成的 `ServerInterface`，不要手改 `*.gen.go`。
5. 在 Gin 启动代码中调用 `RegisterRoutes(r, svc, opts...)`。
6. 运行 `go test ./... -count=1`；修改生成器模板或 fixture 时再跑 `./scripts/test-codegen-e2e.sh`。

## 当前边界

`oapi-ginx` 只生成 Go 类型、ginx 服务端接口/路由、可选 resty 客户端和可选 spec embed。它不负责 Swagger UI 服务、强类型 union/oneOf、鉴权、DI、ORM、数据库访问、tracing/metrics 初始化或 multipart 文件上传客户端。
