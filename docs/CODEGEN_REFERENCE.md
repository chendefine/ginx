# oapi-ginx 完整参考

这是 `oapi-ginx` 的完整参考手册，保留类型映射、请求/响应生成规则、验证规则和长示例。首次接入建议先阅读 [README_CODEGEN.md](../README_CODEGEN.md) 的主线工作流，再按需回到本文查细节。

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
  unwrap_envelope: true     # 自动探测并解包 ginx {code,msg,data} 响应封装（默认 true）
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

## 请求类型生成

oapi-ginx 会为每个 operation 生成一个 `{OperationName}Req` 类型。大多数请求使用结构体合并以下来源的参数；当唯一入参是 JSON `$ref` body 时，直接生成指向引用 schema 的类型别名。

### 参数绑定

| 参数位置 (in) | struct tag | 说明 |
|---|---|---|
| path | `uri:"name"` | 路径参数，始终 required |
| query | `form:"name"` | 查询参数 |
| header | `header:"name"` | 请求头参数 |
| cookie | `cookie:"name"` | Cookie 参数 |

### 请求体处理

**application/json**：
- 如果 schema 是 `$ref` 且 operation 没有 path/query/header/cookie 参数，生成 `type XxxReq = RefType`
- 如果 schema 是 `$ref` 且 operation 还有其他参数，生成组合 Req 并 embed（内嵌）引用类型
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
- 除 `ginx.FileRsp` 外，选中的非 200 2xx 会作为最后一个路由选项生成 `ginx.SuccessStatus(code)`，因此通用 `opts` 不能覆盖 operation 契约
- 客户端接受与主响应 schema/类型兼容的全部声明状态；使用 primary 解决不兼容分支时只接受 primary 状态
- Simple operation 的 HEAD 和 204 客户端方法只返回 `error`，不会解析或返回响应 body；variants 仍返回判别容器

| Content-Type | 生成类型 |
|---|---|
| application/json | `{OperationName}Rsp` 结构体或类型别名 |
| application/octet-stream | `ginx.FileRsp` |
| text/plain | `ginx.StringRsp` |
| text/event-stream | SSE 模式（无响应类型） |
| 仅 3xx 响应（无 2xx） | `ginx.RedirectRsp` |
| 204 No Content | `struct{}` |

`ginx.FileRsp` 使用 `http.ServeFile`，无 Range 时实际返回 200，有效 Range 时可返回 206。文件 operation 必须声明 200；可额外声明 content/schema 兼容的 206，但不能声明其他 2xx 文件状态。生成路由不会为文件响应追加 `SuccessStatus`，以免产生无法兑现的固定状态契约。

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

### 可选 response variants

默认 Simple Server 保持 `context.Context + *Req -> (*Rsp, error)`。只有 operation 显式设置 `x-ginx-response-mode: variants` 时，才为多个成功分支生成判别容器：

```go
type CreateJobResponse struct { /* private discriminant and branches */ }

func NewCreateJob201Response(body *CreateJob201Rsp) *CreateJobResponse
func NewCreateJob202Response(body *CreateJob202Rsp) *CreateJobResponse
func NewCreateJob204Response() *CreateJobResponse
func (r *CreateJobResponse) StatusCode() int
func (r *CreateJobResponse) As201() (*CreateJob201Rsp, bool)
```

服务端通过构造器选分支，客户端按实际状态解析后返回同一种容器。首版只接受显式数字 2xx/3xx，分支必须是 `application/json` 或无 body；4xx/5xx 继续走 error 通道。不支持文件、文本、多媒体、SSE/JSON Lines、typed response headers、wildcard/default response，也不能与 `x-ginx-primary-response` 或 `x-ginx-response` 同时使用，遇到这些组合时生成器直接报错。

当 JSON 响应 schema 是 `$ref` 时，生成类型别名：
```go
type GetPetRsp = Pet
```

### 响应封装自动解包（unwrap_envelope）

ginx 运行时默认把成功响应包装成 `{"code":0,"msg":"","data":{...}}`，因此规范的 response schema 只需描述 `data` 里的业务数据。但若 spec 把 response 直接写成了整层封装 `{code,msg,data}`，codegen 会原样生成三字段 `Rsp`，运行时再包一层，导致 wire 上出现双壳。为避免这种情况，codegen 默认开启封装自动解包（`output_options.unwrap_envelope: true`）。

判定规则（严格匹配，命中才解包）：

- response 的 `application/json` schema 是一个对象；
- **恰好**三个属性 `code`、`msg`、`data`（不多不少）；
- `code` 类型为 `integer`，`msg` 类型为 `string`；
- 存在 `data` 子 schema（`required` 是否声明不影响判定）。

命中后，`{OperationName}Rsp` 只依据 `data` 子 schema 生成（`data` 是 `$ref` 则生成别名、是 inline object 则生成结构体、是 array/primitive 则生成对应类型），ginx 运行时再补上单层封装。`code` 必须是 `integer` 是关键防误判项——业务里名为 `code` 的字段多为字符串（错误码/状态码），不会被误判为封装。

