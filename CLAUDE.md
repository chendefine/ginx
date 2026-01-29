# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ginx is a type-safe HTTP handler wrapper for the Gin web framework, using Go 1.18+ generics. It provides unified request binding (from headers, URI, query, and JSON body), standardized error/response wrapping, and automatic Protobuf schema generation for API documentation.

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

- **ginx.go** — Entry point. Generic handler functions (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`) that accept `func(context.Context, *Req) (*Rsp, error)` signatures. Contains `makeHandlerFunc()` which does multi-source binding (header → URI → query → JSON body) and response wrapping logic.
- **error.go** — `ErrWrap` type for structured errors with application code, message, and optional HTTP status override. `Error()` constructor and `Extend()` for message formatting.
- **regist.go** — Reflection-based type introspection. Parses Go structs into Protobuf message definitions, handles nested/recursive types, and registers RPC service endpoints.
- **doc.go** — Serves auto-generated Protobuf schema at `GET /doc/pb` by compiling all registered types and endpoints.
- **utils.go** — Path joining and function name extraction helpers.

## Key Patterns

**Handler registration** uses generics with struct tag-based binding:
```go
ginx.GET(router, "/path/:id", func(ctx context.Context, req *MyReq) (*MyRsp, error) {
    return &MyRsp{}, nil
})
```

**Request structs** use tags for multi-source binding: `uri:"id"`, `header:"X-Token"`, `form:"page"`, `json:"body"`, with validation via `binding:"required"`.

**HandleOption constants** control behavior: `DataWrap`, `NoDataWrap`, `NoPbParse`, `StatusCodeAlwaysOK`.

**Error responses** use `ginx.Error(code, msg, optionalHttpCode)`. If `HttpCode` is in 100–599, it's used as the HTTP status; otherwise defaults to 500.

**Global config** functions: `SetDataWrap()`, `SetInvalidArgumentCode()`, `SetInternalServerErrorCode()`, `SetJsonDecoderUseNumber()`, `SetServeDoc()`, `SetNamePkgPrefix()`.

## Language

Commit messages and comments are in Chinese. Follow existing convention.
