package codegen

import (
	"go/format"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// specPath resolves a fixture spec under a versioned e2e directory, e.g.
// specPath("openapi-3.0", "basic_types.yaml").
func specPath(version, name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "e2etest", version, "spec", name)
}

// testdataPath keeps the legacy single-argument callers pointed at the 3.0
// suite (the flat e2etest/spec/ layout was removed during the per-version
// restructure).
func testdataPath(name string) string {
	return specPath("openapi-3.0", name)
}

func generateSingleFile(t *testing.T, specFile string, opts ...func(*Config)) string {
	t.Helper()
	return generateSingleFileAt(t, testdataPath(specFile), opts...)
}

// generateSingleFileAt is the version-aware core: it takes an already-resolved
// spec path (use specPath(version, name) to build one for a non-3.0 suite).
func generateSingleFileAt(t *testing.T, resolvedSpecPath string, opts ...func(*Config)) string {
	t.Helper()
	cfg := Config{
		PackageName: "api",
		SpecPath:    resolvedSpecPath,
		OutputOptions: OutputOptions{
			SkipFmt: true,
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	return string(result.Types)
}

// generateSingleFileV generates from a versioned suite (e.g. "openapi-3.1").
func generateSingleFileV(t *testing.T, version, specFile string, opts ...func(*Config)) string {
	t.Helper()
	return generateSingleFileAt(t, specPath(version, specFile), opts...)
}

func generateMultiFile(t *testing.T, specFile string, opts ...func(*Config)) *GenerateResult {
	t.Helper()
	return generateMultiFileAt(t, testdataPath(specFile), opts...)
}

func generateMultiFileAt(t *testing.T, resolvedSpecPath string, opts ...func(*Config)) *GenerateResult {
	t.Helper()
	cfg := Config{
		PackageName: "api",
		SpecPath:    resolvedSpecPath,
		Output: OutputConfig{
			Types:  "types.gen.go",
			Server: "server.gen.go",
			Client: "client.gen.go",
		},
		OutputOptions: OutputOptions{
			SkipFmt: true,
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	return result
}

func generateMultiFileV(t *testing.T, version, specFile string, opts ...func(*Config)) *GenerateResult {
	t.Helper()
	return generateMultiFileAt(t, specPath(version, specFile), opts...)
}

func assertContains(t *testing.T, code, substr string) {
	t.Helper()
	if !strings.Contains(code, substr) {
		t.Errorf("expected code to contain %q, but it doesn't.\nCode:\n%s", substr, code)
	}
}

func assertNotContains(t *testing.T, code, substr string) {
	t.Helper()
	if strings.Contains(code, substr) {
		t.Errorf("expected code NOT to contain %q, but it does", substr)
	}
}

func assertValidGo(t *testing.T, code string) {
	t.Helper()
	_, err := format.Source([]byte(code))
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\nCode:\n%s", err, code)
	}
}

// ============================================================
// Module 1: Basic Types
// ============================================================

func TestE2E_BasicTypes_Integer(t *testing.T) {
	code := generateSingleFile(t, "basic_types.yaml")

	assertContains(t, code, "ID int64")
	assertContains(t, code, "Count int32")
	assertContains(t, code, "PlainInt *int")
}

func TestE2E_BasicTypes_Number(t *testing.T) {
	code := generateSingleFile(t, "basic_types.yaml")

	assertContains(t, code, "Price float64")
	assertContains(t, code, "Rate *float32")
	assertContains(t, code, "PlainNumber *float64")
}

func TestE2E_BasicTypes_String(t *testing.T) {
	code := generateSingleFile(t, "basic_types.yaml")

	assertContains(t, code, "Name string")
	assertContains(t, code, "CreatedAt *time.Time")
	assertContains(t, code, "DateOnly *string")
	assertContains(t, code, "RawData []byte")
	assertContains(t, code, "BinaryData []byte")
	assertContains(t, code, "UserID *string")
	assertContains(t, code, "Website *string")
	assertContains(t, code, "Email *string")
	assertContains(t, code, "Host *string")
	assertContains(t, code, "IPV4 *string")
	assertContains(t, code, "IPV6 *string")
}

func TestE2E_BasicTypes_Boolean(t *testing.T) {
	code := generateSingleFile(t, "basic_types.yaml")

	assertContains(t, code, "Active bool")
	assertContains(t, code, "Deleted *bool")
}

func TestE2E_BasicTypes_TimeImport(t *testing.T) {
	code := generateSingleFile(t, "basic_types.yaml")
	assertContains(t, code, `"time"`)
}

// ============================================================
// Module 2: Complex Types
// ============================================================

func TestE2E_ComplexTypes_Enum(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")

	assertContains(t, code, "type PetStatus string")
	assertContains(t, code, `PetStatusAvailable PetStatus = "available"`)
	assertContains(t, code, `PetStatusPending PetStatus = "pending"`)
	assertContains(t, code, `PetStatusSold PetStatus = "sold"`)

	assertContains(t, code, "type Priority int")
	assertContains(t, code, "Priority1 Priority = 1")
	assertContains(t, code, "Priority2 Priority = 2")
	assertContains(t, code, "Priority3 Priority = 3")
}

func TestE2E_ComplexTypes_EnumSpecialChars(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")

	assertContains(t, code, "type SpecialEnum string")
	assertContains(t, code, `SpecialEnumFooBar SpecialEnum = "foo@bar"`)
	assertContains(t, code, `SpecialEnumAB SpecialEnum = "a+b"`)
	assertContains(t, code, `SpecialEnumHelloWorld SpecialEnum = "hello#world"`)
	assertContains(t, code, `SpecialEnumNormal SpecialEnum = "normal"`)

	// 验证生成的代码可以通过 go/format 解析（即合法 Go 代码）
	_, err := format.Source([]byte(code))
	if err != nil {
		t.Fatalf("generated code with special char enums is not valid Go: %v", err)
	}
}

func TestE2E_ComplexTypes_Object(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")

	assertContains(t, code, "type Pet struct")
	assertContains(t, code, "ID int64")
	assertContains(t, code, "Name string")
	assertContains(t, code, "Status PetStatus")
	assertContains(t, code, "Tags []string")
}

func TestE2E_ComplexTypes_Array(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")
	assertContains(t, code, "type PetList = []Pet")
}

func TestE2E_ComplexTypes_Map(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")
	assertContains(t, code, "type Metadata = map[string]string")
	assertContains(t, code, "type DynamicMap = map[string]any")
}

func TestE2E_ComplexTypes_AllOf(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")

	assertContains(t, code, "// ExtendedPet extended pet object")
	assertContains(t, code, "type ExtendedPet struct")
	assertContains(t, code, "\tPet")
	assertContains(t, code, `Breed string `+"`"+`json:"breed" binding:"required"`+"`")
	assertContains(t, code, "Weight *float32")
}

func TestE2E_ComplexTypes_AllOfAdditionalProperties(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")
	assertContains(t, code, "type LabelMap = map[string]string")
}

func TestE2E_ComplexTypes_AllOfSingleRef(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")
	assertContains(t, code, "type CombinedRef = Pet")
}

func TestE2E_ComplexTypes_OneOf(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")
	assertContains(t, code, "type FlexibleData = json.RawMessage")
}

func TestE2E_ComplexTypes_AnyOf(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")
	assertContains(t, code, "type AnyData = json.RawMessage")
}

func TestE2E_ComplexTypes_NestedObject(t *testing.T) {
	code := generateSingleFile(t, "complex_types.yaml")

	assertContains(t, code, "type NestedObject struct")
	assertContains(t, code, "Address NestedObjectAddress")
	assertContains(t, code, "type NestedObjectAddress struct")
	assertContains(t, code, "City string")
}

// ============================================================
// Module 3: Request Parameters
// ============================================================

func TestE2E_RequestParams_PathParam(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type GetUserReq struct")
	assertContains(t, code, `UserID int64 `+"`"+`uri:"user_id" binding:"required"`+"`")
}

func TestE2E_RequestParams_QueryParam(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type GetUserReq struct")
	assertContains(t, code, `Fields *string `+"`"+`form:"fields"`+"`")
}

func TestE2E_RequestParams_HeaderParam(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, `XRequestID *string `+"`"+`header:"X-Request-ID"`+"`")
}

func TestE2E_RequestParams_CookieParam(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, `Sid string `+"`"+`cookie:"sid" binding:"required"`+"`")
}

func TestE2E_RequestParams_RequiredHeader(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type UpdateUserReq struct")
	assertContains(t, code, `XAuthToken string `+"`"+`header:"X-Auth-Token" binding:"required"`+"`")
}

func TestE2E_RequestParams_InlineBody(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type UpdateUserReq struct")
	assertContains(t, code, `Name string `+"`"+`json:"name" binding:"required"`+"`")
	assertContains(t, code, `Email *string `+"`"+`json:"email" binding:"email"`+"`")
}

func TestE2E_RequestParams_RefBody(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type CreateUserReq struct")
	assertContains(t, code, "\tCreateUserInput")
	assertContains(t, code, `XIdempotencyKey string `+"`"+`header:"X-Idempotency-Key" binding:"required"`+"`")
}

func TestE2E_RequestParams_RefOnlyBodyUsesAlias(t *testing.T) {
	code := generateSingleFile(t, "server_interface.yaml")

	assertContains(t, code, "type CreatePetReq = Pet")
	assertNotContains(t, code, "type CreatePetReq struct")
}

func TestE2E_RequestParams_DefaultValues(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type SearchReq struct")
	assertContains(t, code, `Q string `+"`"+`form:"q" binding:"required"`+"`")
	assertContains(t, code, `Page *int `+"`"+`form:"page" default:"1"`+"`")
	assertContains(t, code, `PageSize *int `+"`"+`form:"page_size" default:"20"`+"`")
}

func TestE2E_RequestParams_MultipartFile(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type UploadFileReq struct")
	assertContains(t, code, "*multipart.FileHeader")
	assertContains(t, code, `form:"file" binding:"required"`)
	assertContains(t, code, `Description string`)
}

func TestE2E_RequestParams_MultipartFileArray(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type UploadBatchReq struct")
	assertContains(t, code, "[]*multipart.FileHeader")
}

func TestE2E_RequestParams_FormURLEncodedBody(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type LoginReq struct")
	assertContains(t, code, `Username string `+"`"+`form:"username" binding:"required"`+"`")
	assertContains(t, code, `Password string `+"`"+`form:"password" binding:"required"`+"`")
	assertContains(t, code, `Remember *bool `+"`"+`form:"remember"`+"`")
}

func TestE2E_RequestParams_FormURLEncodedBinaryIsNotFile(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "openapi.yaml")
	spec := `openapi: "3.0.3"
info:
  title: Form Binary Test
  version: "1.0.0"
paths:
  /submit:
    post:
      operationId: submit
      requestBody:
        content:
          application/x-www-form-urlencoded:
            schema:
              type: object
              properties:
                proof:
                  type: string
                  format: binary
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	result, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      specPath,
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}

	code := string(result.Types)
	assertContains(t, code, `Proof []byte `+"`"+`form:"proof"`+"`")
	assertNotContains(t, code, `Proof *multipart.FileHeader `+"`"+`form:"proof"`+"`")
}

func TestE2E_RequestParams_JSONPreferredOverFormURLEncoded(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "openapi.yaml")
	spec := `openapi: "3.0.3"
info:
  title: JSON Preferred Test
  version: "1.0.0"
paths:
  /submit:
    post:
      operationId: submit
      requestBody:
        content:
          application/x-www-form-urlencoded:
            schema:
              type: object
              properties:
                form_name:
                  type: string
          application/json:
            schema:
              type: object
              properties:
                json_name:
                  type: string
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	result, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      specPath,
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}

	code := string(result.Types)
	assertContains(t, code, `JSONName *string `+"`"+`json:"json_name"`+"`")
	assertNotContains(t, code, `FormName *string `+"`"+`form:"form_name"`+"`")
}