```yaml
# spec 中误写了整层封装 —— 会被自动解包
responses:
  "200":
    content:
      application/json:
        schema:
          type: object
          properties:
            code: { type: integer }
            msg:  { type: string }
            data: { $ref: "#/components/schemas/User" }
# 生成: type GetUserRsp = User
```

#### `allOf` 组合的可复用封装

除了直接写出三字段封装，OpenAPI 还有一种更常见的写法：定义一个可复用的 `Envelope` 组件（`data` 为泛型占位），再用 `allOf` 把它与具体 `data` 组合起来。这种写法同样会被识别并解包：

```yaml
components:
  schemas:
    Envelope:
      type: object
      properties:
        code: { type: integer }
        msg:  { type: string }
        data: { description: 业务数据 }   # 泛型占位，无具体类型
    UserProfile:
      type: object
      properties: { id: {type: string}, name: {type: string} }
paths:
  /account:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                allOf:
                  - $ref: "#/components/schemas/Envelope"
                  - properties:
                      data:
                        $ref: "#/components/schemas/UserProfile"
# 生成: type GetAccountRsp = UserProfile
```

判定与合并语义：

- 把 `allOf` 各成员（`$ref` 成员解析到其值）的属性递归合并后，若整体形状仍是「恰好三字段 `code`(integer)/`msg`(string)/`data`」即视为封装并解包。
- 合并时**更具体的定义优先**：泛型 `data`（如只有 `description`）永远不会覆盖具体的 `data`，与 `allOf` 成员的书写顺序无关；具体 `data` 之间则按后写覆盖。
- 因此下列 `allOf` 变体都会被识别：`{$ref:Envelope}` + 内联 `data` 覆盖、两个成员都是 `$ref`（封装 + 承载 `data` 的组件）、`code`/`msg` 与 `data` 拆分在不同成员里、封装壳本身由 `allOf` 定义再被 `$ref` 引用、响应 schema 同时带 `allOf` 和自身 `properties` 等。
- 若合并后超过三字段（例如在 `data` 之外又加了业务字段），则**不是**封装壳，按既有 `allOf` 规则生成（不解包）。
- 仅 `allOf` 参与判定。`oneOf`/`anyOf` 表示「多选一」而非属性合并，不会被当作封装壳。

#### OpenAPI 3.1 可空类型

OpenAPI 3.1 用类型数组表达可空（如 `["integer","null"]`）。封装判定基于 `type` 集合的成员关系（容忍 `null` 伴随项），所以 3.1 的可空封装同样会被解包：

```yaml
# 3.1 可空封装 —— 会被自动解包
schema:
  type: ["object", "null"]
  properties:
    code: { type: ["integer", "null"] }
    msg:  { type: ["string", "null"] }
    data: { $ref: "#/components/schemas/User" }
# 生成: type GetUserRsp = User
```

`allOf` 组合的可复用封装在 3.1 下（成员字段为可空类型数组）同样适用。3.0 的单值 `type` 与 3.1 的类型数组在判定上等价。

注意：

- 这只影响生成的 Go 类型；内嵌 spec（供 Swagger UI / 非本工具生成的客户端使用）仍保留原始 `{code,msg,data}` 契约。
- 客户端 SDK 无需改动：`ParseResponse` 已通用识别封装并提取 `data`，解包后的类型对客户端同样生效。
- 若某个业务响应**确实**就是 `{code:int, msg:string, data:T}` 三字段结构（非传输封装），会被静默解包为 `type XxxRsp = T`；运行时仍由 ginx 包装 `T`，round-trip 不受影响，但若需保留三字段结构，可设 `output_options: { unwrap_envelope: false }` 关闭。

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

### JSON Lines / NDJSON 流式 (OpenAPI 3.2)

当 operation 满足以下任一条件时，生成 JSON Lines / NDJSON 流式签名：
- 设置了 `x-ginx-jsonl: true` 扩展字段
- 成功响应 content-type 为 `application/jsonl` 或 `application/x-ndjson`

`application/json-seq`（RFC 7464）**不**被识别（其 `0x1E` 分隔符会破坏按行切分）。

handler 签名（每个 item 经 `send` 写为紧凑 JSON + `\n` 并立即 flush）：
```go
TailLogs(ctx context.Context, req *TailLogsReq, send ginx.JSONLinesSender) error
```

路由注册使用 `ginx.JSONLines`（method 作参数，NDJSON 惯例 POST 但不强制）：
```go
ginx.JSONLines(r, "GET", "/logs/:source/tail", s.TailLogs, opts...)
```

JSON Lines 与 SSE 当前只支持 200 成功响应；声明其他成功状态会在生成期报错。

