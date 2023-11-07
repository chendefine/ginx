package ginx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type benchmarkRouteKind string

type benchmarkBodyKind string

const (
	benchmarkRouteFixed benchmarkRouteKind = "fixed"
	benchmarkRouteParam benchmarkRouteKind = "param"

	benchmarkBodySimple  benchmarkBodyKind = "simple"
	benchmarkBodyMedium  benchmarkBodyKind = "medium"
	benchmarkBodyComplex benchmarkBodyKind = "complex"
)

type benchmarkCase struct {
	name      string
	routeKind benchmarkRouteKind
	bodyKind  benchmarkBodyKind
	useQuery  bool
	builder   func(benchmarkCase) http.Handler
}

type benchmarkSuccessData struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Page int    `json:"page,omitempty"`
	OK   bool   `json:"ok"`
}

type benchmarkResult struct {
	Code int                  `json:"code"`
	Msg  string               `json:"msg"`
	Data benchmarkSuccessData `json:"data,omitempty"`
}

type benchmarkSimpleBody struct {
	Name   string  `json:"name" binding:"required"`
	Age    int     `json:"age" binding:"required"`
	Email  string  `json:"email" binding:"required,email"`
	Active bool    `json:"active"`
	Score  float64 `json:"score"`
}

type benchmarkMediumProfile struct {
	City    string `json:"city"`
	Country string `json:"country"`
	ZipCode string `json:"zipCode"`
}

type benchmarkMediumBody struct {
	Name      string                 `json:"name" binding:"required"`
	Age       int                    `json:"age" binding:"required"`
	Email     string                 `json:"email" binding:"required,email"`
	Active    bool                   `json:"active"`
	Score     float64                `json:"score"`
	Phone     string                 `json:"phone"`
	Title     string                 `json:"title"`
	Dept      string                 `json:"dept"`
	Level     int                    `json:"level"`
	Visits    int                    `json:"visits"`
	Profile   benchmarkMediumProfile `json:"profile"`
	Tags      []string               `json:"tags"`
	Marketing bool                   `json:"marketing"`
}

type benchmarkComplexAddress struct {
	Type   string `json:"type"`
	City   string `json:"city"`
	Street string `json:"street"`
	Zip    string `json:"zip"`
}

type benchmarkComplexOrderItem struct {
	SKU      string `json:"sku"`
	Qty      int    `json:"qty"`
	Price    int    `json:"price"`
	Discount int    `json:"discount"`
}

type benchmarkComplexOrder struct {
	ID    string                      `json:"id"`
	Items []benchmarkComplexOrderItem `json:"items"`
}

type benchmarkComplexPreferences struct {
	Language string   `json:"language"`
	Theme    string   `json:"theme"`
	Channels []string `json:"channels"`
}

type benchmarkComplexMetadata struct {
	Source  string `json:"source"`
	Version int    `json:"version"`
}

type benchmarkComplexBody struct {
	Name        string                      `json:"name" binding:"required"`
	Age         int                         `json:"age" binding:"required"`
	Email       string                      `json:"email" binding:"required,email"`
	Active      bool                        `json:"active"`
	Score       float64                     `json:"score"`
	Phone       string                      `json:"phone"`
	Title       string                      `json:"title"`
	Dept        string                      `json:"dept"`
	Level       int                         `json:"level"`
	Visits      int                         `json:"visits"`
	Labels      []string                    `json:"labels"`
	Profile     benchmarkMediumProfile      `json:"profile"`
	Preferences benchmarkComplexPreferences `json:"preferences"`
	Metadata    benchmarkComplexMetadata    `json:"metadata"`
	Addresses   []benchmarkComplexAddress   `json:"addresses"`
	Orders      []benchmarkComplexOrder     `json:"orders"`
}

type benchmarkFixedSimpleReq struct {
	Name   string  `json:"name" binding:"required"`
	Age    int     `json:"age" binding:"required"`
	Email  string  `json:"email" binding:"required,email"`
	Active bool    `json:"active"`
	Score  float64 `json:"score"`
}

type benchmarkFixedMediumReq struct {
	Name      string                 `json:"name" binding:"required"`
	Age       int                    `json:"age" binding:"required"`
	Email     string                 `json:"email" binding:"required,email"`
	Active    bool                   `json:"active"`
	Score     float64                `json:"score"`
	Phone     string                 `json:"phone"`
	Title     string                 `json:"title"`
	Dept      string                 `json:"dept"`
	Level     int                    `json:"level"`
	Visits    int                    `json:"visits"`
	Profile   benchmarkMediumProfile `json:"profile"`
	Tags      []string               `json:"tags"`
	Marketing bool                   `json:"marketing"`
}