func TestE2E_RequestParams_PathLevelParams(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type ListCommentsReq struct")
	assertContains(t, code, `ItemID string `+"`"+`uri:"item_id" binding:"required"`+"`")
	assertContains(t, code, `Limit *int `+"`"+`form:"limit"`+"`")
}

func TestE2E_RequestParams_ScalarBody(t *testing.T) {
	code := generateSingleFile(t, "request_params.yaml")

	assertContains(t, code, "type PostScalarReq struct")
	assertContains(t, code, `Body string `+"`"+`json:"body" binding:"required"`+"`")
}

// ============================================================
// Module 4: Response Types
// ============================================================

func TestE2E_ResponseTypes_JSON(t *testing.T) {
	code := generateSingleFile(t, "response_types.yaml")

	assertContains(t, code, "type GetJSONDataRsp struct")
	assertContains(t, code, "Message string")
}

func TestE2E_ResponseTypes_JSONRef(t *testing.T) {
	code := generateSingleFile(t, "response_types.yaml")
	assertContains(t, code, "type GetJSONRefRsp = User")
}

func TestE2E_ResponseTypes_PDF(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "DownloadPdf(ctx context.Context, req *DownloadPdfReq) (*ginx.FileRsp, error)")
}

func TestE2E_ResponseTypes_Image(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "DownloadImage(ctx context.Context, req *DownloadImageReq) (*ginx.FileRsp, error)")
}

func TestE2E_ResponseTypes_Audio(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "DownloadAudio(ctx context.Context, req *DownloadAudioReq) (*ginx.FileRsp, error)")
}

func TestE2E_ResponseTypes_Video(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "DownloadVideo(ctx context.Context, req *DownloadVideoReq) (*ginx.FileRsp, error)")
}

func TestE2E_ResponseTypes_Binary(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "DownloadBinary(ctx context.Context, req *DownloadBinaryReq) (*ginx.FileRsp, error)")
}

func TestE2E_ResponseTypes_Zip(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "DownloadZip(ctx context.Context, req *DownloadZipReq) (*ginx.FileRsp, error)")
}

func TestE2E_ResponseTypes_Text(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "GetTextContent(ctx context.Context, req *GetTextContentReq) (*ginx.StringRsp, error)")
}

func TestE2E_ResponseTypes_CSV(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "ExportCsv(ctx context.Context, req *ExportCsvReq) (*ginx.StringRsp, error)")
}

func TestE2E_ResponseTypes_HTML(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "GetHTMLPage(ctx context.Context, req *GetHTMLPageReq) (*ginx.StringRsp, error)")
}

func TestE2E_ResponseTypes_Redirect(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	client := string(result.Client)
	assertContains(t, server, "RedirectToHome(ctx context.Context, req *RedirectToHomeReq) (*ginx.RedirectRsp, error)")
	assertContains(t, client, "c.SetRedirectPolicy(resty.RedirectNoPolicy())")
	assertContains(t, client, "ginx.ValidateResponseStatus(resp.StatusCode(), 301)")
}

func TestE2E_ResponseTypes_NoContent(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "DeleteItem(ctx context.Context, req *DeleteItemReq) (*struct{}, error)")
	assertContains(t, server, `ginx.DELETE(r, "/no-content", s.DeleteItem, append(append([]ginx.RouteOption(nil), opts...), ginx.SuccessStatus(204))...)`)
}

func TestE2E_ResponseTypes_FixedAndExpectedStatuses(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	client := string(result.Client)

	assertContains(t, server, `ginx.POST(r, "/accepted-job", s.CreateJob, append(append([]ginx.RouteOption(nil), opts...), ginx.SuccessStatus(202))...)`)
	assertContains(t, server, `ginx.POST(r, "/created-item", s.CreateItem, append(append([]ginx.RouteOption(nil), opts...), ginx.SuccessStatus(201))...)`)
	assertContains(t, client, "ginx.ValidateResponseStatus(resp.StatusCode(), 202)")
	assertContains(t, client, "ginx.ValidateResponseStatus(resp.StatusCode(), 201)")
	assertContains(t, client, "ginx.ValidateResponseStatus(resp.StatusCode(), 200, 206)")

	start := strings.Index(client, "func (c *Client) CreateJob(")
	if start < 0 {
		t.Fatal("CreateJob client method not generated")
	}
	method := client[start:]
	errorCheck := strings.Index(method, "if resp.StatusCode() >= 400")
	statusCheck := strings.Index(method, "ginx.ValidateResponseStatus")
	parseAfterStatus := -1
	if statusCheck >= 0 {
		if offset := strings.Index(method[statusCheck:], "ginx.ParseResponse"); offset >= 0 {
			parseAfterStatus = statusCheck + offset
		}
	}
	if errorCheck < 0 || statusCheck <= errorCheck || parseAfterStatus <= statusCheck {
		t.Fatalf("client contract order is wrong: error=%d status=%d success-parse=%d", errorCheck, statusCheck, parseAfterStatus)
	}
}

func TestE2E_ResponseTypes_FileContractValidation(t *testing.T) {
	tests := []struct {
		name      string
		responses string
		want      string
	}{
		{
			name: "206 requires 200",
			responses: `        "206":
          description: partial
          content:
            application/octet-stream:
              schema: {type: string, format: binary}`,
			want: "declares 206 without 200",
		},
		{
			name: "unsupported fixed status",
			responses: `        "202":
          description: accepted file
          content:
            application/octet-stream:
              schema: {type: string, format: binary}`,
			want: "does not support success status 202",
		},
		{
			name: "incompatible 200 and 206",
			responses: `        "200":
          description: complete
          content:
            application/pdf:
              schema: {type: string, format: binary}
        "206":
          description: partial
          content:
            application/octet-stream:
              schema: {type: string, format: binary}`,
			want: "responses 200 and 206 must use compatible content and schemas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "file.yaml")
			spec := `openapi: 3.0.3
info:
  title: file contract
  version: 1.0.0
paths:
  /download:
    get:
      operationId: download
      responses:
` + tt.responses + "\n"
			if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
				t.Fatalf("write spec: %v", err)
			}
			_, err := GenerateMulti(Config{PackageName: "api", SpecPath: path, OutputOptions: OutputOptions{SkipFmt: true}})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("GenerateMulti error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestE2E_ResponseTypes_JSONPriority(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "GetWithMultipleTypes(ctx context.Context, req *GetWithMultipleTypesReq) (*GetWithMultipleTypesRsp, error)")
}

func TestE2E_ResponseTypes_EmptyResponse(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "GetEmpty(ctx context.Context, req *GetEmptyReq) (*struct{}, error)")
}

func TestE2E_ResponseTypes_XGinxResponseOverride(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "response_override.yaml")
	spec := `openapi: "3.0.3"
info:
  title: Response Override
  version: "1.0.0"
paths:
  /bytes:
    get:
      operationId: getBytes
      responses:
        "200":
          description: bytes
          x-ginx-response: data
          content:
            application/octet-stream:
              schema:
                type: string
                format: binary
  /file:
    get:
      operationId: getFile
      x-ginx-response: file
      responses:
        "200":
          description: file
          content:
            application/json:
              schema:
                type: object
                properties:
                  ignored:
                    type: string
  /text:
    get:
      operationId: getText
      responses:
        "200":
          description: text
          x-ginx-response: string
          content:
            application/json:
              schema:
                type: object
                properties:
                  ignored:
                    type: string
  /redirect:
    get:
      operationId: getRedirect
      responses:
        "200":
          description: redirect
          x-ginx-response: redirect
          content:
            application/json:
              schema:
                type: object
                properties:
                  ignored:
                    type: string
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	result, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      specPath,
		Output:        OutputConfig{Types: "types.go", Server: "server.go", Client: "client.go"},
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	server := string(result.Server)
	client := string(result.Client)
	types := string(result.Types)
	assertContains(t, server, "GetBytes(ctx context.Context, req *GetBytesReq) (*ginx.DataRsp, error)")
	assertContains(t, server, "GetFile(ctx context.Context, req *GetFileReq) (*ginx.FileRsp, error)")
	assertContains(t, server, "GetText(ctx context.Context, req *GetTextReq) (*ginx.StringRsp, error)")
	assertContains(t, server, "GetRedirect(ctx context.Context, req *GetRedirectReq) (*ginx.RedirectRsp, error)")
	assertContains(t, client, "GetBytes(ctx context.Context, req *GetBytesReq) ([]byte, error)")
	assertNotContains(t, types, "GetFileRsp")
	assertNotContains(t, types, "GetTextRsp")
	assertNotContains(t, types, "GetRedirectRsp")
}

func TestE2E_ResponseTypes_InvalidXGinxResponseReturnsError(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "bad_response_override.yaml")
	spec := `openapi: "3.0.3"
info:
  title: Bad Response Override
  version: "1.0.0"
paths:
  /bad:
    get:
      operationId: bad
      responses:
        "200":
          description: bad
          x-ginx-response: stream
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	_, err := GenerateMulti(Config{PackageName: "api", SpecPath: specPath, OutputOptions: OutputOptions{SkipFmt: true}})
	if err == nil || !strings.Contains(err.Error(), `x-ginx-response="stream" is unsupported`) {
		t.Fatalf("GenerateMulti error = %v, want invalid x-ginx-response", err)
	}
}

// ============================================================
// Module 4b: Envelope Unwrap
// ============================================================

// When unwrap_envelope is enabled (the default), a JSON success response shaped
// exactly like the ginx envelope {code:integer, msg:string, data:any} is
// unwrapped: only the data sub-schema becomes the XxxRsp type, so ginx's
// runtime wrapper produces a single envelope on the wire instead of a double
// wrap. See internal/codegen/e2etest/openapi-3.0/spec/envelope_unwrap.yaml.

func TestE2E_Envelope_InlineEnvelopeWithRefData(t *testing.T) {
	code := generateSingleFile(t, "envelope_unwrap.yaml")
	// data is $ref: User -> XxxRsp is an alias, NOT a three-field struct.
	assertContains(t, code, "type GetUserRsp = User")
	assertNotContains(t, code, "type GetUserRsp struct")
}

