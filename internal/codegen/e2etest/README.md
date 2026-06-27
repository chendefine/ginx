# Codegen E2E Tests

End-to-end codegen tests are organized by OpenAPI version under
`internal/codegen/e2etest/openapi-3.<x>/`:

- `openapi-3.0/` — baseline suite (3.0.3)
- `openapi-3.1/` — full 3.0 parity under 3.1.0 plus 3.1-specific features
- `openapi-3.2/` — 3.2.0 features (SSE, JSON Lines / NDJSON, querystring, …)

Each versioned directory has a `spec/` tree of hand-authored OpenAPI documents
and a `code/` tree of fixture packages.

There are two test tiers:

- **Tier-A** — `internal/codegen/codegen_e2e_test.go` runs the generator
  in-process and asserts on the generated source via substring checks. Specs
  are located with `specPath("<version>", "<name>.yaml")` (the legacy
  `testdataPath` helper resolves against `openapi-3.0`).
- **Tier-B** — each `code/<fixture>/` package is a real Go package with
  generated `*.gen.go`, a hand-written `impl.go`, and an `e2e_test.go` that
  round-trips requests through `httptest`.

Run the full flow with:

```sh
scripts/test-codegen-e2e.sh
```

The script builds `cmd/oapi-ginx`, removes stale
`internal/codegen/e2etest/openapi-3.*/code/*/*.gen.go`, regenerates each
fixture from its `oapi-ginx.yaml`, and then runs:

```sh
go test ./internal/codegen/... -count=1
```

Plain `go test ./internal/codegen/...` validates whatever generated fixture
code is present locally. Use the script when a change touches code generation
behavior, fixture specs, or fixture output options.
