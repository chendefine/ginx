# ginx Runtime and Business Implementation

Use the ginx runtime to execute the actual API behavior after code generation, or to define a service directly in Go when OpenAPI is intentionally not used.

For contract-driven services, first follow [codegen.md](codegen.md). Treat generated request/response types, `ServerInterface`, and `RegisterRoutes` as the protocol boundary, then implement the business behavior described here in non-generated Go files.

## Implement after code generation

1. Inspect the generated request/response types and `ServerInterface` method signatures.
2. Create a handwritten service type with the dependencies needed by the business domain.
3. Add a compile-time assertion that the service implements the generated interface.
4. Implement each method with the generated request and response types; call application/domain services from those methods.
5. Configure a dedicated `ginx.Engine` and pass the service to the generated `RegisterRoutes` function.
6. Test binding, validation, business behavior, errors, statuses, and response bodies through HTTP.

Do not duplicate generated request/response structs, re-register generated operations by hand, or place business logic in generated files.

## Implement without OpenAPI

1. Define one request struct and one response type per operation.
2. Bind each request field from its actual HTTP source and add validation rules.
3. Implement the handler with the standard `context.Context` signature.
4. Create and configure a dedicated `ginx.Engine` before route registration.
5. Wrap a Gin router or group with the Engine and register typed routes.
6. Test successful input, every validation boundary, business failures, and internal-error masking.

## Handwritten route example

```go
package main

import (
	"context"
	"net/http"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

type CreateUserReq struct {
	OrgID  string `uri:"org_id" binding:"required"`
	Token  string `header:"X-Token" binding:"required"`
	DryRun bool   `form:"dry_run"`
	Name   string `json:"name" binding:"required,max=100"`
	Email  string `json:"email" binding:"required,email"`
}

type CreateUserRsp struct {
	ID string `json:"id"`
}

func CreateUser(ctx context.Context, req *CreateUserReq) (*CreateUserRsp, error) {
	if req.Name == "taken" {
		return nil, ginx.Error(1001, "user already exists").Status(http.StatusConflict)
	}
	return &CreateUserRsp{ID: "user-1"}, nil
}

func main() {
	r := gin.Default()
	engine := ginx.New(
		ginx.WithStrictJSONBody(true),
		ginx.WithExposeInternalError(false),
		ginx.WithInternalErrorMessage("internal error"),
	)
	api := engine.Wrap(r.Group("/api/v1"))
	ginx.POST(api, "/orgs/:org_id/users", CreateUser)
	_ = r.Run(":8080")
}
```

Expect wrapped JSON success by default:

```json
{"code":0,"msg":"","data":{"id":"user-1"}}
```

## Handler and context rules

Use the ordinary handler contract:

```go
func(ctx context.Context, req *Req) (*Rsp, error)
```

Use the streaming variants only for streaming routes:

```go
func(ctx context.Context, req *Req, send ginx.Sender) error
func(ctx context.Context, req *Req, send ginx.JSONLinesSender) error
```

Keep services dependent on `context.Context`, not `*gin.Context`. At an HTTP boundary, access Gin-specific state with:

```go
gc, ok := ginx.GinContext(ctx)
```

Use `ginx.Get`, `Set`, `GetHeader`, `SetHeader`, `AddHeader`, `ClientIP`, `Request`, `Cookie`, and `SetCookie` when their context-based helpers are enough. Never retain the Gin context beyond the request lifetime.

## Request binding

Use these tags:

| HTTP source | Struct tag |
| --- | --- |
| Path parameter | `uri:"id"` |
| Query, form-urlencoded, or multipart field | `form:"name"` |
| Header | `header:"X-Token"` |
| Cookie | `cookie:"sid"` |
| JSON body | `json:"name"` |
| Default | `default:"1"` |
| Validation | `binding:"required,email,max=100"` |

Use `*multipart.FileHeader` or `[]*multipart.FileHeader` for multipart uploads. Combine path, query, header, and cookie binding with the one body format selected by `Content-Type`.