func TestE2E_Envelope_InlineEnvelopeWithInlineData(t *testing.T) {
	code := generateSingleFile(t, "envelope_unwrap.yaml")
	// data is an inline object -> XxxRsp is the business struct.
	assertContains(t, code, "type GetProductRsp struct")
	assertContains(t, code, "Price float64")
}

func TestE2E_Envelope_RefEnvelope(t *testing.T) {
	code := generateSingleFile(t, "envelope_unwrap.yaml")
	// response is $ref: ApiResponse (the envelope); data is $ref: User -> alias.
	assertContains(t, code, "type GetWrappedRsp = User")
}

func TestE2E_Envelope_ComponentPreservedVerbatim(t *testing.T) {
	code := generateSingleFile(t, "envelope_unwrap.yaml")
	// The envelope component schema itself is still emitted verbatim (it is the
	// real wire contract for non-ginx clients / embedded spec).
	assertContains(t, code, "type APIResponse struct")
	assertContains(t, code, "Code *int")
	assertContains(t, code, "Data *User")
}

func TestE2E_Envelope_PrimitiveData(t *testing.T) {
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /token:
    get:
      operationId: getToken
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  code: {type: integer}
                  msg: {type: string}
                  data: {type: string}
`)
	// data is a primitive string -> XxxRsp is an alias to string.
	assertContains(t, code, "type GetTokenRsp = string")
}

func TestE2E_Envelope_NegativeStringCode(t *testing.T) {
	// code as string is NOT the ginx envelope (code must be integer).
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /sc:
    get:
      operationId: getStrCode
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  code: {type: string}
                  msg: {type: string}
                  data:
                    type: object
                    properties:
                      id: {type: integer}
`)
	assertContains(t, code, "type GetStrCodeRsp struct")
	assertContains(t, code, "Code *string")
}

func TestE2E_Envelope_NegativeFourFields(t *testing.T) {
	// Four properties is NOT exactly the three-field envelope.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /four:
    get:
      operationId: getFour
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  code: {type: integer}
                  msg: {type: string}
                  data: {type: string}
                  extra: {type: string}
`)
	assertContains(t, code, "type GetFourRsp struct")
	assertContains(t, code, "Extra *string")
}

func TestE2E_Envelope_NegativeMissingData(t *testing.T) {
	// Two properties (no data) is NOT the envelope.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /cm:
    get:
      operationId: getCodeMsg
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  code: {type: integer}
                  msg: {type: string}
`)
	assertContains(t, code, "type GetCodeMsgRsp struct")
	assertContains(t, code, "Code *int")
	assertContains(t, code, "Msg *string")
}

func TestE2E_Envelope_OptOutKeepsEnvelope(t *testing.T) {
	// Same shape as the positive case, but unwrap_envelope: false must keep the
	// response schema verbatim (three-field struct, not unwrapped).
	disable := func(c *Config) { v := false; c.OutputOptions.UnwrapEnvelope = &v }
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /optout:
    get:
      operationId: getOptOut
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  code: {type: integer}
                  msg: {type: string}
                  data:
                    $ref: "#/components/schemas/User"
components:
  schemas:
    User:
      type: object
      properties:
        id: {type: integer}
        name: {type: string}
`, disable)
	assertContains(t, code, "type GetOptOutRsp struct")
	assertContains(t, code, "Code *int")
	assertContains(t, code, "Data *User")
	assertNotContains(t, code, "type GetOptOutRsp = User")
}

// The reusable-envelope pattern composes a generic Envelope component with a
// specific data override via allOf. codegen must recognize this shape too and
// unwrap to the data sub-schema, instead of embedding the whole Envelope plus a
// duplicate data field (which the existing resolveAllOf path would produce).

func TestE2E_Envelope_AllOfRefData(t *testing.T) {
	// allOf: [{$ref: Envelope}, {properties:{data:{$ref: User}}}] -> alias to User.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /account:
    get:
      operationId: getAccount
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                allOf:
                  - $ref: "#/components/schemas/Envelope"
                  - properties:
                      data:
                        $ref: "#/components/schemas/User"
components:
  schemas:
    Envelope:
      type: object
      description: 统一响应包络
      properties:
        code: {type: integer}
        msg: {type: string}
        data: {description: 业务数据}
    User:
      type: object
      properties:
        id: {type: integer}
        name: {type: string}
`)
	assertContains(t, code, "type GetAccountRsp = User")
	assertNotContains(t, code, "type GetAccountRsp struct")
}

func TestE2E_Envelope_AllOfInlineData(t *testing.T) {
	// allOf with an inline data object -> XxxRsp is the business struct.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /inline:
    get:
      operationId: getInline
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                allOf:
                  - $ref: "#/components/schemas/Envelope"
                  - properties:
                      data:
                        type: object
                        properties:
                          sku: {type: string}
                          qty: {type: integer}
components:
  schemas:
    Envelope:
      type: object
      properties:
        code: {type: integer}
        msg: {type: string}
        data: {description: 业务数据}
`)
	assertContains(t, code, "type GetInlineRsp struct")
	assertContains(t, code, "Sku *string")
	assertContains(t, code, "Qty *int")
}

func TestE2E_Envelope_AllOfViaComponentRef(t *testing.T) {
	// The response is a $ref to a component that is itself allOf-composed.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /specialized:
    get:
      operationId: getSpecialized
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Specialized"
components:
  schemas:
    Envelope:
      type: object
      properties:
        code: {type: integer}
        msg: {type: string}
        data: {description: 业务数据}
    User:
      type: object
      properties:
        id: {type: integer}
        name: {type: string}
    Specialized:
      allOf:
        - $ref: "#/components/schemas/Envelope"
        - properties:
            data:
              $ref: "#/components/schemas/User"
`)
	assertContains(t, code, "type GetSpecializedRsp = User")
	assertNotContains(t, code, "type GetSpecializedRsp struct")
}

func TestE2E_Envelope_AllOfReversedOrder(t *testing.T) {
	// Override member listed BEFORE the Envelope ref. The concrete data must
	// still win regardless of member order (a generic data must never clobber a
	// specific one), so this unwraps the same as the forward-order case.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /rev:
    get:
      operationId: getRev
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                allOf:
                  - properties:
                      data:
                        $ref: "#/components/schemas/User"
                  - $ref: "#/components/schemas/Envelope"
components:
  schemas:
    Envelope:
      type: object
      properties:
        code: {type: integer}
        msg: {type: string}
        data: {description: 业务数据}
    User:
      type: object
      properties:
        id: {type: integer}
        name: {type: string}
`)
	assertContains(t, code, "type GetRevRsp = User")
	assertNotContains(t, code, "type GetRevRsp struct")
}

func TestE2E_Envelope_AllOfNegativeExtraField(t *testing.T) {
	// allOf merging the envelope with an extra business field yields four merged
	// properties -> not the three-field envelope -> NOT unwrapped; falls back to
	// the existing resolveAllOf embedding behavior.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /extra:
    get:
      operationId: getExtra
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                allOf:
                  - $ref: "#/components/schemas/Envelope"
                  - properties:
                      data:
                        $ref: "#/components/schemas/User"
                      extra: {type: string}
components:
  schemas:
    Envelope:
      type: object
      properties:
        code: {type: integer}
        msg: {type: string}
        data: {description: 业务数据}
    User:
      type: object
      properties:
        id: {type: integer}
        name: {type: string}
`)
	assertContains(t, code, "type GetExtraRsp struct")
	assertContains(t, code, "Extra *string")
	assertNotContains(t, code, "type GetExtraRsp = User")
}

func TestE2E_Envelope_AllOfSplitMembers(t *testing.T) {
	// code/msg in one allOf member, data in another (neither a $ref) -> still
	// merges to the three-field envelope and unwraps.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /split:
    get:
      operationId: getSplit
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                allOf:
                  - properties:
                      code: {type: integer}
                      msg: {type: string}
                  - properties:
                      data:
                        $ref: "#/components/schemas/User"
components:
  schemas:
    User:
      type: object
      properties:
        id: {type: integer}
        name: {type: string}
`)
	assertContains(t, code, "type GetSplitRsp = User")
	assertNotContains(t, code, "type GetSplitRsp struct")
}

func TestE2E_Envelope_AllOfBothRefs(t *testing.T) {
	// Both allOf members are $refs: a reusable envelope plus a separate
	// component carrying the specific data override.
	code := generateFromInlineSpec(t, `openapi: "3.0.3"
info: {title: t, version: "1.0.0"}
paths:
  /both:
    get:
      operationId: getBoth
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                allOf:
                  - $ref: "#/components/schemas/Envelope"
                  - $ref: "#/components/schemas/AccountData"
components:
  schemas:
    Envelope:
      type: object
      properties:
        code: {type: integer}
        msg: {type: string}
        data: {description: 业务数据}
    User:
      type: object
      properties:
        id: {type: integer}
        name: {type: string}
    AccountData:
      type: object
      properties:
        data:
          $ref: "#/components/schemas/User"
`)
	assertContains(t, code, "type GetBothRsp = User")
	assertNotContains(t, code, "type GetBothRsp struct")
}

// OpenAPI 3.1 expresses nullable fields via type arrays (e.g. ["integer","null"]).
// The envelope predicate relies on typeIs, which tolerates a "null" companion, so
// 3.1 nullable envelopes unwrap the same as their 3.0 counterparts.

func TestE2E_Envelope_OAI31_NullableInline(t *testing.T) {
	// 3.1 nullable inline envelope -> unwraps to the data $ref.
	code := generateSingleFileV(t, "openapi-3.1", "envelope_unwrap.yaml")
	assertContains(t, code, "type GetUserRsp = User")
	assertNotContains(t, code, "type GetUserRsp struct")
}

func TestE2E_Envelope_OAI31_NullableAllOf(t *testing.T) {
	// 3.1 nullable allOf-composed envelope -> unwraps to the data $ref.
	code := generateSingleFileV(t, "openapi-3.1", "envelope_unwrap.yaml")
	assertContains(t, code, "type GetAccountRsp = User")
	assertNotContains(t, code, "type GetAccountRsp struct")
}

// generateFromInlineSpec writes spec to a temp dir and returns the generated
// types source. Used for envelope edge cases that don't warrant a fixture.
func generateFromInlineSpec(t *testing.T, spec string, opts ...func(*Config)) string {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	cfg := Config{
		PackageName:   "api",
		SpecPath:      specPath,
		OutputOptions: OutputOptions{SkipFmt: true},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	return string(result.Types)
}

// ============================================================
// Module 5: Validation Rules
// ============================================================

func TestE2E_Validation_Required(t *testing.T) {
	code := generateSingleFile(t, "validation.yaml")
	assertContains(t, code, `Name string `+"`"+`json:"name" binding:"required,min=2,max=100"`+"`")
}

func TestE2E_Validation_Format(t *testing.T) {
	code := generateSingleFile(t, "validation.yaml")
	assertContains(t, code, `Email string `+"`"+`json:"email" binding:"required,email"`+"`")
	assertContains(t, code, `Website *string `+"`"+`json:"website" binding:"url"`+"`")
	assertContains(t, code, `UUIDField *string `+"`"+`json:"uuid_field" binding:"uuid"`+"`")
	assertContains(t, code, `Ipv4Field *string `+"`"+`json:"ipv4_field" binding:"ipv4"`+"`")
	assertContains(t, code, `Ipv6Field *string `+"`"+`json:"ipv6_field" binding:"ipv6"`+"`")
	assertContains(t, code, `HostnameField *string `+"`"+`json:"hostname_field" binding:"hostname"`+"`")
}

func TestE2E_Validation_Enum(t *testing.T) {
	code := generateSingleFile(t, "validation.yaml")
	assertContains(t, code, `binding:"required,oneof=active inactive banned"`)
}

func TestE2E_Validation_MinMax(t *testing.T) {
	code := generateSingleFile(t, "validation.yaml")
	assertContains(t, code, `Score int `+"`"+`json:"score" binding:"required,gte=0,lte=100"`+"`")
}

func TestE2E_Validation_ExclusiveMinMax(t *testing.T) {
	code := generateSingleFile(t, "validation.yaml")
	assertContains(t, code, "gt=0")
	assertContains(t, code, "lt=5")
}

func TestE2E_Validation_ArrayConstraints(t *testing.T) {
	code := generateSingleFile(t, "validation.yaml")
	assertContains(t, code, "min=1")
	assertContains(t, code, "max=10")
	assertContains(t, code, "unique")
}

func TestE2E_Validation_CustomBinding(t *testing.T) {
	code := generateSingleFile(t, "validation.yaml")
	assertContains(t, code, `binding:"e164"`)
}

func TestE2E_Validation_DefaultValues(t *testing.T) {
	code := generateSingleFile(t, "validation.yaml")
	assertContains(t, code, `default:"1"`)
	assertContains(t, code, `default:"20"`)
	assertContains(t, code, `default:"created_at"`)
}

// ============================================================
// Module 5b: OpenAPI 3.1
//
// openapi-3.1/spec/openapi31.yaml uses `openapi: "3.1.0"`. The behavior under
// test is what 3.1 changes relative to 3.0:
//   - exclusiveMinimum/exclusiveMaximum are standalone numeric bounds
//     (not boolean modifiers on minimum/maximum).
//   - `type` may be an array, e.g. ["string", "null"].
// Everything else (formats, enums, inclusive min/max, array rules) must
// keep working unchanged under a 3.1 document.
// ============================================================

func TestE2E_OAI31_NumericExclusiveBounds(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.1", "openapi31.yaml")

	// exclusiveMinimum: 0 / exclusiveMaximum: 100 with NO companion
	// minimum/maximum must become exclusive validator bounds.
	assertContains(t, code, `Score int `+"`"+`json:"score" binding:"required,gt=0,lt=100"`+"`")
	// The 3.0-style inclusive gte/lte must NOT be emitted for score.
	assertNotContains(t, code, `json:"score" binding:"required,gte=0`)
}

