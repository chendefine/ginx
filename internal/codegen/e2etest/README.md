# Codegen E2E Tests

Run the full codegen E2E flow with:

```sh
scripts/test-codegen-e2e.sh
```

The script builds the current `cmd/oapi-ginx` binary, regenerates every
`internal/codegen/e2etest/code/*/*.gen.go` file from its `oapi-ginx.yaml`, and
then runs:

```sh
go test ./internal/codegen/... -count=1
```

Plain `go test ./internal/codegen/...` validates the already-generated fixture
code. Use the script when a change touches code generation behavior and the
fixtures need to be regenerated from the real CLI.

