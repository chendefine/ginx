package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chendefine/ginx/internal/codegen"
)

const initConfig = `# oapi-ginx configuration
package: api
spec: ./openapi.yaml

# Single file output:
# output: api.gen.go

# Multi-file output:
output:
  types: types.gen.go
  server: server.gen.go
  # client: client.gen.go  # uncomment to generate HTTP client SDK
  # spec: spec.gen.go  # uncomment to embed spec

# Server interface name prefix (e.g. "pet_store" -> PetStoreServerInterface / RegisterPetStoreRoutes)
# Useful when generating multiple APIs in the same package
# server_name: ""

# go:generate directive (added to generated file headers)
# generate_directive: "go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml"

# Filter operations by OpenAPI tags
# include_tags: []
# exclude_tags: []

# Custom type mappings (OpenAPI Go type -> replacement)
# type_mapping:
#   time.Time: string

# Output options
# output_options:
#   skip_fmt: false
#   generate_server: true
#   generate_client: false
`

func main() {
	var (
		output     string
		pkgName    string
		configPath string
		initFlag   bool
	)
	flag.StringVar(&output, "o", "", "output file path (default: stdout)")
	flag.StringVar(&output, "output", "", "output file path (default: stdout)")
	flag.StringVar(&pkgName, "p", "", "Go package name (default: derived from output dir or \"api\")")
	flag.StringVar(&pkgName, "package", "", "Go package name (default: derived from output dir or \"api\")")
	flag.StringVar(&configPath, "c", "", "config file path (yaml)")
	flag.StringVar(&configPath, "config", "", "config file path (yaml)")
	flag.BoolVar(&initFlag, "init", false, "output example config to stdout")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: oapi-ginx [flags] [openapi-spec-file]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  oapi-ginx spec.yaml                    # generate to stdout\n")
		fmt.Fprintf(os.Stderr, "  oapi-ginx -o api.gen.go spec.yaml      # generate to file\n")
		fmt.Fprintf(os.Stderr, "  oapi-ginx -c oapi-ginx.yaml            # use config file\n")
		fmt.Fprintf(os.Stderr, "  oapi-ginx -init > oapi-ginx.yaml       # create config template\n")
	}
	flag.Parse()

	if initFlag {
		fmt.Print(initConfig)
		return
	}

	var cfg codegen.Config

	if configPath != "" {
		loaded, err := codegen.LoadConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		cfg = *loaded
	}

	if flag.NArg() >= 1 {
		cfg.SpecPath = flag.Arg(0)
	}
	if cfg.SpecPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	if pkgName != "" {
		cfg.PackageName = pkgName
	}
	if output != "" {
		cfg.Output.Single = output
	}

	if cfg.PackageName == "" {
		if p := cfg.GetOutputPath(); p != "" {
			cfg.PackageName = filepath.Base(filepath.Dir(p))
		} else if cfg.Output.Types != "" {
			cfg.PackageName = filepath.Base(filepath.Dir(cfg.Output.Types))
		}
	}
	if cfg.PackageName == "" || cfg.PackageName == "." {
		cfg.PackageName = "api"
	}

	if !cfg.Output.IsMultiFile() && cfg.Output.Single == "" {
		result, err := codegen.GenerateMulti(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Stdout.Write(result.Types)
		return
	}

	result, err := codegen.GenerateMulti(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if cfg.Output.IsMultiFile() {
		if cfg.Output.Types != "" {
			if err := writeFile(cfg.Output.Types, result.Types); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
		if cfg.Output.Server != "" && result.Server != nil {
			if err := writeFile(cfg.Output.Server, result.Server); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
		if cfg.Output.Client != "" && result.Client != nil {
			if err := writeFile(cfg.Output.Client, result.Client); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
		if cfg.Output.Spec != "" && result.Spec != nil {
			if err := writeFile(cfg.Output.Spec, result.Spec); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
	} else {
		if err := writeFile(cfg.Output.Single, result.Types); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Fprintf(os.Stderr, "generated: %s\n", path)
	return nil
}