func TestE2E_OAI31_InclusiveBounds(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.1", "openapi31.yaml")

	// Inclusive minimum/maximum still map to gte/lte under 3.1.
	assertContains(t, code, `Level int `+"`"+`json:"level" binding:"required,gte=1,lte=10"`+"`")
}

func TestE2E_OAI31_NullableTypeArray(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.1", "openapi31.yaml")

	// `type: ["string", "null"]` must parse without breaking generation.
	assertContains(t, code, `Nickname *string `+"`"+`json:"nickname"`+"`")
}

func TestE2E_OAI31_StringAndEnumAndArray(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.1", "openapi31.yaml")

	assertContains(t, code, `Name string `+"`"+`json:"name" binding:"required,min=2,max=100"`+"`")
	assertContains(t, code, `Email string `+"`"+`json:"email" binding:"required,email"`+"`")
	assertContains(t, code, `Status string `+"`"+`json:"status" binding:"required,oneof=active inactive"`+"`")
	assertContains(t, code, `Tags []string `+"`"+`json:"tags" binding:"min=1,unique"`+"`")
}

func TestE2E_OAI31_RoutesAndValidGo(t *testing.T) {
	multi := generateMultiFileV(t, "openapi-3.1", "openapi31.yaml")

	assertContains(t, string(multi.Server), `ginx.POST(r, "/oai31/validate", s.CreateOai31, opts...)`)
	assertContains(t, string(multi.Server), `CreateOai31(ctx context.Context, req *CreateOai31Req) (*CreateOai31Rsp, error)`)
	assertValidGo(t, string(multi.Types))
	assertValidGo(t, string(multi.Server))
	assertValidGo(t, string(multi.Client))
}

// ============================================================
// Module 5c: OpenAPI 3.1 full parity + new features
//
// The 12 aligned specs under openapi-3.1/spec/ are the 3.0 scenarios
// re-declared as openapi: "3.1.0" (with the lone 3.0-ism — boolean
// exclusiveMinimum/exclusiveMaximum — rewritten in 3.1 numeric form, which
// yields the same gt=/lt= bindings). They MUST generate byte-identical code
// to the 3.0 originals. The dedicated feature specs below cover 3.1-only
// additions: const, prefixItems tuples, nullable type arrays, webhooks, $defs.
// ============================================================

func TestE2E_OAI31_Parity_AlignedSpecsIdentical(t *testing.T) {
	// Every aligned 3.1 spec must produce the same generated types as its 3.0
	// counterpart — proving the full 3.0 test surface is covered under 3.1.
	aligned := []string{
		"basic_types", "client_sdk", "complex_types", "config_tags",
		"naming", "request_params", "response_types", "server_interface",
		"server_name", "sse_operations", "type_mapping", "validation",
	}
	for _, name := range aligned {
		t.Run(name, func(t *testing.T) {
			v30 := generateSingleFile(t, name+".yaml")
			v31 := generateSingleFileV(t, "openapi-3.1", name+".yaml")
			if v30 != v31 {
				t.Fatalf("3.1 output differs from 3.0 for %s.yaml\n--- 3.0 ---\n%s\n--- 3.1 ---\n%s", name, v30, v31)
			}
		})
	}
}

func TestE2E_OAI31_Parity_MultiFileServerClientIdentical(t *testing.T) {
	// Spot-check multi-file server+client parity for two representative specs.
	for _, name := range []string{"server_interface", "sse_operations"} {
		t.Run(name, func(t *testing.T) {
			a := generateMultiFile(t, name+".yaml")
			b := generateMultiFileV(t, "openapi-3.1", name+".yaml")
			if string(a.Server) != string(b.Server) {
				t.Fatalf("server output differs for %s", name)
			}
			if string(a.Client) != string(b.Client) {
				t.Fatalf("client output differs for %s", name)
			}
		})
	}
}

func TestE2E_OAI31_Const(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.1", "const_types.yaml")

	// string/integer const -> oneof=<value>.
	assertContains(t, code, `Kind *string `+"`"+`json:"kind" binding:"oneof=payment"`+"`")
	assertContains(t, code, `Retries *int `+"`"+`json:"retries" binding:"oneof=3"`+"`")
	// boolean const -> NO binding rule (validator oneof panics on bool).
	assertContains(t, code, `Active *bool `+"`"+`json:"active"`+"`")
	assertNotContains(t, code, `oneof=true`)
	assertValidGo(t, code)
}

func TestE2E_OAI31_PrefixItemsTuples(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.1", "prefix_items.yaml")

	// Top-level tuple -> []any.
	assertContains(t, code, "type Point = []any")
	// Tuple as object property -> []any; tuple nested in array item -> [][]any.
	assertContains(t, code, "Coords []any")
	assertContains(t, code, "Samples [][]any")
	assertValidGo(t, code)
}

func TestE2E_OAI31_NullableTypeArrays(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.1", "nullable_types.yaml")

	// Scalar nullable type arrays -> pointers of the scalar Go type.
	assertContains(t, code, `Nickname *string `+"`"+`json:"nickname"`+"`")
	assertContains(t, code, `Age *int `+"`"+`json:"age"`+"`")
	assertContains(t, code, `Score *float64 `+"`"+`json:"score"`+"`")
	assertContains(t, code, `Flag *bool `+"`"+`json:"flag"`+"`")
	// Nullable array ["array","null"] -> []string (Go slices are nilable).
	assertContains(t, code, `Tags []string `+"`"+`json:"tags"`+"`")
	assertValidGo(t, code)
}

func TestE2E_OAI31_Webhooks(t *testing.T) {
	multi := generateMultiFileV(t, "openapi-3.1", "webhooks.yaml")
	server := string(multi.Server)

	// Webhook synthesized as a receiver route under /webhooks/<name>.
	assertContains(t, server, `ginx.POST(r, "/webhooks/ordercreated", s.HandleOrderCreated, opts...)`)
	assertContains(t, server, "HandleOrderCreated(ctx context.Context, req *HandleOrderCreatedReq) (*HandleOrderCreatedRsp, error)")
	assertValidGo(t, server)
	assertValidGo(t, string(multi.Client))
}

func TestE2E_OAI31_DefsAndRefs(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.1", "defs_and_refs.yaml")

	// license.identifier + $defs block parse without breaking generation;
	// top-level $ref schemas resolve to generated types.
	assertContains(t, code, "type Address struct")
	assertContains(t, code, "type Person struct")
	assertContains(t, code, "Home *Address")
	assertContains(t, code, "Shipping *Address")
	assertValidGo(t, code)
}

// ============================================================
// Module 5d: OpenAPI 3.2 (SSE + JSON Lines + querystring)
//
// openapi-3.2/spec/* uses `openapi: "3.2.0"`. kin-openapi v0.142.0 preserves
// MediaType.itemSchema and validates the JSON Lines fixtures below. Its strict
// Validate still rejects the QUERY method, additionalOperations, and structured
// Tags. The `in: querystring` parameter location is normalized to query because
// the structured whole-query-string form is not representable in kin-openapi.
// The buildable 3.2 streaming features are exercised by real round-trip
// fixtures under openapi-3.2/code/.
// ============================================================

func TestE2E_OAI32_VersionLoadsAndGenerates(t *testing.T) {
	// A 3.2.0 document must load, validate, and generate valid Go.
	multi := generateMultiFileV(t, "openapi-3.2", "sse_operations.yaml")
	assertValidGo(t, string(multi.Types))
	assertValidGo(t, string(multi.Server))
	assertValidGo(t, string(multi.Client))
}

func TestE2E_OAI32_SSEUnderDoc(t *testing.T) {
	server := string(generateMultiFileV(t, "openapi-3.2", "sse_operations.yaml").Server)
	assertContains(t, server, "StreamEvents(ctx context.Context, req *StreamEventsReq, send ginx.Sender) error")
	assertContains(t, server, `ginx.SSE(r, "/events/stream", s.StreamEvents, opts...)`)
}

