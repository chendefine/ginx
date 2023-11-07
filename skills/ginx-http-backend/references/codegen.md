# OpenAPI-first Code Generation and Runtime Handoff

Follow this path when OpenAPI owns the HTTP contract or the repository already contains `oapi-ginx.yaml`.

Use `oapi-ginx` to generate Go request/response types, `ServerInterface`, `RegisterRoutes`, optional client code, and optional embedded specs. Do not treat those artifacts as the completed server: the generator does not know or generate application business logic. Always continue by implementing the generated interface in handwritten code and running that implementation through the ginx runtime described in [runtime.md](runtime.md).

## Recommended end-to-end workflow

1. Design or update the OpenAPI spec as the HTTP contract.
2. Configure `oapi-ginx.yaml` and run the repository's pinned generator command.
3. Inspect the generated Go contract and scaffolding: types, validation tags, `ServerInterface`, routes, statuses, and client signatures.
4. Read [runtime.md](runtime.md) and implement every required API method in non-generated business code.
5. Configure a dedicated `ginx.Engine`, pass the concrete service to generated `RegisterRoutes`, and start the Gin server.
6. Compile generated clients and server implementations, then test the complete HTTP behavior.

Keep ownership explicit:

| Artifact | Responsibility | Editing rule |
| --- | --- | --- |
| OpenAPI spec | HTTP operations, schemas, validation, and response contracts | Edit to change the API contract |
| `oapi-ginx.yaml` | Package, outputs, filters, names, and generation options | Edit to change generation |
| Generated `*.gen.go` | Go protocol types and wiring scaffolding | Regenerate; never hand-edit |
| `ServerInterface` implementation | Application and domain business logic | Write and maintain by hand |
| Gin bootstrap and `ginx.Engine` | Runtime policy, middleware, and route activation | Write and maintain by hand |

## Configuration

Prefer a checked-in multi-file configuration:

```yaml
package: api
spec: ./openapi.yaml
output:
  types: types.gen.go
  server: server.gen.go
  # client: client.gen.go
  # spec: spec.gen.go
generate_directive: "go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml"
output_options:
  generate_server: true
  generate_client: false
  skip_fmt: false
  unwrap_envelope: true
```

Use `output: api.gen.go` for single-file output. In multi-file mode, generate only files whose paths are present. Setting `output.client` enables client generation by default; override it explicitly with `output_options.generate_client` when needed. Use `output_options.generate_server`; the top-level `generate_server` key is deprecated.

Use `server_name` when multiple specs generate into one Go package. For example, `server_name: pet_store` yields names such as `PetStoreServerInterface`, `RegisterPetStoreRoutes`, and `NewPetStoreClient`.

Use `include_tags` and `exclude_tags` to select operations. Use `type_mapping` or `type_mapping_ext` only when the target package can consistently own the mapped Go type and import.

## Generate reproducibly

Initialize a config when none exists:

```bash
oapi-ginx -init > oapi-ginx.yaml
```

Prefer the command checked into the repository. Common forms are:

```bash
go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml
oapi-ginx -c oapi-ginx.yaml
go generate ./...
```

Keep the generator version pinned through `go.mod`, a versioned tool installation, or the repository's tool bootstrap. Do not silently switch a repository from its pinned invocation to `@latest`.

Available direct CLI forms include:

```bash
oapi-ginx spec.yaml
oapi-ginx -o api.gen.go spec.yaml
oapi-ginx -p api -o api.gen.go spec.yaml
```

CLI inputs override matching config inputs. A positional spec overrides `spec`; `-p` overrides `package`; `-o` sets the single-file path. If the config already contains an `output` map, generation remains in multi-file mode and uses the paths declared in that map.

## Implement the generated server

Treat this step as required for a working API. `RegisterRoutes` only connects generated route metadata to the concrete `ServerInterface` value supplied by the application; it does not provide the operation behavior itself.

For an ordinary operation, expect generated code shaped like:

```go
type ServerInterface interface {
	GetPet(ctx context.Context, req *GetPetReq) (*GetPetRsp, error)
}

func RegisterRoutes(r gin.IRoutes, s ServerInterface, opts ...ginx.RouteOption)
```

Implement the interface in a non-generated file and add a compile-time assertion:

```go
type PetService struct{}

var _ api.ServerInterface = (*PetService)(nil)

func (s *PetService) GetPet(
	ctx context.Context,
	req *api.GetPetReq,
) (*api.GetPetRsp, error) {
	pet, err := loadPet(ctx, req.PetID)
	if err != nil {
		return nil, ginx.Error(1002, "pet not found").Status(http.StatusNotFound)
	}
	return pet, nil
}
```

Register through a dedicated Engine:

```go
r := gin.Default()
engine := ginx.New(
	ginx.WithStrictJSONBody(true),
	ginx.WithExposeInternalError(false),
	ginx.WithInternalErrorMessage("internal error"),
)
api.RegisterRoutes(engine.Wrap(r.Group("/api/v1")), &PetService{})
```

Use generated request and response types directly at this HTTP boundary. Put reusable business rules in application/domain services called by the interface methods rather than copying protocol types or embedding business behavior in generated files.

Generated non-200 success route options are part of the OpenAPI contract. Do not try to override them through shared `RegisterRoutes(..., opts...)` arguments.

