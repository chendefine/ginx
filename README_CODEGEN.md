# oapi-ginx 使用说明

oapi-ginx 是一个从 OpenAPI 3.0 规范文件生成 ginx 类型安全代码的命令行工具。它可以自动生成请求/响应结构体、服务接口和路由注册代码。

## 安装

```bash
go install github.com/chendefine/ginx/cmd/oapi-ginx@latest
```

## 快速开始

```bash
# 从 OpenAPI spec 生成代码到 stdout
oapi-ginx spec.yaml

# 生成到指定文件
oapi-ginx -o api.gen.go spec.yaml

# 使用配置文件
oapi-ginx -c oapi-ginx.yaml

# 生成示例配置文件
oapi-ginx -init > oapi-ginx.yaml
```

## 命令行参数

| 参数 | 简写 | 说明 |
|------|------|------|
| `--output` | `-o` | 输出文件路径（默认输出到 stdout） |
| `--package` | `-p` | Go 包名（默认从输出目录名推导，兜底为 `api`） |
| `--config` | `-c` | 配置文件路径（YAML 格式） |
| `--init` | - | 输出示例配置到 stdout |

## 配置文件

配置文件为 YAML 格式，支持以下字段：

```yaml
# 生成代码的 Go 包名
package: api

# OpenAPI 规范文件路径
spec: ./openapi.yaml

# 输出配置（两种模式）
# 模式一：单文件输出
output: api.gen.go

# 模式二：多文件输出
output:
  types: types.gen.go      # 类型定义文件
  server: server.gen.go    # 服务接口和路由注册文件
  client: client.gen.go    # HTTP 客户端 SDK 文件（可选）
  spec: spec.gen.go        # 内嵌 spec 文件（可选）

# go:generate 指令（添加到生成文件头部）
generate_directive: "go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml"

# 服务接口名前缀（用于同一 package 下生成多个 API）
# 例如 server_name: pet_store → PetStoreServerInterface / RegisterPetStoreRoutes
# server_name: ""

# 按 OpenAPI tag 过滤 operation
include_tags: [users, pets]    # 只包含这些 tag 的 operation
exclude_tags: [internal]       # 排除这些 tag 的 operation

# 自定义类型映射（OpenAPI Go 类型 → 替换类型）
type_mapping:
  time.Time: string
  int64: MyInt64

# 扩展类型映射，可显式指定 import path
type_mapping_ext:
  time.Time:
    type: civil.DateTime
    import: cloud.google.com/go/civil

# 输出选项
output_options:
  skip_fmt: false           # 跳过 goimports 格式化
  generate_server: true     # 是否生成 ServerInterface 和 RegisterRoutes
  generate_client: true     # 是否生成 HTTP 客户端 SDK
```

兼容说明：顶层 `generate_server` 仍可读取，但已废弃；新配置请使用 `output_options.generate_server`。如果两者同时出现，`output_options.generate_server` 优先。

### 输出模式

**单文件模式**：所有类型、接口、路由注册代码生成到一个文件中。

**多文件模式**：将生成代码拆分为独立文件：
- `types` — 所有结构体、枚举、类型别名
- `server` — `ServerInterface` 接口和 `RegisterRoutes` 函数
- `client` — HTTP 客户端 SDK（基于 resty.dev/v3）
- `spec` — 内嵌压缩后的 OpenAPI spec，提供 `GetSwaggerSpec()` 函数

## OpenAPI 到 Go 的类型映射

### 基础类型

| OpenAPI type | format | Go type |
|---|---|---|
| integer | - | `int` |
| integer | int32 | `int32` |
| integer | int64 | `int64` |
| number | - | `float64` |
| number | float | `float32` |
| number | double | `float64` |
| string | - | `string` |
| string | date-time | `time.Time` |
| string | date | `string` |
| string | byte | `[]byte` |
| string | binary | `[]byte` |
| string | uuid/uri/email/hostname/ipv4/ipv6 | `string` |
| boolean | - | `bool` |

### 复合类型