type benchmarkFixedComplexReq struct {
	Name        string                      `json:"name" binding:"required"`
	Age         int                         `json:"age" binding:"required"`
	Email       string                      `json:"email" binding:"required,email"`
	Active      bool                        `json:"active"`
	Score       float64                     `json:"score"`
	Phone       string                      `json:"phone"`
	Title       string                      `json:"title"`
	Dept        string                      `json:"dept"`
	Level       int                         `json:"level"`
	Visits      int                         `json:"visits"`
	Labels      []string                    `json:"labels"`
	Profile     benchmarkMediumProfile      `json:"profile"`
	Preferences benchmarkComplexPreferences `json:"preferences"`
	Metadata    benchmarkComplexMetadata    `json:"metadata"`
	Addresses   []benchmarkComplexAddress   `json:"addresses"`
	Orders      []benchmarkComplexOrder     `json:"orders"`
}

type benchmarkParamSimpleReq struct {
	ID     string  `uri:"id" binding:"required"`
	Page   int     `form:"page" binding:"required,gt=0"`
	Size   int     `form:"size" binding:"required,gt=0"`
	Name   string  `json:"name" binding:"required"`
	Age    int     `json:"age" binding:"required"`
	Email  string  `json:"email" binding:"required,email"`
	Active bool    `json:"active"`
	Score  float64 `json:"score"`
}

type benchmarkParamMediumReq struct {
	ID        string                 `uri:"id" binding:"required"`
	Page      int                    `form:"page" binding:"required,gt=0"`
	Size      int                    `form:"size" binding:"required,gt=0"`
	Name      string                 `json:"name" binding:"required"`
	Age       int                    `json:"age" binding:"required"`
	Email     string                 `json:"email" binding:"required,email"`
	Active    bool                   `json:"active"`
	Score     float64                `json:"score"`
	Phone     string                 `json:"phone"`
	Title     string                 `json:"title"`
	Dept      string                 `json:"dept"`
	Level     int                    `json:"level"`
	Visits    int                    `json:"visits"`
	Profile   benchmarkMediumProfile `json:"profile"`
	Tags      []string               `json:"tags"`
	Marketing bool                   `json:"marketing"`
}

type benchmarkParamComplexReq struct {
	ID          string                      `uri:"id" binding:"required"`
	Page        int                         `form:"page" binding:"required,gt=0"`
	Size        int                         `form:"size" binding:"required,gt=0"`
	Name        string                      `json:"name" binding:"required"`
	Age         int                         `json:"age" binding:"required"`
	Email       string                      `json:"email" binding:"required,email"`
	Active      bool                        `json:"active"`
	Score       float64                     `json:"score"`
	Phone       string                      `json:"phone"`
	Title       string                      `json:"title"`
	Dept        string                      `json:"dept"`
	Level       int                         `json:"level"`
	Visits      int                         `json:"visits"`
	Labels      []string                    `json:"labels"`
	Profile     benchmarkMediumProfile      `json:"profile"`
	Preferences benchmarkComplexPreferences `json:"preferences"`
	Metadata    benchmarkComplexMetadata    `json:"metadata"`
	Addresses   []benchmarkComplexAddress   `json:"addresses"`
	Orders      []benchmarkComplexOrder     `json:"orders"`
}

var benchmarkSimpleJSON = []byte(`{"name":"alice","age":18,"email":"alice@example.com","active":true,"score":9.5}`)
var benchmarkMediumJSON = []byte(`{"name":"alice","age":18,"email":"alice@example.com","active":true,"score":9.5,"phone":"13800000000","title":"engineer","dept":"platform","level":3,"visits":12,"profile":{"city":"shanghai","country":"CN","zipCode":"200000"},"tags":["core","api","benchmark"],"marketing":true}`)
var benchmarkComplexJSON = []byte(`{"name":"alice","age":18,"email":"alice@example.com","active":true,"score":9.5,"phone":"13800000000","title":"engineer","dept":"platform","level":3,"visits":12,"labels":["core","api","benchmark","critical"],"profile":{"city":"shanghai","country":"CN","zipCode":"200000"},"preferences":{"language":"zh-CN","theme":"dark","channels":["email","sms","push"]},"metadata":{"source":"load-test","version":2},"addresses":[{"type":"home","city":"shanghai","street":"road 1","zip":"200000"},{"type":"office","city":"beijing","street":"road 2","zip":"100000"}],"orders":[{"id":"o-1","items":[{"sku":"sku-1","qty":1,"price":100,"discount":5},{"sku":"sku-2","qty":2,"price":50,"discount":0}]},{"id":"o-2","items":[{"sku":"sku-3","qty":1,"price":88,"discount":8}]}]}`)

