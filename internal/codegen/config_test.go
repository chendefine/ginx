package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadConfig_MapOutputAndOptions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "oapi-ginx.yaml")
	data := []byte(`
package: petapi
spec: ./openapi.yaml
output:
  types: gen/types.go
  server: gen/server.go
  client: gen/client.go
  spec: gen/spec.go
server_name: pet_store
generate_directive: go run ./cmd/oapi-ginx -c oapi-ginx.yaml
include_tags: [public]
exclude_tags: [internal]
type_mapping:
  time.Time: string
output_options:
  skip_fmt: true
  generate_server: false
  generate_client: true
`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.PackageName != "petapi" || cfg.SpecPath != "./openapi.yaml" {
		t.Fatalf("unexpected package/spec: %#v", cfg)
	}
	if cfg.Output.Types != "gen/types.go" || cfg.Output.Server != "gen/server.go" || cfg.Output.Client != "gen/client.go" || cfg.Output.Spec != "gen/spec.go" {
		t.Fatalf("unexpected output config: %#v", cfg.Output)
	}
	if !cfg.Output.IsMultiFile() {
		t.Fatal("expected multifile output")
	}
	if got := cfg.GetServerName(); got != "PetStore" {
		t.Fatalf("GetServerName() = %q, want PetStore", got)
	}
	if cfg.ShouldGenerateServer() {
		t.Fatal("ShouldGenerateServer() = true, want false")
	}
	if !cfg.ShouldGenerateClient() {
		t.Fatal("ShouldGenerateClient() = false, want true")
	}
	if !cfg.OutputOptions.SkipFmt {
		t.Fatal("SkipFmt = false, want true")
	}
	if cfg.TypeMapping["time.Time"] != "string" {
		t.Fatalf("unexpected type mapping: %#v", cfg.TypeMapping)
	}
	if cfg.ShouldIncludeOperation([]string{"internal", "public"}) {
		t.Fatal("exclude_tags should win over include_tags")
	}
	if !cfg.ShouldIncludeOperation([]string{"public"}) {
		t.Fatal("public operation should be included")
	}
}

func TestOutputConfig_StringOutputCompatibility(t *testing.T) {
	t.Parallel()

	var cfg Config
	if err := yaml.Unmarshal([]byte("output: api.gen.go\n"), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if cfg.Output.Single != "api.gen.go" {
		t.Fatalf("Single = %q, want api.gen.go", cfg.Output.Single)
	}
	if got := cfg.GetOutputPath(); got != "api.gen.go" {
		t.Fatalf("GetOutputPath() = %q, want api.gen.go", got)
	}
	if cfg.Output.IsMultiFile() {
		t.Fatal("single-file output should not be multifile")
	}

	out, err := yaml.Marshal(Config{Output: OutputConfig{Single: "api.gen.go"}})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if !strings.Contains(string(out), "output: api.gen.go") {
		t.Fatalf("single output did not marshal as scalar:\n%s", out)
	}
}

func TestOutputConfig_MapMarshalAndInvalidOutput(t *testing.T) {
	t.Parallel()

	out, err := yaml.Marshal(Config{Output: OutputConfig{
		Types:  "types.gen.go",
		Server: "server.gen.go",
		Client: "client.gen.go",
		Spec:   "spec.gen.go",
	}})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	for _, want := range []string{
		"types: types.gen.go",
		"server: server.gen.go",
		"client: client.gen.go",
		"spec: spec.gen.go",
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("marshal output missing %q:\n%s", want, out)
		}
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte("output:\n  - bad\n"), &cfg); err == nil {
		t.Fatal("expected invalid output yaml to fail")
	}
}

func TestDeprecatedGenerateServerFallback(t *testing.T) {
	t.Parallel()

	falseVal := false
	cfg := Config{GenerateServer: &falseVal}
	if cfg.ShouldGenerateServer() {
		t.Fatal("deprecated generate_server=false should disable server generation")
	}

	trueVal := true
	cfg.OutputOptions.GenerateServer = &trueVal
	if !cfg.ShouldGenerateServer() {
		t.Fatal("output_options.generate_server should take precedence")
	}
}
