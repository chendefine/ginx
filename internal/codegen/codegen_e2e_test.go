package codegen

import (
	"go/format"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "e2etest", "spec", name)
}

func generateSingleFile(t *testing.T, specFile string, opts ...func(*Config)) string {
	t.Helper()
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath(specFile),
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

func generateMultiFile(t *testing.T, specFile string, opts ...func(*Config)) *GenerateResult {
	t.Helper()
	cfg := Config{
		PackageName: "api",
		SpecPath:    testdataPath(specFile),
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
	assertContains(t, server, "RedirectToHome(ctx context.Context, req *RedirectToHomeReq) (*ginx.RedirectRsp, error)")
}

func TestE2E_ResponseTypes_NoContent(t *testing.T) {
	result := generateMultiFile(t, "response_types.yaml")
	server := string(result.Server)
	assertContains(t, server, "DeleteItem(ctx context.Context, req *DeleteItemReq) (*struct{}, error)")
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
	assertContains(t, server, `ginx.POST(r, "/pets", s.CreatePet, opts...)`)
	assertContains(t, server, `ginx.GET(r, "/pets/:pet_id", s.GetPet, opts...)`)
	assertContains(t, server, `ginx.DELETE(r, "/pets/:pet_id", s.DeletePet, opts...)`)
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

func TestE2E_Client_HeaderParams(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, `r.SetHeader("X-Tenant-ID"`)
	assertContains(t, client, `r.SetHeader("X-Idempotency-Key"`)
}

func TestE2E_Client_CookieParams(t *testing.T) {
	result := generateMultiFile(t, "request_params.yaml")
	client := string(result.Client)

	assertContains(t, client, `r.SetCookie(&http.Cookie{Name: "sid"`)
}

func TestE2E_Client_BodyEmbed(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertContains(t, client, "r.SetBody(&req.CreateItemInput)")
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

func TestE2E_Client_SkipMultipart(t *testing.T) {
	result := generateMultiFile(t, "client_sdk.yaml")
	client := string(result.Client)

	assertNotContains(t, client, "UploadItemImage")
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

	assertContains(t, client, `strings.Replace(sseURL, "{room_id}", req.RoomID, 1)`)
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

func TestE2E_ClientImportSkipsCookieOnlyMultipartOperation(t *testing.T) {
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

	result, err := GenerateMulti(Config{
		PackageName: "api",
		SpecPath:    specPath,
		Output:      OutputConfig{Types: "t.go", Client: "c.go"},
		OutputOptions: OutputOptions{
			SkipFmt: true,
		},
	})
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}

	client := string(result.Client)
	assertNotContains(t, client, `"net/http"`)
	assertNotContains(t, client, "SetCookie")
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
	assertContains(t, server, `ginx.HEAD(r, "/ping", s.HeadPing, opts...)`)
	assertContains(t, server, `ginx.OPTIONS(r, "/ping", s.OptionsPing, opts...)`)
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
