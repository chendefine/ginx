package codegen

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressSpecRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specPath := filepath.Join(dir, "openapi.yaml")
	spec := []byte("openapi: 3.0.3\ninfo:\n  title: Embedded\n  version: 1.0.0\npaths: {}\n")
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	encoded, err := CompressSpec(specPath)
	if err != nil {
		t.Fatalf("CompressSpec: %v", err)
	}

	zipped, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode compressed spec: %v", err)
	}
	reader := flate.NewReader(bytes.NewReader(zipped))
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("decompress spec: %v", err)
	}
	if !bytes.Equal(got, spec) {
		t.Fatalf("round trip mismatch:\ngot:\n%s\nwant:\n%s", got, spec)
	}
}

func TestGenerateMulti_SpecOutput(t *testing.T) {
	t.Parallel()

	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath("basic_types.yaml"),
		Output: OutputConfig{
			Types: "types.gen.go",
			Spec:  "spec.gen.go",
		},
		OutputOptions: OutputOptions{SkipFmt: true},
	}

	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	specCode := string(result.Spec)
	for _, want := range []string{
		"package api",
		`"compress/flate"`,
		"const swaggerSpecBase64 = ",
		"func GetSwaggerSpec() ([]byte, error)",
	} {
		if !strings.Contains(specCode, want) {
			t.Fatalf("spec output missing %q:\n%s", want, specCode)
		}
	}
	if len(result.Types) == 0 {
		t.Fatal("types output should still be generated")
	}
}

func TestGenerateCompatibilityWrapper(t *testing.T) {
	t.Parallel()

	code, err := Generate(Config{
		PackageName: "api",
		SpecPath:    testdataPath("server_name.yaml"),
		OutputOptions: OutputOptions{
			SkipFmt: true,
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	text := string(code)
	for _, want := range []string{
		"package api",
		"type CreateOrderReq struct",
		"type ServerInterface interface",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Generate output missing %q:\n%s", want, text)
		}
	}
}