| OpenAPI 结构 | Go 类型 |
|---|---|
| object（有 properties） | struct |
| object（仅 additionalProperties） | `map[string]T` |
| array | `[]T` 或 type alias |
| enum | 命名类型 + const 常量组 |
| allOf（仅 $ref） | struct 内嵌（embed） |
| allOf（$ref + properties） | struct 内嵌 + 额外字段 |
| oneOf / anyOf | `json.RawMessage`（保守降级，不生成强类型 union） |
| $ref | 引用对应的 Go 类型名 |

### 可空性规则

- `required` 字段：直接使用值类型
- 非 required 字段：使用指针类型（`*T`）
- slice 和 map 类型本身可为 nil，不额外加指针

## 请求结构体生成

oapi-ginx 会为每个 operation 生成一个 `{OperationName}Req` 结构体，自动合并以下来源的参数：

### 参数绑定

| 参数位置 (in) | struct tag | 说明 |
|---|---|---|
| path | `uri:"name"` | 路径参数，始终 required |
| query | `form:"name"` | 查询参数 |
| header | `header:"name"` | 请求头参数 |
| cookie | `cookie:"name"` | Cookie 参数 |

### 请求体处理

**application/json**：
- 如果 schema 是 `$ref`，生成 embed（内嵌引用类型）
- 如果 schema 是 inline object，将字段平铺到 Req 结构体中（`json:"name"` tag）
- 如果 schema 是非 object 类型，生成 `Body` 字段

**multipart/form-data**：
- 普通字段使用 `form:"name"` tag
- `type: string, format: binary` → `*multipart.FileHeader`
- `type: array, items: {type: string, format: binary}` → `[]*multipart.FileHeader`

**application/x-www-form-urlencoded**：
- 字段使用 `form:"name"` tag

生成的服务端 `Req` 保持 Gin 的 `form` tag 语义：同一个字段可由 URL query string、`application/x-www-form-urlencoded` body 或 `multipart/form-data` body 绑定。

### 示例

OpenAPI 定义：
```yaml
/pets/{pet_id}:
  put:
    operationId: updatePet
    parameters:
      - name: pet_id
        in: path
        required: true
        schema:
          type: integer
          format: int64
    requestBody:
      required: true
      content:
        application/json:
          schema:
            type: object
            required: [name]
            properties:
              name:
                type: string
              tag:
                type: string
```

生成代码：
```go
type UpdatePetReq struct {
    PetID int64   `uri:"pet_id" binding:"required"`
    Name  string  `json:"name" binding:"required"`
    Tag   *string `json:"tag"`
}
```

## 响应类型生成

根据成功 response 的状态码和 content-type 自动选择响应类型：

- 默认选择最小的 `2xx` 响应状态码
- 如果多个 `2xx` JSON 响应 schema 不一致，生成器会报错，避免误选成功响应
- 可在某个 `2xx` response 上设置 `x-ginx-primary-response: true` 明确主成功响应
- `application/json` 优先生成 `{OperationName}Rsp`
- 非 JSON 成功响应按 binary/text 分类
- `204 No Content` 或无 content 的 `2xx` 响应生成 `struct{}`
- 如果没有 `2xx`，但存在 `3xx` 响应，则生成 `ginx.RedirectRsp`

| Content-Type | 生成类型 |
|---|---|
| application/json | `{OperationName}Rsp` 结构体或类型别名 |
| application/octet-stream | `ginx.FileRsp` |
| text/plain | `ginx.StringRsp` |
| text/event-stream | SSE 模式（无响应类型） |
| 仅 3xx 响应（无 2xx） | `ginx.RedirectRsp` |
| 204 No Content | `struct{}` |

### 响应扩展字段

当 content-type 无法表达业务意图时，可使用 `x-ginx-response` 覆盖响应类型。该扩展可放在 operation 上，也可放在具体 response 上；response 级优先。

```yaml
/download:
  get:
    operationId: download
    responses:
      "200":
        description: raw bytes in memory
        x-ginx-response: data
        content:
          application/octet-stream:
            schema:
              type: string
              format: binary
```

支持值：

| `x-ginx-response` | 生成类型 |
|---|---|
| `file` | `ginx.FileRsp` |
| `string` | `ginx.StringRsp` |
| `data` | `ginx.DataRsp` |
| `redirect` | `ginx.RedirectRsp` |

