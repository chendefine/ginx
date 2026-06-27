package responsetypes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer() (*httptest.Server, *Client, *TestService) {
	svc := NewTestService()
	r := gin.New()
	RegisterRoutes(r, svc)
	srv := httptest.NewServer(r)
	return srv, NewClient(srv.URL), svc
}

func TestJSONResponse(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	rsp, err := client.GetJSONData(context.Background(), &GetJSONDataReq{})
	if err != nil {
		t.Fatalf("GetJSONData: %v", err)
	}
	if rsp.Message != "hello" {
		t.Errorf("Message = %q, want hello", rsp.Message)
	}
	if rsp.Count == nil || *rsp.Count != 42 {
		t.Errorf("Count = %v, want 42", rsp.Count)
	}
}

func TestJSONRefResponse(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	rsp, err := client.GetJSONRef(context.Background(), &GetJSONRefReq{})
	if err != nil {
		t.Fatalf("GetJSONRef: %v", err)
	}
	if rsp.ID != 1 {
		t.Errorf("ID = %d, want 1", rsp.ID)
	}
	if rsp.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", rsp.Name)
	}
}

func TestFileDownload_PDF(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	data, err := client.DownloadPdf(context.Background(), &DownloadPdfReq{})
	if err != nil {
		t.Fatalf("DownloadPdf: %v", err)
	}
	if string(data) != "binary-content" {
		t.Errorf("data = %q", string(data))
	}
}

func TestFileDownload_Image(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	data, err := client.DownloadImage(context.Background(), &DownloadImageReq{})
	if err != nil {
		t.Fatalf("DownloadImage: %v", err)
	}
	if string(data) != "binary-content" {
		t.Errorf("data = %q", string(data))
	}
}

func TestFileDownload_Audio(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	data, err := client.DownloadAudio(context.Background(), &DownloadAudioReq{})
	if err != nil {
		t.Fatalf("DownloadAudio: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestFileDownload_Video(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	data, err := client.DownloadVideo(context.Background(), &DownloadVideoReq{})
	if err != nil {
		t.Fatalf("DownloadVideo: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestFileDownload_Zip(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	data, err := client.DownloadZip(context.Background(), &DownloadZipReq{})
	if err != nil {
		t.Fatalf("DownloadZip: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestStringResponse_Text(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	text, err := client.GetTextContent(context.Background(), &GetTextContentReq{})
	if err != nil {
		t.Fatalf("GetTextContent: %v", err)
	}
	if text != "plain text content" {
		t.Errorf("text = %q", text)
	}
}

func TestStringResponse_CSV(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	csv, err := client.ExportCsv(context.Background(), &ExportCsvReq{})
	if err != nil {
		t.Fatalf("ExportCsv: %v", err)
	}
	if csv != "id,name\n1,Alice" {
		t.Errorf("csv = %q", csv)
	}
}

func TestStringResponse_HTML(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	html, err := client.GetHTMLPage(context.Background(), &GetHTMLPageReq{})
	if err != nil {
		t.Fatalf("GetHTMLPage: %v", err)
	}
	if html != "<h1>Hello</h1>" {
		t.Errorf("html = %q", html)
	}
}

func TestDeleteNoContent(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	err := client.DeleteItem(context.Background(), &DeleteItemReq{})
	if err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
}

func TestJSONPriority(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	rsp, err := client.GetWithMultipleTypes(context.Background(), &GetWithMultipleTypesReq{})
	if err != nil {
		t.Fatalf("GetWithMultipleTypes: %v", err)
	}
	if rsp.Data == nil || *rsp.Data != "json wins" {
		t.Errorf("Data = %v", rsp.Data)
	}
}

func TestEmptyResponse(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	err := client.GetEmpty(context.Background(), &GetEmptyReq{})
	if err != nil {
		t.Fatalf("GetEmpty: %v", err)
	}
}

func TestAcceptedJSONResponse(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	rsp, err := client.CreateJob(context.Background(), &CreateJobReq{})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if rsp.JobID != "job-1" {
		t.Errorf("JobID = %q, want job-1", rsp.JobID)
	}
}

func TestPartialFileDownload(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	data, err := client.DownloadPartial(context.Background(), &DownloadPartialReq{})
	if err != nil {
		t.Fatalf("DownloadPartial: %v", err)
	}
	if string(data) != "binary-content" {
		t.Errorf("data = %q", string(data))
	}
}

func TestHeadAndOptions(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	if err := client.HeadCheck(context.Background(), &HeadCheckReq{}); err != nil {
		t.Fatalf("HeadCheck: %v", err)
	}
	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/options-check", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("OPTIONS status = %d, want 200", resp.StatusCode)
	}
}