func TestE2E_OAI32_JSONLinesItemSchemaPreserved(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile(specPath("openapi-3.2", "jsonlines.yaml"))
	if err != nil {
		t.Fatalf("load OpenAPI 3.2 JSON Lines spec: %v", err)
	}

	tests := []struct {
		path        string
		contentType string
		operation   func(*openapi3.PathItem) *openapi3.Operation
	}{
		{
			path:        "/logs/{source}/tail",
			contentType: "application/jsonl",
			operation:   func(item *openapi3.PathItem) *openapi3.Operation { return item.Get },
		},
		{
			path:        "/ingest",
			contentType: "application/x-ndjson",
			operation:   func(item *openapi3.PathItem) *openapi3.Operation { return item.Post },
		},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			pathItem := doc.Paths.Value(tt.path)
			if pathItem == nil {
				t.Fatalf("path %q was not loaded", tt.path)
			}
			op := tt.operation(pathItem)
			if op == nil || op.Responses == nil {
				t.Fatalf("operation for %q was not loaded", tt.path)
			}
			response := op.Responses.Status(http.StatusOK)
			if response == nil || response.Value == nil {
				t.Fatalf("200 response for %q was not loaded", tt.path)
			}
			mediaType := response.Value.Content[tt.contentType]
			if mediaType == nil {
				t.Fatalf("content type %q was not loaded", tt.contentType)
			}
			if mediaType.ItemSchema == nil || mediaType.ItemSchema.Value == nil {
				t.Fatalf("itemSchema for %q was not preserved", tt.contentType)
			}
			if mediaType.Schema != nil {
				t.Fatalf("legacy schema unexpectedly populated for %q", tt.contentType)
			}
		})
	}
}

func TestE2E_OAI32_JSONLinesServer(t *testing.T) {
	server := string(generateMultiFileV(t, "openapi-3.2", "jsonlines.yaml").Server)
	// Both JSON Lines media types use OpenAPI 3.2 itemSchema and generate
	// ginx.JSONLines streaming handlers, NOT FileRsp binary handlers.
	assertContains(t, server, "TailLogs(ctx context.Context, req *TailLogsReq, send ginx.JSONLinesSender) error")
	assertContains(t, server, "IngestBatch(ctx context.Context, req *IngestBatchReq, send ginx.JSONLinesSender) error")
	assertContains(t, server, `ginx.JSONLines(r, "GET", "/logs/:source/tail", s.TailLogs, opts...)`)
	assertContains(t, server, `ginx.JSONLines(r, "POST", "/ingest", s.IngestBatch, opts...)`)
	assertNotContains(t, server, "ginx.FileRsp")
}

func TestE2E_OAI32_JSONLinesSingleFileServer(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.2", "jsonlines.yaml")
	assertContains(t, code, "TailLogs(ctx context.Context, req *TailLogsReq, send ginx.JSONLinesSender) error")
	assertContains(t, code, "IngestBatch(ctx context.Context, req *IngestBatchReq, send ginx.JSONLinesSender) error")
	assertContains(t, code, `ginx.JSONLines(r, "GET", "/logs/:source/tail", s.TailLogs, opts...)`)
	assertContains(t, code, `ginx.JSONLines(r, "POST", "/ingest", s.IngestBatch, opts...)`)
	assertValidGo(t, code)
}

func TestE2E_OAI32_JSONLinesClient(t *testing.T) {
	client := string(generateMultiFileV(t, "openapi-3.2", "jsonlines.yaml").Client)
	assertContains(t, client, "TailLogs(ctx context.Context, req *TailLogsReq) (*ginx.JSONLinesStream, error)")
	assertContains(t, client, "IngestBatch(ctx context.Context, req *IngestBatchReq) (*ginx.JSONLinesStream, error)")
	// The streaming client must opt out of response buffering.
	assertContains(t, client, ".SetResponseDoNotParse(true)")
	assertContains(t, client, "ginx.ValidateResponseStatus(resp.StatusCode(), 200)")
	assertContains(t, client, "ginx.NewJSONLinesStream(ctx, resp.Body)")
}

func TestE2E_OAI32_JSONLinesMediaTypesNotBinary(t *testing.T) {
	// Direct guard: JSON Lines media types must not be classified as binary.
	if isBinaryContentType("application/jsonl") {
		t.Error("application/jsonl should not be binary")
	}
	if isBinaryContentType("application/x-ndjson") {
		t.Error("application/x-ndjson should not be binary")
	}
	if isJSONLinesContentType("application/jsonl") != true || isJSONLinesContentType("application/x-ndjson") != true {
		t.Error("jsonl/x-ndjson should be JSON Lines content types")
	}
}

func TestE2E_OAI32_QuerystringNormalizedToQuery(t *testing.T) {
	code := generateSingleFileV(t, "openapi-3.2", "querystring_params.yaml")
	// `in: querystring` (3.2) is normalized to an ordinary query parameter.
	assertContains(t, code, `Q string `+"`"+`form:"q" binding:"required"`+"`")
	assertContains(t, code, `Limit *int `+"`"+`form:"limit"`+"`")
	assertValidGo(t, code)
}

// TestE2E_OAI32_UnsupportedStructuralFieldsRejected documents the remaining
// kin-openapi v0.142.0 boundary. itemSchema is intentionally absent from this
// table because it is now represented and exercised by jsonlines.yaml above.
func TestE2E_OAI32_UnsupportedStructuralFieldsRejected(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"additionalOperations": `openapi: "3.2.0"
info: { title: t, version: "1" }
paths:
  /s:
    get:
      operationId: s
      responses: { "200": { description: ok } }
    additionalOperations:
      PURGE:
        operationId: p
        responses: { "204": { description: ok } }
`,
		"queryMethod": `openapi: "3.2.0"
info: { title: t, version: "1" }
paths:
  /s:
    query:
      operationId: q
      responses: { "200": { description: ok } }
`,
		"structuredTags": `openapi: "3.2.0"
info: { title: t, version: "1" }
tags:
  - { name: x, kind: resource, parent: y, summary: s }
paths: {}
`,
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			specPath := filepath.Join(dir, name+".yaml")
			if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
				t.Fatalf("write spec: %v", err)
			}
			_, err := GenerateMulti(Config{
				PackageName:   "api",
				SpecPath:      specPath,
				OutputOptions: OutputOptions{SkipFmt: true},
			})
			if err == nil {
				t.Fatalf("expected validation error for 3.2 field %q, got nil", name)
			}
			if !strings.Contains(err.Error(), "validate spec") {
				t.Fatalf("error should come from spec validation, got: %v", err)
			}
		})
	}
}

// ============================================================
// Module 6: Server Interface
// ============================================================

func TestE2E_Server_InterfaceGenerated(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	server := string(result.Server)

	assertContains(t, server, "type ServerInterface interface")
	assertContains(t, server, "ListPets(ctx context.Context, req *ListPetsReq) (*ListPetsRsp, error)")
	assertContains(t, server, "CreatePet(ctx context.Context, req *CreatePetReq) (*CreatePetRsp, error)")
	assertContains(t, server, "GetPet(ctx context.Context, req *GetPetReq) (*GetPetRsp, error)")
	assertContains(t, server, "DeletePet(ctx context.Context, req *DeletePetReq) (*struct{}, error)")
}

func TestE2E_Server_RegisterRoutes(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	server := string(result.Server)

	assertContains(t, server, "func RegisterRoutes(r gin.IRoutes, s ServerInterface, opts ...ginx.RouteOption)")
	assertContains(t, server, `ginx.GET(r, "/pets", s.ListPets, opts...)`)
	assertContains(t, server, `ginx.POST(r, "/pets", s.CreatePet, append(append([]ginx.RouteOption(nil), opts...), ginx.SuccessStatus(201))...)`)
	assertContains(t, server, `ginx.GET(r, "/pets/:pet_id", s.GetPet, opts...)`)
	assertContains(t, server, `ginx.DELETE(r, "/pets/:pet_id", s.DeletePet, append(append([]ginx.RouteOption(nil), opts...), ginx.SuccessStatus(204))...)`)
}

func TestE2E_Server_SSEHandler(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	server := string(result.Server)

	assertContains(t, server, "StreamEvents(ctx context.Context, req *StreamEventsReq, send ginx.Sender) error")
	assertContains(t, server, `ginx.SSE(r, "/events", s.StreamEvents, opts...)`)
}

func TestE2E_Server_SSEViaContentType(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	server := string(result.Server)

	assertContains(t, server, "StreamNotifications(ctx context.Context, req *StreamNotificationsReq, send ginx.Sender) error")
	assertContains(t, server, `ginx.SSE(r, "/notifications", s.StreamNotifications, opts...)`)
}

func TestE2E_Server_SSERejectsNon200Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sse-202.yaml")
	spec := `openapi: 3.0.3
info:
  title: invalid SSE status
  version: 1.0.0
paths:
  /events:
    get:
      operationId: streamEvents
      x-ginx-sse: true
      responses:
        "200":
          description: normal stream status
          content:
            text/event-stream:
              schema:
                type: string
        "202":
          description: invalid stream status
          content:
            text/event-stream:
              schema:
                type: string
`
	if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	_, err := GenerateMulti(Config{PackageName: "api", SpecPath: path, OutputOptions: OutputOptions{SkipFmt: true}})
	if err == nil || !strings.Contains(err.Error(), "SSE success response must use HTTP 200") {
		t.Fatalf("GenerateMulti error = %v, want SSE HTTP 200 contract error", err)
	}
}

func TestE2E_Server_JSONLinesRejectsNon200Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jsonlines-202.yaml")
	spec := `openapi: 3.0.3
info:
  title: invalid JSON Lines status
  version: 1.0.0
paths:
  /events:
    get:
      operationId: streamEvents
      x-ginx-jsonl: true
      responses:
        "200":
          description: normal stream status
          content:
            application/x-ndjson:
              schema:
                type: string
        "202":
          description: invalid stream status
          content:
            application/x-ndjson:
              schema:
                type: string
`
	if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	_, err := GenerateMulti(Config{PackageName: "api", SpecPath: path, OutputOptions: OutputOptions{SkipFmt: true}})
	if err == nil || !strings.Contains(err.Error(), "JSON Lines success response must use HTTP 200") {
		t.Fatalf("GenerateMulti error = %v, want JSON Lines HTTP 200 contract error", err)
	}
}

func TestE2E_Server_PathConversion(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	server := string(result.Server)

	assertContains(t, server, "/pets/:pet_id")
	assertNotContains(t, server, "{pet_id}")
}

func TestE2E_Server_Imports(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	server := string(result.Server)

	assertContains(t, server, `"context"`)
	assertContains(t, server, `"github.com/chendefine/ginx"`)
	assertContains(t, server, `"github.com/gin-gonic/gin"`)
}

// ============================================================
// Module 7: Client SDK
// ============================================================

