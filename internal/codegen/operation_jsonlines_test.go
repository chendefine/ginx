package codegen

import (
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func jsonLinesOperation(status int, content openapi3.Content) *openapi3.Operation {
	response := openapi3.NewResponse().
		WithDescription("test response").
		WithContent(content)
	return &openapi3.Operation{
		Responses: openapi3.NewResponses(openapi3.WithStatus(status, &openapi3.ResponseRef{Value: response})),
	}
}

func TestIsJSONLinesOperation(t *testing.T) {
	tests := []struct {
		name string
		op   *openapi3.Operation
		want bool
	}{
		{
			name: "jsonl",
			op: jsonLinesOperation(http.StatusOK, openapi3.Content{
				"application/jsonl": openapi3.NewMediaType(),
			}),
			want: true,
		},
		{
			name: "ndjson",
			op: jsonLinesOperation(http.StatusOK, openapi3.Content{
				"application/x-ndjson": openapi3.NewMediaType(),
			}),
			want: true,
		},
		{
			name: "media type parameters",
			op: jsonLinesOperation(http.StatusOK, openapi3.Content{
				"application/jsonl; charset=utf-8": openapi3.NewMediaType(),
			}),
			want: true,
		},
		{
			name: "case insensitive",
			op: jsonLinesOperation(http.StatusOK, openapi3.Content{
				"Application/X-NDJSON": openapi3.NewMediaType(),
			}),
			want: true,
		},
		{
			name: "explicit extension",
			op: &openapi3.Operation{
				Extensions: map[string]any{"x-ginx-jsonl": true},
				Responses:  openapi3.NewResponses(),
			},
			want: true,
		},
		{
			name: "false extension does not hide media type",
			op: func() *openapi3.Operation {
				op := jsonLinesOperation(http.StatusOK, openapi3.Content{
					"application/jsonl": openapi3.NewMediaType(),
				})
				op.Extensions = map[string]any{"x-ginx-jsonl": false}
				return op
			}(),
			want: true,
		},
		{
			name: "json is not streaming",
			op: jsonLinesOperation(http.StatusOK, openapi3.Content{
				"application/json": openapi3.NewMediaType(),
			}),
			want: false,
		},
		{
			name: "application wildcard is not json lines",
			op: jsonLinesOperation(http.StatusOK, openapi3.Content{
				"application/*": openapi3.NewMediaType(),
			}),
			want: false,
		},
		{
			name: "global wildcard is not json lines",
			op: jsonLinesOperation(http.StatusOK, openapi3.Content{
				"*/*": openapi3.NewMediaType(),
			}),
			want: false,
		},
		{
			name: "nil media type",
			op: jsonLinesOperation(http.StatusOK, openapi3.Content{
				"application/jsonl": nil,
			}),
			want: false,
		},
		{
			name: "error response is not a success stream",
			op: jsonLinesOperation(http.StatusBadRequest, openapi3.Content{
				"application/jsonl": openapi3.NewMediaType(),
			}),
			want: false,
		},
		{
			name: "missing responses",
			op:   &openapi3.Operation{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isJSONLinesOperation(tt.op); got != tt.want {
				t.Fatalf("isJSONLinesOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsJSONLinesContentType(t *testing.T) {
	tests := []struct {
		contentType string
		want        bool
	}{
		{"application/jsonl", true},
		{"application/x-ndjson", true},
		{"application/jsonl; charset=utf-8", true},
		{" Application/X-NDJSON ; charset=UTF-8 ", true},
		{"application/json", false},
		{"application/json-seq", false},
		{"application/*", false},
		{"*/*", false},
		{"not a media type", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			if got := isJSONLinesContentType(tt.contentType); got != tt.want {
				t.Fatalf("isJSONLinesContentType(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}