Remember these details:

- Apply defaults before binding and validation.
- Select JSON binding only for JSON or `+json` content types.
- Treat `WithStrictJSONBody(true)` as rejection of a second/trailing JSON value; it does not reject unknown object fields.
- Use pointer fields when omission must differ from a scalar zero value.
- Let binding errors flow through ginx validation handling instead of repeating parsing inside the business handler.

## Engine and route policy

Prefer an instance Engine:

```go
engine := ginx.New(opts...)
api := engine.Wrap(r.Group("/api"))
// Or: api := engine.Group(r, "/api")
```

Configure it before registering any route. `Engine.Configure` and package-level `ginx.Configure` affect only subsequently registered routes because each route captures a configuration snapshot.

Use package-level `ginx.GET`, `POST`, and `Set*` with the shared default Engine only for simple programs with startup-only configuration. Avoid shared mutable defaults in parallel tests or services with different policies.

Register ordinary routes with `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, or `OPTIONS`. Use `Any` for the seven common methods and `Handle` for an explicit method list.

Apply route options deliberately:

- Use `NoDataWrap()` for raw JSON and `WrapData()` to force the `{code,msg,data}` envelope.
- Use `SuccessStatus(code)` only with a fixed 2xx success contract; invalid values panic during route registration.
- Use `AlwaysOK()` only for protocols that require HTTP 200 even on errors.
- Use `RouteInterceptor(...)` for typed, route-specific interception.
- Expect HEAD and 204 responses to have no response body.

## Errors and response rendering

Return domain failures as `*ginx.ErrWrap`:

```go
return nil, ginx.Error(1004, "user not found").Status(http.StatusNotFound)
```

Treat an ordinary `error` as an internal failure. Public services should use `WithExposeInternalError(false)` plus `WithInternalErrorMessage(...)`, or supply `WithErrorHandler(...)`.

Customize only the required layer:

- Use `WithValidationErrorHandler` for validation localization or a custom error envelope.
- Use `ginx.FormatValidationError` to reuse ginx's validation formatting.
- Use `WithErrorHandler` for ordinary errors.
- Use `WithSuccessHandler` for wrapped JSON success bodies only.
- Use `WithJSONRenderer` to replace JSON serialization/writing.

Return non-JSON responses through constructors so invalid status codes are normalized:

```go
return ginx.StringResponse(http.StatusOK, "ok"), nil
return ginx.DataResponse(http.StatusOK, "application/octet-stream", data), nil
return ginx.FileResponse(path, "report.pdf"), nil
return ginx.RedirectResponse(http.StatusFound, "/login"), nil
```

## Middleware and interceptors

Use Gin middleware for raw HTTP concerns that must run before binding. Use `ginx.Interceptor` for typed request concerns after binding:

```go
ginx.WithInterceptor(func(
	ctx context.Context,
	req any,
	next func() (any, error),
) (any, error) {
	return next()
})
```

Apply Engine interceptors outside route interceptors. Call `next()` at most once. Return its original response, `nil`, or the exact response pointer type declared by the handler.

## Streaming

Register SSE with `ginx.SSE`; it always uses GET. Send `ginx.Event` values through the supplied sender. Implement heartbeats, reconnection semantics, broadcasting, and connection governance above ginx.

Register NDJSON/JSON Lines with `ginx.JSONLines(r, method, path, handler)`. Each send writes one compact JSON value followed by `\n` and flushes. An error before the first record uses the normal JSON error response; an error after streaming begins terminates the stream and is recorded in Gin errors without appending an error envelope.

## Verification checklist

- Compile the exact handler signature through route registration.
- Exercise path/query/header/cookie/body combinations with `httptest`.
- Verify defaults, required fields, validation limits, malformed bodies, and trailing JSON values.
- Verify the actual wire status and body for success, business errors, internal errors, HEAD, and 204.
- Verify stream headers, empty streams, send failures, and client cancellation where applicable.
- Run `gofmt` and `go test ./... -count=1`.