当 JSON 响应 schema 是 `$ref` 时，生成类型别名：
```go
type GetPetRsp = Pet
```

支持的 HTTP 方法包括 `GET`、`HEAD`、`POST`、`PUT`、`PATCH`、`DELETE`、`OPTIONS`。`TRACE` 暂不生成，遇到时会返回明确错误，避免静默丢失 operation。

## 服务接口生成

生成 `ServerInterface` 接口和 `RegisterRoutes` 注册函数：

```go
type ServerInterface interface {
    // GET /pets
    ListPets(ctx context.Context, req *ListPetsReq) (*ListPetsRsp, error)
    // POST /pets
    CreatePet(ctx context.Context, req *CreatePetReq) (*CreatePetRsp, error)
    // DELETE /pets/:pet_id
    DeletePet(ctx context.Context, req *DeletePetReq) (*struct{}, error)
}

func RegisterRoutes(r gin.IRoutes, s ServerInterface, opts ...ginx.RouteOption) {
    ginx.GET(r, "/pets", s.ListPets, opts...)
    ginx.POST(r, "/pets", s.CreatePet, opts...)
    ginx.DELETE(r, "/pets/:pet_id", s.DeletePet, opts...)
}
```

### 自定义接口名前缀 (server_name)

当同一个 package 下需要生成多个 OpenAPI 接口时，使用 `server_name` 避免命名冲突：

```yaml
# user-api.yaml
server_name: user

# order-api.yaml
server_name: order
```

生成结果：
```go
// user-api 生成
type UserServerInterface interface { ... }
func RegisterUserRoutes(r gin.IRoutes, s UserServerInterface, opts ...ginx.RouteOption) { ... }

// order-api 生成
type OrderServerInterface interface { ... }
func RegisterOrderRoutes(r gin.IRoutes, s OrderServerInterface, opts ...ginx.RouteOption) { ... }
```

`server_name` 的值会按 Go 命名规范转为 CamelCase（如 `pet_store` → `PetStore`）。不配置时默认无前缀。

### SSE (Server-Sent Events)

当 operation 满足以下任一条件时，生成 SSE 签名：
- 设置了 `x-ginx-sse: true` 扩展字段
- 响应 content-type 为 `text/event-stream`

SSE handler 签名：
```go
ListEvents(ctx context.Context, req *ListEventsReq, send ginx.Sender) error
```

路由注册使用 `ginx.SSE`：
```go
ginx.SSE(r, "/events", s.ListEvents, opts...)
```

## HTTP 客户端 SDK 生成

配置 `output.client` 或 `output_options.generate_client: true` 后，oapi-ginx 会为每个 operation 生成类型安全的 HTTP 客户端方法，基于 [resty.dev/v3](https://resty.dev)。

### 配置

```yaml
output:
  types: types.gen.go
  server: server.gen.go
  client: client.gen.go    # 指定路径即启用

# 或在单文件模式下通过 output_options 启用
output: api.gen.go
output_options:
  generate_client: true
```

### 生成结构

```go
// ClientOption 用于配置底层 resty 客户端
type ClientOption func(*resty.Client)

// ClientInterface 定义所有 API 方法
type ClientInterface interface {
    ListPets(ctx context.Context, req *ListPetsReq) (*ListPetsRsp, error)
    CreatePet(ctx context.Context, req *CreatePetReq) (*CreatePetRsp, error)
    // ...
}

// Client 实现 ClientInterface
type Client struct {
    client *resty.Client
}

// NewClient 创建客户端实例
func NewClient(baseURL string, opts ...ClientOption) *Client
```

### 使用示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "resty.dev/v3"
    "your/project/api"
)

