# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ginx is a type-safe HTTP handler wrapper for the Gin web framework, using Go 1.18+ generics. It provides unified request binding (from headers, URI, query, and JSON body) and standardized error/response wrapping.

## Build & Development Commands

```bash
# Build
go build ./...

# Run demo server (listens on :8081)
go run ./demo/main.go

# Test
go test ./...

# Format
go fmt ./...

# Module maintenance
go mod tidy
```

## Architecture

**Single-package library** (`github.com/chendefine/ginx`) with these core files:

- **ginx.go** — Entry point. Generic handler functions (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`) that accept `func(*Context, *Req) (*Rsp, error)` signatures. Contains `makeHandlerFunc()` which does multi-source binding (header → URI → query → JSON body) and response wrapping logic.
- **context.go** — `Context` wrapper around `*gin.Context` that implements `context.Context` interface. Exposes a controlled surface (Get/Set, headers, cookies, ClientIP, Request). `GetValue[T]()` provides generic type-safe value retrieval. Use `GinContext()` as the explicit escape hatch to access the underlying `*gin.Context`.
- **response.go** — `Response` interface for non-JSON responses. When a handler returns a type implementing `Response`, it bypasses default JSON serialization. Built-in implementations: `FileRsp`, `RedirectRsp`, `StringRsp`, `DataRsp`.
- **error.go** — `ErrWrap` type for structured errors with application code, message, and optional HTTP status override. `Error()` constructor and `Format()` for message formatting.

## Key Patterns

**Handler registration** uses generics with struct tag-based binding:
```go
ginx.GET(router, "/path/:id", func(ctx *ginx.Context, req *MyReq) (*MyRsp, error) {
    return &MyRsp{}, nil
})
```

**Request structs** use tags for multi-source binding: `uri:"id"`, `header:"X-Token"`, `form:"page"`, `json:"body"`, with validation via `binding:"required"`.

**HandleOption constants** control behavior: `DataWrap`, `NoDataWrap`, `StatusCodeAlwaysOK`.

**Error responses** use `ginx.Error(code, msg, optionalHttpCode)`. If `HttpCode` is in 101–599, it's used as the HTTP status; otherwise defaults to 500.

**Response interface**: Return `*FileRsp`, `*RedirectRsp`, `*StringRsp`, or `*DataRsp` (or any `Response` implementor) from handlers to bypass JSON wrapping entirely.

**Global config** functions: `SetDataWrap()`, `SetInvalidArgumentCode()`, `SetInternalServerErrorCode()`, `SetJsonDecoderUseNumber()`.

## Language

Commit messages and comments are in Chinese. Follow existing convention.
