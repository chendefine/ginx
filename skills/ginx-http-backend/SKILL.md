---
name: ginx-http-backend
description: Use when building or modifying Go HTTP backend services with github.com/chendefine/ginx, including typed Gin routes, OpenAPI/oapi-ginx generated servers or clients, request binding, validation, response wrapping, SSE, and ginx runtime safety options.
---

# ginx HTTP Backend

Use this skill when the task is to create, change, or review Go backend code that uses `github.com/chendefine/ginx`.

For local repository work, prefer `README.md` and `README_CODEGEN.md` as the concise main-path docs. Open `docs/RUNTIME_REFERENCE.md` or `docs/CODEGEN_REFERENCE.md` only when detailed reference behavior is needed.

## Core Model

`ginx` is a typed adapter over Gin. Business handlers should use:

```go
func(ctx context.Context, req *Req) (*Rsp, error)
```

Do not use old `func(ctx *ginx.Context, ...)` patterns. Use standard `context.Context`; call `ginx.GinContext(ctx)` only at HTTP boundaries when Gin-specific behavior is needed.

## Recommended Runtime Setup

Prefer a dedicated `Engine` over package-level defaults:

```go
engine := ginx.New(
	ginx.WithStrictJSONBody(true),
	ginx.WithExposeInternalError(false),
	ginx.WithInternalErrorMessage("internal error"),
)

r := gin.Default()
api := engine.Wrap(r.Group("/api"))
ginx.POST(api, "/users", CreateUser)
```

Rules:

- Configure before route registration. Engine config is snapshotted when each route is registered.
- Package-level `ginx.Set*` mutates the shared default engine; avoid it in tests and multi-module services.
- Public services should not expose plain `err.Error()`; use `WithExposeInternalError(false)`, `WithInternalErrorMessage`, or `WithErrorHandler`.
- `WithStrictJSONBody(true)` rejects trailing JSON tokens while preserving empty-body validation behavior.

## Request Types

Use struct tags to bind from multiple sources:

```go
type CreateUserReq struct {
	OrgID string `uri:"org_id" binding:"required"`
	Token string `header:"X-Token" binding:"required"`
	Page  *int   `form:"page" default:"1"`
	Name  string `json:"name" binding:"required,max=100"`
	Email string `json:"email" binding:"required,email"`
}
```

Binding sources:

- `uri:"id"` for path params.
- `form:"q"` for query, form-urlencoded, and multipart form fields.
- `header:"X-Token"` for headers.
- `cookie:"sid"` for cookies.
- `json:"name"` for JSON body.
- `*multipart.FileHeader` or `[]*multipart.FileHeader` for server-side multipart upload fields.

## Responses And Errors

Default success response is `{code,msg,data}`. Use route options when needed:

- `ginx.NoDataWrap()` for raw JSON response.
- `ginx.WrapData()` to force wrapping on one route.
- `ginx.AlwaysOK()` only for legacy protocols that require HTTP 200.
- `ginx.RouteInterceptor(...)` for route-specific typed middleware.

Return structured business errors with:

```go
return nil, ginx.Error(1001, "user not found").Status(http.StatusNotFound)
```

For non-JSON responses, use constructors:

```go
return ginx.StringResponse(http.StatusOK, "ok"), nil
return ginx.DataResponse(http.StatusOK, "application/octet-stream", data), nil
return ginx.FileResponse(path, "report.pdf"), nil
return ginx.RedirectResponse(http.StatusFound, "/login"), nil
```

Use constructors instead of direct `StringRsp` / `DataRsp` / `RedirectRsp` literals so invalid status codes are normalized.

## Interceptors

Interceptors receive already-bound `req`:

```go
ginx.WithInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
	rsp, err := next()
	return rsp, err
})
```

Rules:

- Call `next()` at most once. Repeated calls panic.
- Return `next()` result, `nil`, or the exact handler response pointer type.
- Use Gin middleware for raw transport concerns before binding; use ginx interceptors for typed request concerns.

## OpenAPI Codegen Workflow

When a project has `oapi-ginx.yaml`, prefer codegen:

```bash
go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml
```

Then:

1. Implement the generated `ServerInterface`.
2. Register with `RegisterRoutes(r, svc, opts...)`.
3. Use `server_name` when multiple specs generate into one package.
4. Use `output_options.generate_server` / `generate_client`; top-level `generate_server` is deprecated.
5. Do not hand-edit `*.gen.go`; change the spec, config, or templates.

Important OpenAPI extensions:

- `x-ginx-sse: true` marks an operation as SSE.
- `x-ginx-response: file|string|data|redirect` overrides response classification.
- `x-ginx-primary-response: true` selects the primary 2xx response when multiple JSON schemas exist.

Current limitation: multipart file upload server binding is supported, but generated client SDK fails clearly for multipart file upload operations instead of silently omitting methods.

## Validation And Tests

Use `WithValidationErrorHandler` for i18n or custom validation bodies. Reuse `ginx.FormatValidationError(err, namer)` if you want ginx default messages in a custom envelope.

Run:

```bash
go test ./... -count=1
```

If codegen templates, fixture specs, or fixture configs changed, also run:

```bash
./scripts/test-codegen-e2e.sh
```

The e2e script removes stale ignored `*.gen.go` files before regenerating.