## Model the request contract

Give operations stable `operationId` values because they drive generated method and type names. Expect one `{OperationName}Req` to merge parameters and the request body:

| OpenAPI input | Generated tag or type |
| --- | --- |
| Path parameter | `uri:"name"`, always required |
| Query parameter | `form:"name"` |
| Header parameter | `header:"name"` |
| Cookie parameter | `cookie:"name"` |
| JSON `$ref` body | Alias to the referenced type when it is the only input; otherwise embedded in the combined `Req` |
| Inline JSON object body | JSON-tagged fields flattened into `Req` |
| Form or multipart body | `form:"name"` fields |
| Multipart binary field | `*multipart.FileHeader` or slice on the server |

Expect OpenAPI defaults to become `default` tags and supported schema constraints to become Gin `binding` rules. Optional scalar inputs generally become pointers so generated clients can omit `nil` values.

Use `x-binding: "..."` on a schema or property only for validator rules that the standard OpenAPI constraints cannot express.

The ordinary HTTP method set is GET, HEAD, POST, PUT, PATCH, DELETE, and OPTIONS. Treat TRACE as unsupported by generation.

## Model the response contract

Describe the business payload in a JSON success response. ginx normally adds the wire envelope `{code,msg,data}`. The generator defaults `unwrap_envelope` to true and recognizes an accidentally specified exact envelope, including supported `allOf` forms, to avoid double wrapping. Set it to false only when the three-field structure is itself the business payload.

For simple operations, expect the generator to select the response marked `x-ginx-primary-response: true`, otherwise the smallest 2xx response. Incompatible multiple 2xx JSON schemas fail generation instead of being guessed. Expect 204 or a successful response without content to use `struct{}` and write no body.

Use response media types or `x-ginx-response` to express non-JSON behavior:

| Contract | Generated response |
| --- | --- |
| JSON object, array, or `$ref` | Operation response type |
| `application/octet-stream` | `ginx.FileRsp` |
| `text/plain` | `ginx.StringRsp` |
| `text/event-stream` | SSE handler |
| `application/jsonl` or `application/x-ndjson` | JSON Lines handler |
| Redirect-only success | `ginx.RedirectRsp` |

Set `x-ginx-response` to `file`, `string`, `data`, or `redirect` when media type alone is ambiguous.

For files, declare 200 and optionally a compatible 206; let `http.ServeFile` choose 206 for valid Range requests. Keep SSE and JSON Lines successful responses at 200.

## Multiple successful responses

Use the simple primary-response model whenever one response type represents the operation. When business logic must dynamically choose among multiple 2xx/3xx JSON or no-body responses, set:

```yaml
x-ginx-response-mode: variants
```

Return the generated discriminated container through its constructors and use its `StatusCode` or `AsXXX` getters in clients/tests. Do not combine variants with `x-ginx-primary-response`, streaming, typed response headers, wildcard/default responses, or unsupported media types.

## Streaming operations

Mark SSE with `x-ginx-sse: true` or a `text/event-stream` successful response. Implement:

```go
ListEvents(ctx context.Context, req *ListEventsReq, send ginx.Sender) error
```

Mark JSON Lines with `x-ginx-jsonl: true`, `application/jsonl`, or `application/x-ndjson`. Implement:

```go
TailLogs(ctx context.Context, req *TailLogsReq, send ginx.JSONLinesSender) error
```

Generated SSE clients return `*ginx.SSEStream`; generated JSON Lines clients return `*ginx.JSONLinesStream`. Consume with `Recv()` and always call `Close()`.

## Generated client behavior

Enable a resty client with `output.client` and `output_options.generate_client`. Construct it with `NewClient` or the server-name-prefixed equivalent and pass options that configure the underlying resty client.

Expect generated clients to:

- Reuse server request types and omit nil pointer inputs.
- Validate declared success or redirect statuses before decoding.
- Return `*ginx.UnexpectedStatusError` for undeclared statuses below 400.
- Return `*ginx.ErrWrap` for 4xx/5xx responses.
- Avoid following redirects by default when the spec contains redirect operations, so callers can observe the declared 3xx response.
- Return only `error` for simple HEAD/204 operations without a response body.

Do not enable generated client support for multipart file-upload operations; the generator currently fails explicitly because that client path is unsupported.

## Compatibility and supported scope

Treat regeneration as an API change review. Pay special attention to 201/202/204 wire statuses, HEAD/204 client signatures, expected-status validation, redirect following, response variants, and generated names.

Use OpenAPI 3.0 or 3.1 for the broadest supported path. The generator supports selected OpenAPI 3.2 behavior, including preserved JSON Lines `itemSchema` metadata and flattened `in: querystring`, but not full OpenAPI 3.2. It does not provide Swagger UI, authentication, DI, ORM, tracing setup, or full union/oneOf modeling.

## Verification checklist

- Regenerate from a clean understanding of the spec/config and inspect every generated diff.
- Confirm that every generated `ServerInterface` method has a handwritten implementation; successful generation alone is not completion.
- Compile all handwritten `ServerInterface` implementations and client call sites.
- Test request binding and validation at the HTTP boundary, not only service methods.
- Assert exact response statuses, envelopes, empty-body behavior, redirects, files, and stream framing.
- Run `gofmt` on handwritten Go files and `go test ./... -count=1`.
