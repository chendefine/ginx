# Codegen E2E Tests

Run the full codegen E2E flow with:

```sh
scripts/test-codegen-e2e.sh
```

The script builds the current `cmd/oapi-ginx` binary, removes stale
`internal/codegen/e2etest/code/*/*.gen.go` files, regenerates each fixture from
its `oapi-ginx.yaml`, and then runs:

```sh
go test ./internal/codegen/... -count=1
```

Plain `go test ./internal/codegen/...` validates whatever generated fixture code
is present locally. Use the script when a change touches code generation
behavior, fixture specs, or fixture output options.