客户端方法返回 `*ginx.JSONLinesStream`，内部用 `resty` 的 `SetResponseDoNotParse(true)` 关闭响应缓冲，`Recv()` 返回每行 JSON 的 `json.RawMessage`，调用方自行 `json.Unmarshal`，结束时 `Close()`。

handler 在首条记录发送前失败时，返回正常的 HTTP JSON 错误；一旦流已经开始，后续错误只会记录到 `gin.Context.Errors` 并结束流，不会把错误 envelope 追加为伪造的 NDJSON 业务记录。

> **类型说明**：item 类型仍为无类型（`any` / `json.RawMessage`）。kin-openapi v0.142.0 已保留 OpenAPI 3.2 `itemSchema`，但 ginx 为保持现有 `JSONLinesSender` / `JSONLinesStream` API 兼容性，暂未生成强类型 item wrapper。调用方可依据 `itemSchema` 对 `json.RawMessage` 自行反序列化（与 SSE 的 `Event.Data any` 类似）。

### Webhooks (OpenAPI 3.1)

顶层 `webhooks` 下的每个入站 operation 会生成接收端处理器。webhook 名是标识符而非 URL，ginx 合成为确定性路由 `/webhooks/<name>`（小写、非法字符替换为 `-`），按 key 字典序处理以保证输出可复现。webhook 与普通 path operation 走同一套模板（支持 JSON / SSE / JSON Lines 响应）。

```yaml
webhooks:
  orderCreated:
    post:
      operationId: handleOrderCreated
      requestBody: { required: true, content: { application/json: { schema: { ... } } } }
      responses: { "200": { description: ack } }
```
生成 `ginx.POST(r, "/webhooks/ordercreated", s.HandleOrderCreated, opts...)`。

### OpenAPI 3.1 schema 特性

- **`const`** → 校验规则 `oneof=<value>`（仅对 `string`/`integer`/`number` 生成；validator 的 `oneof` 会在 `bool` 字段上 panic，故布尔 const 仅作文档，不生成 binding）。
- **`prefixItems`（元组）** → `[]any`（位置类型丢失，JSON 数组不会自动反序列化进 Go struct）。
- **可空 type 数组**（`["string","null"]`、`["array","null"]` 等）→ 标量变指针；可空数组变 `[]<elem>`（Go 切片本身即可为 nil）。
- 数值型 `exclusiveMinimum`/`exclusiveMaximum`（独立边界）→ `gt=`/`lt=`。
- `webhooks`、`$defs`、license `identifier`、Path Item `$ref` 等均可解析且不破坏生成。

### OpenAPI 3.2 支持边界

`openapi: "3.2.0"` 文档可被 kin-openapi v0.142.0 加载与校验（按 3.1-or-later 处理）。可工作的 3.2 特性：SSE、带 `itemSchema` 的 JSON Lines（见上）、`in: querystring`（归一化为普通 query 参数；其结构化“整个 query 串当一个 schema”形式不可表达）。

> **库限制**：kin-openapi v0.142.0 的 `Validate()` 仍会**拒绝** OpenAPI 3.2 的 `QUERY` 方法、`additionalOperations` 和结构化 Tags（`kind`/`parent`/`summary`）。这些会在校验阶段以清晰错误报出（而非静默误生成）；待上游提供对应结构后再扩展生成能力。

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
| `$ref` body 别名 | `r.SetBody(req)` |
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
| HEAD（不论 schema） | `error` |

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

所有非 SSE 的生成客户端，以及 JSON Lines 客户端，都会先处理 4xx/5xx，再用 `ginx.ValidateResponseStatus` 校验实际状态，最后才解析 body。未声明的 `<400` 状态返回可通过 `errors.As` 识别的 `*ginx.UnexpectedStatusError`。包含 3xx operation 时，`NewClient` 默认使用 `resty.RedirectNoPolicy()` 以观察原始状态和 `Location`；该默认在 `ClientOption` 前应用，调用方可显式恢复跟随策略。

即使 4xx/5xx body 错误地使用了 `code: 0` 的成功封装，客户端仍返回 `*ginx.ErrWrap`，不会把 HTTP error 当成成功。

### 响应契约升级说明

重新生成旧项目时，201/202/204 operation 的真实 wire status 可能从历史上的 200 改为 spec 声明值；Simple operation 的 HEAD/204 客户端签名可能收紧为仅返回 `error`；包含 3xx operation 的客户端默认不再跟随重定向；所有生成客户端会拒绝未声明的 `<400` 状态。文件响应还需满足“200，及可选的兼容 206”，SSE/JSON Lines 必须使用 200。Simple Server 的 handler 签名保持不变，但服务实现、客户端调用点和 HTTP 断言应在重新生成后一起编译验证。

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

开始前，先读取 [AGENTS.md](../AGENTS.md) 和 [skills/ginx-http-backend/SKILL.md](../skills/ginx-http-backend/SKILL.md)，以获得面向 AI 的最短操作规则。

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