func main() {
    client := api.NewClient("http://localhost:8080",
        func(c *resty.Client) {
            c.SetTimeout(10 * time.Second)
            c.SetAuthToken("my-token")
        },
    )

    // 调用 API
    rsp, err := client.ListPets(context.Background(), &api.ListPetsReq{
        Limit: ptr(int32(10)),
    })
    if err != nil {
        // err 可能是 *ginx.ErrWrap（API 业务错误）或网络错误
        fmt.Println("error:", err)
        return
    }
    fmt.Println("pets:", rsp)
}
```

### 参数映射

生成的客户端方法复用服务端的 `Req` 结构体，根据 OpenAPI 来源和 content type 分发参数：

| 来源 / tag | 客户端行为 |
|---|---|
| `uri:"name"` | `r.SetPathParam("name", value)` |
| OpenAPI `in: query` + `form:"name"` | `r.SetQueryParam("name", value)` |
| `application/x-www-form-urlencoded` + `form:"name"` | `r.SetFormData(...)` |
| `header:"name"` | `r.SetHeader("name", value)` |
| `cookie:"name"` | `r.SetCookie(&http.Cookie{Name: "name", Value: value})` |
| `json:"name"` | 作为 JSON body 发送 |
| embed（内嵌类型） | `r.SetBody(&req.EmbedType)` |

可选参数（指针类型）在值为 nil 时自动跳过。非 string 类型参数自动使用 `fmt.Sprintf` 转换。

SSE 客户端因直接拼接 EventSource URL，会对 path 参数额外执行 `url.PathEscape`。

### 响应处理

| 服务端响应类型 | 客户端返回值 |
|---|---|
| JSON 结构体 | `(*RspType, error)` |
| `ginx.FileRsp` | `([]byte, error)` |
| `ginx.DataRsp` | `([]byte, error)` |
| `ginx.StringRsp` | `(string, error)` |
| `ginx.RedirectRsp` | `error` |
| `struct{}`（204） | `error` |

### 错误处理

当服务端返回 4xx/5xx 时，客户端返回 `*ginx.ErrWrap` 错误，其中 `HttpCode` 为实际 HTTP 状态码：

```go
rsp, err := client.GetPet(ctx, &api.GetPetReq{PetID: 999})
if err != nil {
    var apiErr *ginx.ErrWrap
    if errors.As(err, &apiErr) {
        fmt.Printf("业务错误 code=%d msg=%s http=%d\n",
            apiErr.Code, apiErr.Msg, apiErr.HttpCode)
    }
}
```

### 当前限制

multipart/form-data 文件上传服务端绑定已支持，但客户端 SDK 暂不生成文件上传方法。只要启用 client 生成且 spec 中包含 `type: string, format: binary` 的 multipart 字段，生成器会直接失败并指出 operation 名称，避免客户端接口中静默缺失方法。

如果项目需要上传客户端，建议先将上传 API 单独拆到 server/types 生成，或后续基于明确的 `io.Reader` / 文件路径模型扩展生成器。

SSE（Server-Sent Events）operation 会生成返回 `*ginx.SSEStream` 的客户端方法，调用方通过 `Recv()` 拉取事件，并在结束时调用 `Close()`。

SSE 客户端会对 path 参数执行 `url.PathEscape`，query/header/cookie 参数沿用普通客户端规则。

### server_name 前缀

与服务端接口一样，`server_name` 配置会影响客户端命名：

```yaml
server_name: pet_store
```

生成：
```go
type PetStoreClientInterface interface { ... }
type PetStoreClient struct { ... }
func NewPetStoreClient(baseURL string, opts ...PetStoreClientOption) *PetStoreClient
```

## 验证规则自动生成

oapi-ginx 根据 OpenAPI schema 约束自动生成 `binding` tag（基于 gin 的 validator）：

| OpenAPI 约束 | binding 规则 |
|---|---|
| required | `required` |
| enum | `oneof=val1 val2 ...` |
| minimum（inclusive） | `gte=N` |
| minimum（exclusive） | `gt=N` |
| maximum（inclusive） | `lte=N` |
| maximum（exclusive） | `lt=N` |
| minLength / minItems | `min=N` |
| maxLength / maxItems | `max=N` |
| uniqueItems | `unique` |
| format: email | `email` |
| format: uri | `url` |
| format: uuid | `uuid` |
| format: ipv4 | `ipv4` |
| format: ipv6 | `ipv6` |
| format: hostname | `hostname` |

### 自定义验证扩展

使用 `x-binding` 扩展字段添加自定义验证规则：

```yaml
properties:
  phone:
    type: string
    x-binding: "e164"