func TestE2E_Client_InterfaceGenerated(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, "type ClientInterface interface")
	assertContains(t, client, "ListItems(ctx context.Context, req *ListItemsReq)")
	assertContains(t, client, "CreateItem(ctx context.Context, req *CreateItemReq)")
	assertContains(t, client, "GetItem(ctx context.Context, req *GetItemReq)")
	assertContains(t, client, "UpdateItem(ctx context.Context, req *UpdateItemReq)")
	assertContains(t, client, "DeleteItem(ctx context.Context, req *DeleteItemReq) error")
}

func TestE2E_Client_StructAndConstructor(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, "type Client struct")
	assertContains(t, client, "client *resty.Client")
	assertContains(t, client, "func NewClient(baseURL string, opts ...ClientOption) *Client")
	assertContains(t, client, "type ClientOption func(*resty.Client)")
}

func TestE2E_Client_PathParams(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `r.SetPathParam("item_id"`)
}

func TestE2E_Client_QueryParams(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `r.SetQueryParam("page"`)
	assertContains(t, client, `r.SetQueryParam("limit"`)
	assertContains(t, client, "if req.Page != nil")
}

func TestE2E_Client_TimeParam(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `"time"`)
	assertContains(t, client, ".Format(time.RFC3339)")
}

func TestE2E_Client_HeaderParams(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `r.SetHeader("X-Tenant-ID"`)
	assertContains(t, client, `r.SetHeader("X-Idempotency-Key"`)
}

func TestE2E_Client_CookieParams(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `r.SetCookie(&http.Cookie{Name: "sid"`)
}

func TestE2E_Client_BodyEmbed(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, "r.SetBody(&req.CreateItemInput)")
}

func TestE2E_Client_RefOnlyBodyAlias(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	types := string(result.Types)
	client := string(result.Client)

	assertContains(t, types, "type CreatePetReq = Pet")
	assertContains(t, client, "r.SetBody(req)")
	assertNotContains(t, client, "r.SetBody(&req.Pet)")
}

func TestE2E_Client_BodyInlineWithParams(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `"name": req.Name`)
}

func TestE2E_Client_FormURLEncodedBody(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	types := string(result.Types)
	client := string(result.Client)

	assertContains(t, types, "type CreateTokenReq struct")
	assertContains(t, types, `Username string `+"`"+`form:"username" binding:"required"`+"`")
	assertContains(t, client, `r.SetFormData(formData)`)
	assertContains(t, client, `"username": req.Username`)
	assertNotContains(t, client, `r.SetQueryParam("username"`)
}

func TestE2E_Client_FileRspReturn(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, "func (c *Client) ExportItem(ctx context.Context, req *ExportItemReq) ([]byte, error)")
	assertContains(t, client, "resp.Bytes(), nil")
}

func TestE2E_Client_StringRspReturn(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, "func (c *Client) GetItemDescription(ctx context.Context, req *GetItemDescriptionReq) (string, error)")
	assertContains(t, client, "resp.String(), nil")
}

func TestE2E_Client_NoContentReturn(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, "func (c *Client) DeleteItem(ctx context.Context, req *DeleteItemReq) error")
}

func TestE2E_Client_RedirectReturn(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, "func (c *Client) RedirectToItems(ctx context.Context, req *RedirectToItemsReq) error")
}

