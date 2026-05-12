package codegen

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"fmt"
	"os"
)

func CompressSpec(specPath string) (string, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return "", fmt.Errorf("read spec for embedding: %w", err)
	}

	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return "", fmt.Errorf("create flate writer: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return "", fmt.Errorf("compress spec: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close flate writer: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