```

生成：
```go
Phone string `json:"phone" binding:"e164"`
```

## 默认值

当 schema 定义了 `default` 值时，自动生成 `default` tag：

```yaml
properties:
  page:
    type: integer
    default: 1
```

生成：
```go
Page *int `form:"page" default:"1"`
```

## 命名规则

### 类型名

所有名称转换为 Go 风格的 CamelCase，并识别常见缩写词（保持全大写）：

`API`, `ASCII`, `CPU`, `CSS`, `DNS`, `EOF`, `HTML`, `HTTP`, `HTTPS`, `ID`, `IP`, `JSON`, `OS`, `QPS`, `RAM`, `RPC`, `SQL`, `SSH`, `TCP`, `TLS`, `TTL`, `UDP`, `UI`, `UID`, `URI`, `URL`, `UTF8`, `UUID`, `VM`, `XML`, `YAML`

示例：
- `pet_id` → `PetID`
- `http_url` → `HTTPURL`
- `user_name` → `UserName`

### Operation 名

优先使用 `operationId`（转为 CamelCase）。如果未定义 operationId，则由 HTTP 方法 + 路径拼接：
- `GET /pets/{pet_id}` → `GetPetsPetId`

## Spec 内嵌

配置多文件输出的 `spec` 字段后，会生成一个包含压缩后 OpenAPI spec 的文件：

```go
func GetSwaggerSpec() ([]byte, error)
```

spec 使用 flate 压缩 + base64 编码存储，运行时解压返回原始 YAML/JSON 内容。适用于需要在运行时提供 API 文档的场景。

## 与 go:generate 集成

推荐在配置文件中设置 `generate_directive`，然后在项目中使用 `go generate`：

```yaml
# oapi-ginx.yaml
package: api
spec: ./openapi.yaml
output:
  types: types.gen.go
  server: server.gen.go
generate_directive: "go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml"
```

生成的文件头部会包含：
```go
//go:generate go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml
```

之后只需运行：
```bash
go generate ./...
```

## AI / Code Agent 使用指南

当你让 AI 或 code agent 基于 OpenAPI 接入 ginx，推荐按这个顺序执行：

1. 先阅读 `oapi-ginx.yaml` 和 OpenAPI spec，确认 `package`、`output`、`server_name`、tag 过滤和是否生成 client。
2. 运行 `go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml` 或项目约定的 `go generate ./...`。
3. 在生成包中实现 `ServerInterface`，handler 签名保持 `context.Context, *Req -> (*Rsp, error)`。
4. 在 Gin 启动代码中调用 `RegisterRoutes(r, svc, opts...)`；如果需要统一策略，先创建并配置 `ginx.Engine`，再用 `engine.Wrap(...)` 注册。
5. 如果启用 client SDK，优先通过 `NewClient(baseURL, opts...)` 注入 resty timeout/auth/retry。
6. 运行 `go test ./...`；修改 codegen 模板后，先跑 `./scripts/test-codegen-e2e.sh` 再跑全量测试。

常见注意点：

- 不要手改 `*.gen.go`，应改 spec、配置或生成器模板。
- 多个 `2xx` JSON 响应 schema 不一致时，添加 `x-ginx-primary-response: true` 或调整 spec。
- 二进制内存响应用 `x-ginx-response: data`，文件下载用 `file`。
- multipart 文件上传客户端暂不生成；服务端类型仍是 `*multipart.FileHeader` / `[]*multipart.FileHeader`。
- 旧顶层 `generate_server` 只为兼容保留，新配置写到 `output_options.generate_server`。

## 当前边界

oapi-ginx 只生成 Go 类型、ginx 服务端接口/路由、可选 resty 客户端和可选 spec embed。它不负责：

- 完整 OpenAPI 文档站点或 Swagger UI 服务
- union/oneOf 强类型模型
- 鉴权、DI、ORM、数据库访问代码
- tracing/metrics SDK 初始化
- multipart 文件上传客户端 SDK

## 完整示例

### OpenAPI Spec (petstore.yaml)

```yaml
openapi: "3.0.3"
info:
  title: Petstore API
  version: "1.0.0"
