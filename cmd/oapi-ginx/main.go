package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chendefine/ginx/cmd/oapi-ginx/codegen"
)

func main() {
	var (
		output     string
		pkgName    string
		configPath string
	)
	flag.StringVar(&output, "o", "", "output file path (default: stdout)")
	flag.StringVar(&output, "output", "", "output file path (default: stdout)")
	flag.StringVar(&pkgName, "p", "", "Go package name (default: derived from output dir or \"api\")")
	flag.StringVar(&pkgName, "package", "", "Go package name (default: derived from output dir or \"api\")")
	flag.StringVar(&configPath, "c", "", "config file path (yaml)")
	flag.StringVar(&configPath, "config", "", "config file path (yaml)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: oapi-ginx [flags] <openapi-spec-file>\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

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
		cfg.OutputPath = output
	}

	if cfg.PackageName == "" && cfg.OutputPath != "" {
		cfg.PackageName = filepath.Base(filepath.Dir(cfg.OutputPath))
	}
	if cfg.PackageName == "" || cfg.PackageName == "." {
		cfg.PackageName = "api"
	}

	code, err := codegen.Generate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if cfg.OutputPath == "" {
		os.Stdout.Write(code)
		return
	}

	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(cfg.OutputPath, code, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing file: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "generated: %s\n", cfg.OutputPath)
}