func TestE2E_Client_MultipartUploadFailsClearly(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "multipart.yaml")
	spec := `openapi: "3.0.3"
info:
  title: Multipart Client
  version: "1.0.0"
paths:
  /upload:
    post:
      operationId: uploadItemImage
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                image:
                  type: string
                  format: binary
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	_, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      specPath,
		Output:        OutputConfig{Types: "types.go", Client: "client.go"},
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err == nil || !strings.Contains(err.Error(), "client generation does not support multipart file upload for operation UploadItemImage") {
		t.Fatalf("GenerateMulti error = %v, want multipart client unsupported", err)
	}
}

func TestE2E_Client_HTTPMethods(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `.Get("/items")`)
	assertContains(t, client, `.Post("/items")`)
	assertContains(t, client, `.Put("/items/{item_id}")`)
	assertContains(t, client, `.Delete("/items/{item_id}")`)
}

func TestE2E_Client_Imports(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `"context"`)
	assertContains(t, client, `"fmt"`)
	assertContains(t, client, `"resty.dev/v3"`)
	assertContains(t, client, `"github.com/chendefine/ginx"`)
}

// ============================================================
// Module 8: SSE Operations
// ============================================================

func TestE2E_SSE_ServerInterface(t *testing.T) {
	result := generateMultiFile(t, "sse_operations.yaml")
	server := string(result.Server)

	assertContains(t, server, "StreamEvents(ctx context.Context, req *StreamEventsReq, send ginx.Sender) error")
	assertContains(t, server, "StreamRoomMessages(ctx context.Context, req *StreamRoomMessagesReq, send ginx.Sender) error")
	assertContains(t, server, "StreamNotifications(ctx context.Context, req *StreamNotificationsReq, send ginx.Sender) error")
	assertContains(t, server, "StreamMetrics(ctx context.Context, req *StreamMetricsReq, send ginx.Sender) error")
}

func TestE2E_SSE_RouteRegistration(t *testing.T) {
	result := generateMultiFile(t, "sse_operations.yaml")
	server := string(result.Server)

	assertContains(t, server, `ginx.SSE(r, "/events/stream", s.StreamEvents, opts...)`)
	assertContains(t, server, `ginx.SSE(r, "/rooms/:room_id/messages", s.StreamRoomMessages, opts...)`)
	assertContains(t, server, `ginx.SSE(r, "/notifications", s.StreamNotifications, opts...)`)
	assertContains(t, server, `ginx.SSE(r, "/metrics", s.StreamMetrics, opts...)`)
}

func TestE2E_SSE_ClientInterface(t *testing.T) {
	result := generateMultiFile(t, "sse_operations.yaml")
	client := string(result.Client)

	assertContains(t, client, "StreamEvents(ctx context.Context, req *StreamEventsReq) (*ginx.SSEStream, error)")
	assertContains(t, client, "StreamRoomMessages(ctx context.Context, req *StreamRoomMessagesReq) (*ginx.SSEStream, error)")
	assertContains(t, client, "StreamNotifications(ctx context.Context, req *StreamNotificationsReq) (*ginx.SSEStream, error)")
	assertContains(t, client, "StreamMetrics(ctx context.Context, req *StreamMetricsReq) (*ginx.SSEStream, error)")
}

func TestE2E_SSE_ClientPathParam(t *testing.T) {
	result := generateMultiFile(t, "sse_operations.yaml")
	client := string(result.Client)

	assertContains(t, client, `strings.Replace(sseURL, "{room_id}", url.PathEscape(req.RoomID), 1)`)
}

func TestE2E_SSE_ClientQueryParam(t *testing.T) {
	result := generateMultiFile(t, "sse_operations.yaml")
	client := string(result.Client)

	assertContains(t, client, `q.Set("channel", req.Channel)`)
	assertContains(t, client, "if req.LastEventID != nil")
}

func TestE2E_SSE_ClientHeaderParam(t *testing.T) {
	result := generateMultiFile(t, "sse_operations.yaml")
	client := string(result.Client)

	assertContains(t, client, `es.SetHeader("X-Auth-Token"`)
}

func TestE2E_SSE_ClientImports(t *testing.T) {
	result := generateMultiFile(t, "sse_operations.yaml")
	client := string(result.Client)

	assertContains(t, client, `"strings"`)
	assertContains(t, client, `"net/url"`)
}

func TestE2E_SSE_NoResponseType(t *testing.T) {
	code := generateSingleFile(t, "sse_operations.yaml")
	assertNotContains(t, code, "StreamEventsRsp")
	assertNotContains(t, code, "StreamRoomMessagesRsp")
}

// ============================================================
// Module 9: Naming Rules
// ============================================================

func TestE2E_Naming_Initialisms(t *testing.T) {
	code := generateSingleFile(t, "naming.yaml")

	assertContains(t, code, "type HTTPURLConfig struct")
	assertContains(t, code, "HTTPURL *string")
	assertContains(t, code, "APIKey *string")
	assertContains(t, code, "UserID *int64")
	assertContains(t, code, "HTMLContent *string")
	assertContains(t, code, "JSONData *string")
	assertContains(t, code, "TCPPort *int")
	assertContains(t, code, "SSHKey *string")
	assertContains(t, code, "CPUUsage *float32")
	assertContains(t, code, "DNSServer *string")
	assertContains(t, code, "TLSEnabled *bool")
	assertContains(t, code, "XMLPayload *string")
	assertContains(t, code, "UUIDField *string")
}

func TestE2E_Naming_OperationID(t *testing.T) {
	code := generateSingleFile(t, "naming.yaml")
	assertContains(t, code, "type ListHTTPAPIEndpointsReq struct")
}

func TestE2E_Naming_NumericSchemaRef(t *testing.T) {
	code := generateSingleFile(t, "naming.yaml")
	assertContains(t, code, "type X123Error struct")
	assertContains(t, code, "type GetNumericSchemaRefRsp = X123Error")
	assertNotContains(t, code, " = 123Error")
}

func TestE2E_Naming_NoOperationID(t *testing.T) {
	code := generateSingleFile(t, "naming.yaml")
	assertContains(t, code, "type GetNoOperationIDResourceIDReq struct")
	assertContains(t, code, "type PostNoOperationIDResourceIDReq struct")
}

func TestE2E_Naming_ParamNames(t *testing.T) {
	code := generateSingleFile(t, "naming.yaml")
	assertContains(t, code, `APIID string`)
	assertContains(t, code, `IPAddress *string`)
}

// ============================================================
// Module 10: Config Features - Tag Filtering
// ============================================================

func TestE2E_Config_IncludeTags(t *testing.T) {
	code := generateSingleFile(t, "config_tags.yaml", func(cfg *Config) {
		cfg.IncludeTags = []string{"users", "pets"}
	})

	assertContains(t, code, "ListUsersReq")
	assertContains(t, code, "ListPetsReq")
	assertNotContains(t, code, "HealthCheckReq")
	assertNotContains(t, code, "GetStatsReq")
	assertNotContains(t, code, "GetUntaggedReq")
}

func TestE2E_Config_ExcludeTags(t *testing.T) {
	code := generateSingleFile(t, "config_tags.yaml", func(cfg *Config) {
		cfg.ExcludeTags = []string{"internal"}
	})

	assertContains(t, code, "ListUsersReq")
	assertContains(t, code, "ListPetsReq")
	assertNotContains(t, code, "HealthCheckReq")
	assertNotContains(t, code, "GetStatsReq")
	assertContains(t, code, "GetUntaggedReq")
}

func TestE2E_Config_NoTagFilter(t *testing.T) {
	code := generateSingleFile(t, "config_tags.yaml")

	assertContains(t, code, "ListUsersReq")
	assertContains(t, code, "ListPetsReq")
	assertContains(t, code, "HealthCheckReq")
	assertContains(t, code, "GetStatsReq")
	assertContains(t, code, "GetUntaggedReq")
}

// ============================================================
// Module 11: Config Features - Server Name
// ============================================================

func TestE2E_Config_ServerName(t *testing.T) {
	result := generateMultiFile(t, "server_name.yaml", func(cfg *Config) {
		cfg.ServerName = "order_service"
	})
	server := string(result.Server)

	assertContains(t, server, "type OrderServiceServerInterface interface")
	assertContains(t, server, "func RegisterOrderServiceRoutes(r gin.IRoutes, s OrderServiceServerInterface")
}

func TestE2E_Config_ServerNameClient(t *testing.T) {
	result := generateMultiFile(t, "server_name.yaml", func(cfg *Config) {
		cfg.ServerName = "order_service"
	})
	client := string(result.Client)

	assertContains(t, client, "type OrderServiceClientInterface interface")
	assertContains(t, client, "type OrderServiceClient struct")
	assertContains(t, client, "func NewOrderServiceClient(baseURL string, opts ...OrderServiceClientOption)")
	assertContains(t, client, "type OrderServiceClientOption func(*resty.Client)")
}

func TestE2E_Config_NoServerName(t *testing.T) {
	result := generateMultiFile(t, "server_name.yaml")
	server := string(result.Server)

	assertContains(t, server, "type ServerInterface interface")
	assertContains(t, server, "func RegisterRoutes(r gin.IRoutes, s ServerInterface")
}

// ============================================================
// Module 12: Config Features - Type Mapping
// ============================================================

func TestE2E_Config_TypeMapping(t *testing.T) {
	code := generateSingleFile(t, "type_mapping.yaml", func(cfg *Config) {
		cfg.TypeMapping = map[string]string{
			"time.Time": "string",
			"int64":     "MyInt64",
		}
	})

	assertContains(t, code, "CreatedAt string")
	assertContains(t, code, "ID MyInt64")
	assertNotContains(t, code, "time.Time")
	assertContains(t, code, "Timestamps []string")
	assertContains(t, code, "Schedule map[string]string")
}

func TestE2E_Config_TypeMappingExt(t *testing.T) {
	code := generateSingleFile(t, "type_mapping.yaml", func(cfg *Config) {
		cfg.TypeMappingExt = map[string]TypeMappingExt{
			"time.Time": {Type: "civil.DateTime", Import: "cloud.google.com/go/civil"},
		}
	})

	assertContains(t, code, `"cloud.google.com/go/civil"`)
	assertContains(t, code, "CreatedAt civil.DateTime")
	assertContains(t, code, "UpdatedAt *civil.DateTime")
	assertContains(t, code, "Timestamps []civil.DateTime")
	assertContains(t, code, "Schedule map[string]civil.DateTime")
}

// ============================================================
// Module 13: Config Features - Generate Directive
// ============================================================

func TestE2E_Config_GenerateDirective(t *testing.T) {
	code := generateSingleFile(t, "server_name.yaml", func(cfg *Config) {
		cfg.GenerateDirective = "go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml"
	})

	assertContains(t, code, "//go:generate go run github.com/chendefine/ginx/cmd/oapi-ginx -c oapi-ginx.yaml")
}

func TestE2E_Config_NoGenerateDirective(t *testing.T) {
	code := generateSingleFile(t, "server_name.yaml")
	assertNotContains(t, code, "//go:generate")
}

// ============================================================
// Module 14: Config Features - Disable Server/Client
// ============================================================

func TestE2E_Config_DisableServer(t *testing.T) {
	falseVal := false
	code := generateSingleFile(t, "server_interface.yaml", func(cfg *Config) {
		cfg.OutputOptions.GenerateServer = &falseVal
	})

	assertNotContains(t, code, "ServerInterface")
	assertNotContains(t, code, "RegisterRoutes")
}

func TestE2E_Config_DisableClient(t *testing.T) {
	falseVal := false
	result := generateMultiFile(t, "client_sdk.yaml", func(cfg *Config) {
		cfg.OutputOptions.GenerateClient = &falseVal
	})

	if result.Client != nil && len(result.Client) > 0 {
		t.Error("expected no client code when generate_client is false")
	}
}

func TestE2E_ClientCookieMultipartOperationFailsBeforeImportGeneration(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "openapi.yaml")
	spec := `openapi: "3.0.3"
info:
  title: Cookie Multipart Client Import
  version: "1.0.0"
paths:
  /upload:
    post:
      operationId: upload
      parameters:
        - name: sid
          in: cookie
          schema:
            type: string
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  ok:
                    type: boolean
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	_, err := GenerateMulti(Config{
		PackageName: "api",
		SpecPath:    specPath,
		Output:      OutputConfig{Types: "t.go", Client: "c.go"},
		OutputOptions: OutputOptions{
			SkipFmt: true,
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "client generation does not support multipart file upload for operation Upload") {
			return
		}
		t.Fatalf("GenerateMulti error = %v, want multipart client unsupported", err)
	}
	t.Fatalf("GenerateMulti succeeded, want multipart client unsupported")
}

// ============================================================
// Module 15: Single File Mode (Combined Output)
// ============================================================

func TestE2E_SingleFile_ContainsAll(t *testing.T) {
	trueVal := true
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath("server_interface.yaml"),
		Output:      OutputConfig{Single: "api.gen.go"},
		OutputOptions: OutputOptions{
			SkipFmt:        true,
			GenerateClient: &trueVal,
		},
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	code := string(result.Types)

	assertContains(t, code, "type Pet struct")
	assertContains(t, code, "type ServerInterface interface")
	assertContains(t, code, "func RegisterRoutes")
	assertContains(t, code, "type ClientInterface interface")
	assertContains(t, code, "func NewClient")
}

// ============================================================
// Module 16: Multi-File Mode
// ============================================================

func TestE2E_MultiFile_TypesOnly(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	types := string(result.Types)

	assertContains(t, types, "type Pet struct")
	assertContains(t, types, "type ListPetsReq struct")
	assertNotContains(t, types, "ServerInterface")
	assertNotContains(t, types, "RegisterRoutes")
}

func TestE2E_MultiFile_ServerOnly(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	server := string(result.Server)

	assertContains(t, server, "ServerInterface")
	assertContains(t, server, "RegisterRoutes")
	assertNotContains(t, server, "type Pet struct")
}

func TestE2E_MultiFile_ClientOnly(t *testing.T) {
	result := generateMultiFile(t, "server_interface.yaml")
	client := string(result.Client)

	assertContains(t, client, "ClientInterface")
	assertContains(t, client, "NewClient")
	assertNotContains(t, client, "type Pet struct")
	assertNotContains(t, client, "ServerInterface")
}

func TestE2E_ClientOnly_NoRequestStillGeneratesReq(t *testing.T) {
	falseVal := false
	result := generateMultiFile(t, "response_types.yaml", func(cfg *Config) {
		cfg.Output = OutputConfig{Types: "types.gen.go", Client: "client.gen.go"}
		cfg.OutputOptions.GenerateServer = &falseVal
	})

	types := string(result.Types)
	client := string(result.Client)
	assertContains(t, types, "type GetEmptyReq struct")
	assertContains(t, client, "GetEmpty(ctx context.Context, req *GetEmptyReq) error")
	assertValidGo(t, types)
	assertValidGo(t, client)
}

func TestE2E_Methods_HEADAndOPTIONS(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "methods.yaml")
	spec := `openapi: 3.0.3
info:
  title: Methods
  version: 1.0.0
paths:
  /ping:
    head:
      operationId: headPing
      responses:
        "204":
          description: ok
    options:
      operationId: optionsPing
      responses:
        "204":
          description: ok
  /typed-head:
    head:
      operationId: typedHead
      responses:
        "200":
          description: schema must not imply a response body for the client
          content:
            application/json:
              schema:
                type: object
                required: [etag]
                properties:
                  etag:
                    type: string
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	result, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      specPath,
		Output:        OutputConfig{Types: "types.go", Server: "server.go", Client: "client.go"},
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	server := string(result.Server)
	client := string(result.Client)
	assertContains(t, server, "HeadPing(ctx context.Context, req *HeadPingReq) (*struct{}, error)")
	assertContains(t, server, `ginx.HEAD(r, "/ping", s.HeadPing, append(append([]ginx.RouteOption(nil), opts...), ginx.SuccessStatus(204))...)`)
	assertContains(t, server, `ginx.OPTIONS(r, "/ping", s.OptionsPing, append(append([]ginx.RouteOption(nil), opts...), ginx.SuccessStatus(204))...)`)
	assertContains(t, server, "TypedHead(ctx context.Context, req *TypedHeadReq) (*TypedHeadRsp, error)")
	assertContains(t, client, "TypedHead(ctx context.Context, req *TypedHeadReq) error")
	assertContains(t, client, `resp, err := r.Head("/ping")`)
	assertContains(t, client, `resp, err := r.Options("/ping")`)
}

func TestE2E_TraceOperationReturnsError(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "trace.yaml")
	spec := `openapi: 3.0.3
info:
  title: Trace
  version: 1.0.0
paths:
  /trace:
    trace:
      operationId: tracePing
      responses:
        "204":
          description: ok
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	_, err := GenerateMulti(Config{PackageName: "api", SpecPath: specPath, OutputOptions: OutputOptions{SkipFmt: true}})
	if err == nil || !strings.Contains(err.Error(), "TRACE operations are not supported") {
		t.Fatalf("GenerateMulti error = %v, want TRACE unsupported", err)
	}
}

func TestE2E_ResponseStatusSelection(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "responses.yaml")
	spec := `openapi: 3.0.3
info:
  title: Responses
  version: 1.0.0
paths:
  /accepted:
    post:
      operationId: createJob
      responses:
        "202":
          description: accepted
          content:
            application/json:
              schema:
                type: object
                properties:
                  job_id:
                    type: string
  /partial:
    get:
      operationId: downloadPart
      responses:
        "200":
          description: complete
          content:
            application/octet-stream:
              schema:
                type: string
                format: binary
        "206":
          description: partial
          content:
            application/octet-stream:
              schema:
                type: string
                format: binary
  /empty:
    delete:
      operationId: deleteEmpty
      responses:
        "204":
          description: no content
  /redirect:
    get:
      operationId: redirectOnly
      responses:
        "302":
          description: found
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	result, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      specPath,
		Output:        OutputConfig{Types: "types.go", Server: "server.go"},
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	server := string(result.Server)
	types := string(result.Types)
	assertContains(t, types, "type CreateJobRsp struct")
	assertContains(t, server, "CreateJob(ctx context.Context, req *CreateJobReq) (*CreateJobRsp, error)")
	assertContains(t, server, "DownloadPart(ctx context.Context, req *DownloadPartReq) (*ginx.FileRsp, error)")
	assertContains(t, server, "DeleteEmpty(ctx context.Context, req *DeleteEmptyReq) (*struct{}, error)")
	assertContains(t, server, "RedirectOnly(ctx context.Context, req *RedirectOnlyReq) (*ginx.RedirectRsp, error)")
}

func TestE2E_Multiple2xxJSONSameSchemaAllowed(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "same_schema.yaml")
	spec := `openapi: 3.0.3