paths:
  /pets:
    get:
      operationId: listPets
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
            format: int32
        - name: X-Request-ID
          in: header
          schema:
            type: string
      responses:
        "200":
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Pet"
    post:
      operationId: createPet
      parameters:
        - name: X-Idempotency-Key
          in: header
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreatePetInput"
      responses:
        "201":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Pet"
  /pets/{pet_id}:
    get:
      operationId: getPet
      parameters:
        - name: pet_id
          in: path
          required: true
          schema:
            type: integer
            format: int64
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Pet"
components:
  schemas:
    PetStatus:
      type: string
      enum: [available, pending, sold]
    Pet:
      type: object
      required: [id, name, status]
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
        tag:
          type: string
        status:
          $ref: "#/components/schemas/PetStatus"
    CreatePetInput:
      type: object
      required: [name]
      properties:
        name:
          type: string
        tag:
          type: string
        status:
          $ref: "#/components/schemas/PetStatus"
```

### 生成代码

```go
// Code generated by oapi-ginx; DO NOT EDIT.
//go:generate go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml

package api

import (
    "context"

    "github.com/chendefine/ginx"
    "github.com/gin-gonic/gin"
)

type CreatePetInput struct {
    Name   string     `json:"name" binding:"required"`
    Status *PetStatus `json:"status" binding:"oneof=available pending sold"`
    Tag    *string    `json:"tag"`
}

type Pet struct {
    ID     int64     `json:"id" binding:"required"`
    Name   string    `json:"name" binding:"required"`
    Status PetStatus `json:"status" binding:"required,oneof=available pending sold"`
    Tag    *string   `json:"tag"`
}

type PetStatus string

const (
    PetStatusAvailable PetStatus = "available"
    PetStatusPending   PetStatus = "pending"
    PetStatusSold      PetStatus = "sold"
)

type ListPetsReq struct {
    Limit      *int32  `form:"limit"`
    XRequestID *string `header:"X-Request-ID"`
}

type ListPetsRsp = []Pet

type CreatePetReq struct {
    CreatePetInput
    XIdempotencyKey string `header:"X-Idempotency-Key" binding:"required"`
}

type CreatePetRsp = Pet

type GetPetReq struct {
    PetID int64 `uri:"pet_id" binding:"required"`
}

type GetPetRsp = Pet

type ServerInterface interface {
    // GET /pets
    ListPets(ctx context.Context, req *ListPetsReq) (*ListPetsRsp, error)
    // POST /pets
    CreatePet(ctx context.Context, req *CreatePetReq) (*CreatePetRsp, error)
    // GET /pets/:pet_id
    GetPet(ctx context.Context, req *GetPetReq) (*GetPetRsp, error)
}

func RegisterRoutes(r gin.IRoutes, s ServerInterface, opts ...ginx.RouteOption) {
    ginx.GET(r, "/pets", s.ListPets, opts...)
    ginx.POST(r, "/pets", s.CreatePet, opts...)
    ginx.GET(r, "/pets/:pet_id", s.GetPet, opts...)
}
```

### 实现服务

```go
package api

import (
    "context"
    "github.com/chendefine/ginx"
)

type PetService struct{}

func (s *PetService) ListPets(ctx context.Context, req *ListPetsReq) (*ListPetsRsp, error) {
    // 实现业务逻辑
    pets := ListPetsRsp{
        {ID: 1, Name: "Buddy", Status: PetStatusAvailable},
    }
    return &pets, nil
}

func (s *PetService) CreatePet(ctx context.Context, req *CreatePetReq) (*CreatePetRsp, error) {
    // req.Name, req.Tag, req.Status 来自 JSON body（通过 embed CreatePetInput）
    // req.XIdempotencyKey 来自 header
    return nil, ginx.Error(1001, "not implemented")
}

func (s *PetService) GetPet(ctx context.Context, req *GetPetReq) (*GetPetRsp, error) {
    // req.PetID 来自 URI path
    return nil, ginx.Error(1002, "pet not found").Status(404)
}
```

### 注册路由

```go
package main

import (
    "github.com/gin-gonic/gin"
    "your/project/api"
)

func main() {
    r := gin.Default()
    svc := &api.PetService{}
    api.RegisterRoutes(r, svc)
    r.Run(":8080")
}
```
