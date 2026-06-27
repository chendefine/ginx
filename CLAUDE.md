# CLAUDE.md

This file gives code agents the current working context for this repository.

## Project

`github.com/chendefine/ginx` is a type-safe adapter layer on top of Gin. Runtime handlers use:

```go
func(ctx context.Context, req *Req) (*Rsp, error)
```

`ginx` handles multi-source binding, validation, response wrapping, non-JSON responses, SSE, interceptors, and optional OpenAPI code generation.

Go version is `go 1.25` in `go.mod`.

## Commands

```bash
go test ./...
go test ./internal/codegen -count=1
./scripts/test-codegen-e2e.sh
go run ./examples/basic
go run ./cmd/oapi-ginx -c path/to/oapi-ginx.yaml
```

Run `./scripts/test-codegen-e2e.sh` after changing codegen templates or generated fixture specs. The script deletes stale `*.gen.go` files before regenerating.

## AI Usage Entry Points

- `AGENTS.md` is the generic repository entry point for AI coding agents.
- `skills/ginx-http-backend/SKILL.md` is a portable Codex-style skill for building Go HTTP services with ginx. Read it before implementing service code or OpenAPI/codegen workflows.
- `README.md` and `README_CODEGEN.md` are concise main-path docs for humans and agents.
- `docs/RUNTIME_REFERENCE.md` and `docs/CODEGEN_REFERENCE.md` keep the long-form runtime/codegen reference details.

## Runtime Architecture

- `ginx.go`: public registration functions (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`, `Any`, `Handle`, `SSE`) and core handler/SSE types.
- `engine.go`: `Engine`, `EngineOption`, route options, config snapshot resolution, package-level default engine.
- `internals.go`: binding plan cache, request binding, validation formatting, handler invocation, error/success response writing.
- `context.go`: helpers around `context.Context` and the underlying `*gin.Context`.
- `response.go`: non-JSON response interface and helpers (`FileRsp`, `RedirectRsp`, `StringRsp`, `DataRsp`).
- `error.go`: `ErrWrap`, `Error`, `Status`, `Format`, `errors.Is` support.
- `client.go`: generated-client response parsing and SSE stream client wrapper.

Important runtime semantics:

- Engine config is snapshotted when a route is registered. `Configure`, `Set*`, and `(*Engine).Configure` affect only routes registered later.
- Prefer independent `ginx.New(...)` engines in tests and real services. Package-level `Set*` mutates shared default state.
- Strict JSON is opt-in via `WithStrictJSONBody(true)` / `SetStrictJSONBody(true)`.
- Plain `error` still exposes `err.Error()` by default for compatibility. Public services should use `WithExposeInternalError(false)`, `WithInternalErrorMessage(...)`, or `WithErrorHandler(...)`.
- Interceptor `next()` can be called once. Repeated calls panic. Interceptors must return `*Rsp` or `nil`.
- `StringResponse`, `DataResponse`, and `RedirectResponse` normalize invalid status codes; direct struct literals do not.

## Codegen Architecture

- CLI entry: `cmd/oapi-ginx/main.go`
- Config: `internal/codegen/config.go`
- OpenAPI operation extraction: `internal/codegen/operation.go`
- Schema/type generation: `internal/codegen/schema.go`, `typemap.go`, `naming.go`, `validation.go`
- Validation of generated names/client support: `internal/codegen/validate.go`
- Templates: `internal/codegen/templates/*.tmpl`
- Tests: `internal/codegen/codegen_e2e_test.go` plus `internal/codegen/e2etest`

Current codegen behavior:

- Generates request/response types, `ServerInterface`, `RegisterRoutes`, optional resty client SDK, optional compressed spec embed.
- Supports multi-file output (`types`, `server`, `client`, `spec`) and single-file output.
- `output_options.generate_server` is the preferred server toggle. Top-level `generate_server` is deprecated but still supported.
- `x-ginx-sse: true` or `text/event-stream` creates SSE server/client methods.
- `x-ginx-response: file|string|data|redirect` overrides response classification.
- Multiple `2xx` JSON responses with different schemas fail generation unless one response has `x-ginx-primary-response: true`.
- Multipart file upload server types are supported, but client generation fails clearly for multipart file upload operations.
- SSE client path params are URL path escaped.
- OpenAPI 3.1/3.2 are supported (specs live under `internal/codegen/e2etest/openapi-3.<x>/`). The harness is version-aware: `specPath(version, name)`; legacy `testdataPath` resolves to `openapi-3.0`. `scripts/test-codegen-e2e.sh` globs `openapi-3.*/code/*`.
- JSON Lines / NDJSON streaming: `x-ginx-jsonl: true` or a `application/jsonl`/`application/x-ndjson` success response generates a `ginx.JSONLines` handler (`send ginx.JSONLinesSender`) and a client returning `*ginx.JSONLinesStream` (uses resty `SetResponseDoNotParse`). `application/json-seq` is NOT supported.
- OpenAPI 3.1 additions: `const`→`oneof` (string/number only; bool const skipped — validator panics), `prefixItems` tuples→`[]any`, nullable type arrays (`["X","null"]`) resolve to pointer scalars / typed slices, top-level `webhooks` synthesize `/webhooks/<name>` receiver routes, numeric `exclusiveMinimum/Maximum`.
- OpenAPI 3.2: `openapi: "3.2.0"` loads/validates; `in: querystring` is normalized to `query`. Brand-new 3.2 struct fields (`itemSchema`, `query` method, `additionalOperations`, structured Tags) are REJECTED by kin-openapi v0.140.0's `Validate()` (latest version; no upgrade path yet) with a clear error — see `TestE2E_OAI32_UnsupportedStructuralFieldsRejected`.

## Editing Notes

- Use `rg` for searches.
- Use `apply_patch` for source/doc edits.
- Do not edit generated `*.gen.go` fixtures by hand; edit specs/templates and run `./scripts/test-codegen-e2e.sh`.
- Keep behavior backward-compatible unless the user explicitly asks for a breaking change.
- README and README_CODEGEN are user-facing Chinese quick-entry docs; keep them concise and accurate when API behavior changes. Put long reference material in `docs/RUNTIME_REFERENCE.md` or `docs/CODEGEN_REFERENCE.md`.