func TestBenchmarkPayloadsAreValidJSON(t *testing.T) {
	for _, raw := range [][]byte{
		benchmarkSimpleJSON,
		benchmarkMediumJSON,
		benchmarkComplexJSON,
	} {
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
	}
}

func TestBenchmarkGinNativeSuccessShape(t *testing.T) {
	router := buildBenchmarkGinRouter(benchmarkCase{
		routeKind: benchmarkRouteParam,
		bodyKind:  benchmarkBodySimple,
		useQuery:  true,
	})

	req := httptest.NewRequest(http.MethodPost, "/bench/users/u-1?page=1&size=20", bytes.NewReader(benchmarkSimpleJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if body["code"].(float64) != 0 {
		t.Fatalf("code=%v", body["code"])
	}
	if body["msg"].(string) != "" {
		t.Fatalf("msg=%q", body["msg"])
	}
	data := body["data"].(map[string]any)
	if data["id"].(string) != "u-1" {
		t.Fatalf("id=%v", data["id"])
	}
	if data["name"].(string) != "alice" {
		t.Fatalf("name=%v", data["name"])
	}
	if data["page"].(float64) != 1 {
		t.Fatalf("page=%v", data["page"])
	}
	if data["ok"].(bool) != true {
		t.Fatalf("ok=%v", data["ok"])
	}
}

func TestBenchmarkGinNativeBindErrorShape(t *testing.T) {
	router := buildBenchmarkGinRouter(benchmarkCase{
		routeKind: benchmarkRouteParam,
		bodyKind:  benchmarkBodySimple,
		useQuery:  true,
	})

	req := httptest.NewRequest(http.MethodPost, "/bench/users/u-1?page=0&size=20", bytes.NewReader([]byte(`{"name":"alice","age":18,"active":true,"score":9.5}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if body["code"].(float64) != 1 {
		t.Fatalf("code=%v", body["code"])
	}
	if _, ok := body["msg"].(string); !ok {
		t.Fatalf("msg=%T", body["msg"])
	}
}

func buildBenchmarkGinRouter(bc benchmarkCase) http.Handler {
	r := gin.New()
	path := "/bench"
	if bc.routeKind == benchmarkRouteParam {
		path = "/bench/users/:id"
	}

	r.POST(path, func(c *gin.Context) {
		switch {
		case bc.routeKind == benchmarkRouteFixed && bc.bodyKind == benchmarkBodySimple:
			var req benchmarkFixedSimpleReq
			handleBenchmarkGin(c, bc, &req, func() benchmarkSuccessData {
				return benchmarkSuccessData{Name: req.Name, OK: true}
			})
		case bc.routeKind == benchmarkRouteFixed && bc.bodyKind == benchmarkBodyMedium:
			var req benchmarkFixedMediumReq
			handleBenchmarkGin(c, bc, &req, func() benchmarkSuccessData {
				return benchmarkSuccessData{Name: req.Name, OK: true}
			})
		case bc.routeKind == benchmarkRouteFixed && bc.bodyKind == benchmarkBodyComplex:
			var req benchmarkFixedComplexReq
			handleBenchmarkGin(c, bc, &req, func() benchmarkSuccessData {
				return benchmarkSuccessData{Name: req.Name, OK: true}
			})
		case bc.routeKind == benchmarkRouteParam && bc.bodyKind == benchmarkBodySimple:
			var req benchmarkParamSimpleReq
			handleBenchmarkGin(c, bc, &req, func() benchmarkSuccessData {
				return benchmarkSuccessData{ID: req.ID, Name: req.Name, Page: req.Page, OK: true}
			})
		case bc.routeKind == benchmarkRouteParam && bc.bodyKind == benchmarkBodyMedium:
			var req benchmarkParamMediumReq
			handleBenchmarkGin(c, bc, &req, func() benchmarkSuccessData {
				return benchmarkSuccessData{ID: req.ID, Name: req.Name, Page: req.Page, OK: true}
			})
		case bc.routeKind == benchmarkRouteParam && bc.bodyKind == benchmarkBodyComplex:
			var req benchmarkParamComplexReq
			handleBenchmarkGin(c, bc, &req, func() benchmarkSuccessData {
				return benchmarkSuccessData{ID: req.ID, Name: req.Name, Page: req.Page, OK: true}
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 2, "msg": "unsupported benchmark case"})
		}
	})

	return r
}

func handleBenchmarkGin[T any](c *gin.Context, bc benchmarkCase, req *T, buildResult func() benchmarkSuccessData) {
	if bc.routeKind == benchmarkRouteParam {
		if err := c.ShouldBindUri(req); err != nil && !isValidationError(err) {
			writeBenchmarkBindError(c, err)
			return
		}
	}
	if bc.useQuery {
		if err := c.ShouldBindQuery(req); err != nil && !isValidationError(err) {
			writeBenchmarkBindError(c, err)
			return
		}
	}
	if err := c.ShouldBindJSON(req); err != nil && !isValidationError(err) {
		writeBenchmarkBindError(c, err)
		return
	}
	if err := binding.Validator.ValidateStruct(req); err != nil {
		writeBenchmarkBindError(c, err)
		return
	}

	c.JSON(http.StatusOK, benchmarkResult{Code: 0, Msg: "", Data: buildResult()})
}

func writeBenchmarkBindError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code": 1,
		"msg":  sanitizeValidationError(err, nil),
	})
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func TestBenchmarkGinxSuccessShape(t *testing.T) {
	router := buildBenchmarkGinxRouter(benchmarkCase{
		routeKind: benchmarkRouteParam,
		bodyKind:  benchmarkBodySimple,
		useQuery:  true,
	})

	req := httptest.NewRequest(http.MethodPost, "/bench/users/u-1?page=1&size=20", bytes.NewReader(benchmarkSimpleJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if body["code"].(float64) != 0 {
		t.Fatalf("code=%v", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["id"].(string) != "u-1" {
		t.Fatalf("id=%v", data["id"])
	}
	if data["name"].(string) != "alice" {
		t.Fatalf("name=%v", data["name"])
	}
	if data["page"].(float64) != 1 {
		t.Fatalf("page=%v", data["page"])
	}
	if data["ok"].(bool) != true {
		t.Fatalf("ok=%v", data["ok"])
	}
}

func TestBenchmarkRequestForCase(t *testing.T) {
	req := newBenchmarkRequest(benchmarkCase{
		routeKind: benchmarkRouteParam,
		bodyKind:  benchmarkBodyMedium,
		useQuery:  true,
	})

	if req.URL.Path != "/bench/users/u-100" {
		t.Fatalf("path=%s", req.URL.Path)
	}
	if req.URL.RawQuery != "page=1&size=20" {
		t.Fatalf("query=%s", req.URL.RawQuery)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type=%q", got)
	}
}

func TestBenchmarkCasesCount(t *testing.T) {
	cases := benchmarkCases()
	if len(cases) != 12 {
		t.Fatalf("cases=%d", len(cases))
	}
}

func buildBenchmarkGinxRouter(bc benchmarkCase) http.Handler {
	r := gin.New()
	switch {
	case bc.routeKind == benchmarkRouteFixed && bc.bodyKind == benchmarkBodySimple:
		POST(r, "/bench", func(ctx context.Context, req *benchmarkFixedSimpleReq) (*benchmarkSuccessData, error) {
			return &benchmarkSuccessData{Name: req.Name, OK: true}, nil
		})
	case bc.routeKind == benchmarkRouteFixed && bc.bodyKind == benchmarkBodyMedium:
		POST(r, "/bench", func(ctx context.Context, req *benchmarkFixedMediumReq) (*benchmarkSuccessData, error) {
			return &benchmarkSuccessData{Name: req.Name, OK: true}, nil
		})
	case bc.routeKind == benchmarkRouteFixed && bc.bodyKind == benchmarkBodyComplex:
		POST(r, "/bench", func(ctx context.Context, req *benchmarkFixedComplexReq) (*benchmarkSuccessData, error) {
			return &benchmarkSuccessData{Name: req.Name, OK: true}, nil
		})
	case bc.routeKind == benchmarkRouteParam && bc.bodyKind == benchmarkBodySimple:
		POST(r, "/bench/users/:id", func(ctx context.Context, req *benchmarkParamSimpleReq) (*benchmarkSuccessData, error) {
			return &benchmarkSuccessData{ID: req.ID, Name: req.Name, Page: req.Page, OK: true}, nil
		})
	case bc.routeKind == benchmarkRouteParam && bc.bodyKind == benchmarkBodyMedium:
		POST(r, "/bench/users/:id", func(ctx context.Context, req *benchmarkParamMediumReq) (*benchmarkSuccessData, error) {
			return &benchmarkSuccessData{ID: req.ID, Name: req.Name, Page: req.Page, OK: true}, nil
		})
	case bc.routeKind == benchmarkRouteParam && bc.bodyKind == benchmarkBodyComplex:
		POST(r, "/bench/users/:id", func(ctx context.Context, req *benchmarkParamComplexReq) (*benchmarkSuccessData, error) {
			return &benchmarkSuccessData{ID: req.ID, Name: req.Name, Page: req.Page, OK: true}, nil
		})
	default:
		panic("unsupported benchmark case")
	}
	return r
}

func payloadForBody(kind benchmarkBodyKind) []byte {
	switch kind {
	case benchmarkBodySimple:
		return benchmarkSimpleJSON
	case benchmarkBodyMedium:
		return benchmarkMediumJSON
	case benchmarkBodyComplex:
		return benchmarkComplexJSON
	default:
		panic("unsupported benchmark body kind")
	}
}

func newBenchmarkRequest(bc benchmarkCase) *http.Request {
	path := "/bench"
	if bc.routeKind == benchmarkRouteParam {
		path = "/bench/users/u-100"
	}
	if bc.useQuery {
		path += "?page=1&size=20"
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payloadForBody(bc.bodyKind)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func benchmarkCases() []benchmarkCase {
	return []benchmarkCase{
		{name: "Ginx_FixedPath_NoQuery_SimpleJSON", routeKind: benchmarkRouteFixed, bodyKind: benchmarkBodySimple, useQuery: false, builder: buildBenchmarkGinxRouter},
		{name: "Gin_FixedPath_NoQuery_SimpleJSON", routeKind: benchmarkRouteFixed, bodyKind: benchmarkBodySimple, useQuery: false, builder: buildBenchmarkGinRouter},
		{name: "Ginx_FixedPath_NoQuery_MediumJSON", routeKind: benchmarkRouteFixed, bodyKind: benchmarkBodyMedium, useQuery: false, builder: buildBenchmarkGinxRouter},
		{name: "Gin_FixedPath_NoQuery_MediumJSON", routeKind: benchmarkRouteFixed, bodyKind: benchmarkBodyMedium, useQuery: false, builder: buildBenchmarkGinRouter},
		{name: "Ginx_FixedPath_NoQuery_ComplexJSON", routeKind: benchmarkRouteFixed, bodyKind: benchmarkBodyComplex, useQuery: false, builder: buildBenchmarkGinxRouter},
		{name: "Gin_FixedPath_NoQuery_ComplexJSON", routeKind: benchmarkRouteFixed, bodyKind: benchmarkBodyComplex, useQuery: false, builder: buildBenchmarkGinRouter},
		{name: "Ginx_PathParam_Query_SimpleJSON", routeKind: benchmarkRouteParam, bodyKind: benchmarkBodySimple, useQuery: true, builder: buildBenchmarkGinxRouter},
		{name: "Gin_PathParam_Query_SimpleJSON", routeKind: benchmarkRouteParam, bodyKind: benchmarkBodySimple, useQuery: true, builder: buildBenchmarkGinRouter},
		{name: "Ginx_PathParam_Query_MediumJSON", routeKind: benchmarkRouteParam, bodyKind: benchmarkBodyMedium, useQuery: true, builder: buildBenchmarkGinxRouter},
		{name: "Gin_PathParam_Query_MediumJSON", routeKind: benchmarkRouteParam, bodyKind: benchmarkBodyMedium, useQuery: true, builder: buildBenchmarkGinRouter},
		{name: "Ginx_PathParam_Query_ComplexJSON", routeKind: benchmarkRouteParam, bodyKind: benchmarkBodyComplex, useQuery: true, builder: buildBenchmarkGinxRouter},
		{name: "Gin_PathParam_Query_ComplexJSON", routeKind: benchmarkRouteParam, bodyKind: benchmarkBodyComplex, useQuery: true, builder: buildBenchmarkGinRouter},
	}
}

func benchmarkHTTP(b *testing.B, bc benchmarkCase) {
	b.Helper()
	if bc.builder == nil {
		b.Skip("benchmark builder not set")
	}
	router := bc.builder(bc)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := newBenchmarkRequest(bc)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkHTTP(b *testing.B) {
	for _, bc := range benchmarkCases() {
		bc := bc
		b.Run(bc.name, func(b *testing.B) {
			benchmarkHTTP(b, bc)
		})
	}
}

func BenchmarkHTTP_Ginx_FixedPath_NoQuery_SimpleJSON(b *testing.B) {
	benchmarkHTTP(b, benchmarkCase{
		name:      "Ginx_FixedPath_NoQuery_SimpleJSON",
		routeKind: benchmarkRouteFixed,
		bodyKind:  benchmarkBodySimple,
		useQuery:  false,
		builder:   buildBenchmarkGinxRouter,
	})
}

func init() {
	gin.SetMode(gin.TestMode)
}