info:
  title: Same Schema
  version: 1.0.0
paths:
  /jobs:
    post:
      operationId: createJob
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
        "202":
          description: accepted
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	result, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      specPath,
		Output:        OutputConfig{Types: "types.go", Server: "server.go"},
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	assertContains(t, string(result.Server), "CreateJob(ctx context.Context, req *CreateJobReq) (*CreateJobRsp, error)")
}

func TestE2E_Multiple2xxJSONDifferentSchemaFails(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "different_schema.yaml")
	spec := `openapi: 3.0.3
info:
  title: Different Schema
  version: 1.0.0
paths:
  /jobs:
    post:
      operationId: createJob
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
        "202":
          description: accepted
          content:
            application/json:
              schema:
                type: object
                properties:
                  job_id:
                    type: string
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	_, err := GenerateMulti(Config{PackageName: "api", SpecPath: specPath, OutputOptions: OutputOptions{SkipFmt: true}})
	if err == nil || !strings.Contains(err.Error(), "multiple 2xx JSON responses have different schemas") {
		t.Fatalf("GenerateMulti error = %v, want multiple 2xx schema error", err)
	}
}

func TestE2E_Multiple2xxPrimaryResponseOverridesSelection(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "primary_response.yaml")
	spec := `openapi: 3.0.3
info:
  title: Primary Response
  version: 1.0.0
paths:
  /jobs:
    post:
      operationId: createJob
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
        "202":
          description: accepted
          x-ginx-primary-response: true
          content:
            application/json:
              schema:
                type: object
                properties:
                  job_id:
                    type: string
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	result, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      specPath,
		Output:        OutputConfig{Types: "types.go", Server: "server.go"},
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	types := string(result.Types)
	assertContains(t, types, "JobID *string")
	assertNotContains(t, types, "\tID *string")
}

func TestE2E_FormatErrorIsReturned(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "badfmt.yaml")
	spec := `openapi: 3.0.3
info:
  title: BadFmt
  version: 1.0.0
components:
  schemas:
    Broken:
      type: object
      properties:
        bad-name:
          type: string
paths: {}
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	_, err := GenerateMulti(Config{PackageName: "bad-package", SpecPath: specPath})
	if err == nil || !strings.Contains(err.Error(), "format generated code") {
		t.Fatalf("GenerateMulti error = %v, want format error", err)
	}

	result, err := GenerateMulti(Config{
		PackageName:   "bad-package",
		SpecPath:      specPath,
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti skip fmt: %v", err)
	}
	assertContains(t, string(result.Types), "package bad-package")
}

func TestE2E_NameConflictErrors(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "conflict.yaml")
	spec := `openapi: 3.0.3
info:
  title: Conflict
  version: 1.0.0
components:
  schemas:
    user_id:
      type: object
      properties:
        value:
          type: string
    user-id:
      type: object
      properties:
        value:
          type: string
paths: {}
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	_, err := GenerateMulti(Config{PackageName: "api", SpecPath: specPath, OutputOptions: OutputOptions{SkipFmt: true}})
	if err == nil || !strings.Contains(err.Error(), "type name conflict") {
		t.Fatalf("GenerateMulti error = %v, want type name conflict", err)
	}
	if !strings.Contains(err.Error(), "rename one schema") && !strings.Contains(err.Error(), "type_mapping") {
		t.Fatalf("GenerateMulti error = %v, want actionable schema conflict guidance", err)
	}
}

// ============================================================
// Module 17: Generated Code Validity
// ============================================================

func TestE2E_ValidGo_BasicTypes(t *testing.T) {
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath("basic_types.yaml"),
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	assertValidGo(t, string(result.Types))
}

func TestE2E_ValidGo_ComplexTypes(t *testing.T) {
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath("complex_types.yaml"),
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	assertValidGo(t, string(result.Types))
}

func TestE2E_ValidGo_ServerInterface(t *testing.T) {
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath("server_interface.yaml"),
		Output:      OutputConfig{Types: "t.go", Server: "s.go", Client: "c.go"},
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	assertValidGo(t, string(result.Types))
	assertValidGo(t, string(result.Server))
	assertValidGo(t, string(result.Client))
}

func TestE2E_ValidGo_ClientSDK(t *testing.T) {
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath("client_sdk.yaml"),
		Output:      OutputConfig{Types: "t.go", Server: "s.go", Client: "c.go"},
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	assertValidGo(t, string(result.Types))
	assertValidGo(t, string(result.Server))
	assertValidGo(t, string(result.Client))
}

func TestE2E_ValidGo_SSE(t *testing.T) {
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath("sse_operations.yaml"),
		Output:      OutputConfig{Types: "t.go", Server: "s.go", Client: "c.go"},
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	assertValidGo(t, string(result.Types))
	assertValidGo(t, string(result.Server))
	assertValidGo(t, string(result.Client))
}

func TestE2E_ValidGo_SingleFileCombined(t *testing.T) {
	trueVal := true
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath("client_sdk.yaml"),
		Output:      OutputConfig{Single: "api.gen.go"},
		OutputOptions: OutputOptions{
			GenerateClient: &trueVal,
		},
	}
	result, err := GenerateMulti(cfg)
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	assertValidGo(t, string(result.Types))
}

func TestE2E_ResponseVariants_MultiAndSingleFile(t *testing.T) {
	for _, version := range []string{"openapi-3.0", "openapi-3.1"} {
		t.Run(version+"/multi", func(t *testing.T) {
			result := generateMultiFileAt(t, specPath(version, "response_variants.yaml"))
			types := string(result.Types)
			server := string(result.Server)
			client := string(result.Client)

			assertContains(t, types, "type CreateJob201Rsp struct")
			assertContains(t, types, "type CreateJob202Rsp struct")
			assertContains(t, types, "type CreateJobResponse struct")
			assertContains(t, types, "func NewCreateJob201Response(body *CreateJob201Rsp) *CreateJobResponse")
			assertContains(t, types, "func NewCreateJob204Response() *CreateJobResponse")
			assertContains(t, types, "func (r *CreateJobResponse) As202() (*CreateJob202Rsp, bool)")
			assertContains(t, types, "func (r *CreateJobResponse) GinxResponseVariant() (int, any)")
			assertContains(t, server, "CreateJob(ctx context.Context, req *CreateJobReq) (*CreateJobResponse, error)")
			assertNotContains(t, server, "ginx.SuccessStatus(")
			assertContains(t, client, "ginx.ValidateResponseStatus(resp.StatusCode(), 201, 202, 204)")
			assertContains(t, client, "switch resp.StatusCode()")
			assertContains(t, client, "return NewCreateJob202Response(&result), nil")
			assertContains(t, client, "return NewCreateJob204Response(), nil")
			assertValidGo(t, types)
			assertValidGo(t, server)
			assertValidGo(t, client)
		})

		t.Run(version+"/single", func(t *testing.T) {
			generateClient := true
			code := generateSingleFileAt(t, specPath(version, "response_variants.yaml"), func(cfg *Config) {
				cfg.Output = OutputConfig{Single: "api.gen.go"}
				cfg.OutputOptions.GenerateClient = &generateClient
			})
			assertContains(t, code, "type CreateJobResponse struct")
			assertContains(t, code, "func NewCreateJob201Response(body *CreateJob201Rsp) *CreateJobResponse")
			assertContains(t, code, "return NewCreateJob204Response(), nil")
			assertValidGo(t, code)
		})
	}
}

func TestE2E_ResponseVariants_HEADKeepsDiscriminatedClientResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "head-variants.yaml")
	spec := `openapi: 3.0.3
info:
  title: HEAD variants
  version: 1.0.0
paths:
  /resource:
    head:
      operationId: probeResource
      x-ginx-response-mode: variants
      responses:
        "200":
          description: current
        "304":
          description: not modified
`
	if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	result, err := GenerateMulti(Config{
		PackageName:   "api",
		SpecPath:      path,
		Output:        OutputConfig{Types: "types.go", Server: "server.go", Client: "client.go"},
		OutputOptions: OutputOptions{SkipFmt: true},
	})
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	assertContains(t, string(result.Server), "ProbeResource(ctx context.Context, req *ProbeResourceReq) (*ProbeResourceResponse, error)")
	assertContains(t, string(result.Client), "ProbeResource(ctx context.Context, req *ProbeResourceReq) (*ProbeResourceResponse, error)")
	assertContains(t, string(result.Client), "return NewProbeResource304Response(), nil")
}

func TestE2E_ResponseVariants_RejectUnsupportedContracts(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		response  string
		want      string
	}{
		{
			name: "primary conflict",
			response: `        "201":
          description: created
          x-ginx-primary-response: true`,
			want: "conflicts with x-ginx-primary-response",
		},
		{
			name:      "stream",
			operation: "      x-ginx-sse: true\n",
			response: `        "200":
          description: stream
          content:
            text/event-stream:
              schema:
                type: string`,
			want: "does not support streaming responses",
		},
		{
			name: "non json",
			response: `        "200":
          description: text
          content:
            text/plain:
              schema:
                type: string`,
			want: "supports only application/json or no body",
		},
		{
			name: "typed headers",
			response: `        "201":
          description: created
          headers:
            X-Request-ID:
              schema:
                type: string`,
			want: "does not support typed response headers",
		},
		{
			name: "304 body",
			response: `        "304":
          description: not modified
          content:
            application/json:
              schema:
                type: object`,
			want: "response 304 must not declare a body",
		},
		{
			name: "default",
			response: `        default:
          description: fallback`,
			want: "does not support wildcard or default response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "variants.yaml")
			spec := `openapi: 3.0.3
info:
  title: variants validation
  version: 1.0.0
paths:
  /jobs:
    post:
      operationId: createJob
      x-ginx-response-mode: variants
` + tt.operation + `      responses:
` + tt.response + "\n"
			if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
				t.Fatalf("write spec: %v", err)
			}
			_, err := GenerateMulti(Config{PackageName: "api", SpecPath: path, OutputOptions: OutputOptions{SkipFmt: true}})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("GenerateMulti error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestIsBinaryContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"application/octet-stream", true},
		{"application/pdf", true},
		{"application/zip", true},
		{"image/png", true},
		{"audio/mpeg", true},
		{"video/mp4", true},
		{"application/json", false},
		{"application/xml", false},
		{"application/x-www-form-urlencoded", false},
		{"application/graphql", false},
		{"application/ld+json", false},
		{"application/merge-patch+json", false},
		{"application/problem+json", false},
		{"application/soap+xml", false},
		{"text/plain", false},
		{"text/html", false},
	}
	for _, tt := range tests {
		t.Run(tt.ct, func(t *testing.T) {
			got := isBinaryContentType(tt.ct)
			if got != tt.want {
				t.Errorf("isBinaryContentType(%q) = %v, want %v", tt.ct, got, tt.want)
			}
		})
	}
}
