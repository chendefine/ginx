package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func TestCLI_GeneratesFilesFromConfig(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "oapi-ginx.yaml")
	config := `package: clie2e
spec: ` + filepath.ToSlash(filepath.Join(root, "internal/codegen/e2etest/openapi-3.0/spec/basic_types.yaml")) + `
output:
  types: ` + filepath.ToSlash(filepath.Join(dir, "types.gen.go")) + `
  server: ` + filepath.ToSlash(filepath.Join(dir, "server.gen.go")) + `
  client: ` + filepath.ToSlash(filepath.Join(dir, "client.gen.go")) + `
  spec: ` + filepath.ToSlash(filepath.Join(dir, "spec.gen.go")) + `
generate_directive: go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/oapi-ginx", "-c", configPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run ./cmd/oapi-ginx failed: %v\n%s", err, out)
	}
	for _, name := range []string{"types.gen.go", "server.gen.go", "client.gen.go", "spec.gen.go"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected generated %s: %v\nCLI output:\n%s", name, err, out)
		}
		code := string(data)
		if !strings.Contains(code, "package clie2e") {
			t.Fatalf("%s missing package name:\n%s", name, code)
		}
		if !strings.Contains(code, "//go:generate go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml") {
			t.Fatalf("%s missing generate directive:\n%s", name, code)
		}
	}
}

func TestCLI_StdoutSingleFile(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	specPath := filepath.Join(root, "internal/codegen/e2etest/openapi-3.0/spec/server_name.yaml")
	cmd := exec.Command("go", "run", "./cmd/oapi-ginx", "-p", "stdoutapi", specPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run ./cmd/oapi-ginx stdout failed: %v\n%s", err, out)
	}
	code := string(out)
	for _, want := range []string{
		"package stdoutapi",
		"type CreateOrderReq struct",
		"type ListOrdersRsp = []any",
		"type ServerInterface interface",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("stdout output missing %q:\n%s", want, code)
		}
	}
}

func TestCLI_InitOutputsExampleConfig(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	cmd := exec.Command("go", "run", "./cmd/oapi-ginx", "-init")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run ./cmd/oapi-ginx -init failed: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{
		"# oapi-ginx configuration",
		"package: api",
		"output:",
		"generate_server: true",
		"generate_client: false",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("init output missing %q:\n%s", want, text)
		}
	}
}
