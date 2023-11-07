package responsetypes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chendefine/ginx"
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
	resp, err := http.Post(srv.URL+"/accepted-job", "application/json", nil)
	if err != nil {
		t.Fatalf("POST accepted-job: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("accepted-job status = %d, want 202", resp.StatusCode)
	}
}

func TestCreatedJSONResponse(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	rsp, err := client.CreateItem(context.Background(), &CreateItemReq{})
	if err != nil || rsp.ID != "item-1" {
		t.Fatalf("CreateItem response=%#v err=%v", rsp, err)
	}
	httpRsp, err := http.Post(srv.URL+"/created-item", "application/json", nil)
	if err != nil {
		t.Fatalf("POST created-item: %v", err)
	}
	defer httpRsp.Body.Close()
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(httpRsp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode created-item: %v", err)
	}
	if httpRsp.StatusCode != http.StatusCreated || envelope.Code != 0 || envelope.Data.ID != "item-1" {
		t.Fatalf("created-item status=%d body=%#v", httpRsp.StatusCode, envelope)
	}
}

func TestPartialFileDownload(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	data, err := client.DownloadPartial(context.Background(), &DownloadPartialReq{})
	if err != nil {
		t.Fatalf("DownloadPartial without Range: %v", err)
	}
	if string(data) != "binary-content" {
		t.Errorf("complete data = %q", string(data))
	}

	rangeHeader := "bytes=0-5"
	data, err = client.DownloadPartial(context.Background(), &DownloadPartialReq{Range: &rangeHeader})
	if err != nil {
		t.Fatalf("DownloadPartial: %v", err)
	}
	if string(data) != "binary" {
		t.Errorf("data = %q", string(data))
	}
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/partial-download", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Range", rangeHeader)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Range request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusPartialContent || resp.Header.Get("Content-Range") != "bytes 0-5/14" || string(body) != "binary" {
		t.Fatalf("range status=%d content-range=%q body=%q", resp.StatusCode, resp.Header.Get("Content-Range"), body)
	}
}

func TestHeadAndOptions(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	if err := client.HeadCheck(context.Background(), &HeadCheckReq{}); err != nil {
		t.Fatalf("HeadCheck: %v", err)
	}
	headResp, err := http.Head(srv.URL + "/head-check")
	if err != nil {
		t.Fatalf("HEAD request: %v", err)
	}
	defer headResp.Body.Close()
	headBody, _ := io.ReadAll(headResp.Body)
	if headResp.StatusCode != http.StatusNoContent || len(headBody) != 0 {
		t.Fatalf("HEAD status=%d body=%q", headResp.StatusCode, headBody)
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
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent || len(body) != 0 {
		t.Fatalf("OPTIONS status=%d body=%q", resp.StatusCode, body)
	}
}

func TestDeleteAndRedirectWireContracts(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/no-content", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || len(body) != 0 {
		t.Fatalf("DELETE status=%d body=%q", resp.StatusCode, body)
	}
	if err := client.RedirectToHome(context.Background(), &RedirectToHomeReq{}); err != nil {
		t.Fatalf("generated redirect client: %v", err)
	}
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err = noRedirect.Get(srv.URL + "/redirect")
	if err != nil {
		t.Fatalf("GET redirect: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "/home" {
		t.Fatalf("redirect status=%d location=%q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestClientRejectsUnexpectedSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"job_id":"wrong-status"}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL).CreateJob(context.Background(), &CreateJobReq{})
	var statusErr *ginx.UnexpectedStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusOK {
		t.Fatalf("CreateJob error = %#v, want UnexpectedStatusError(200)", err)
	}
}

func TestClientRejectsHTTPErrorWithSuccessEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":0,"msg":"invalid success envelope"}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL).CreateJob(context.Background(), &CreateJobReq{})
	var wrapped *ginx.ErrWrap
	if !errors.As(err, &wrapped) || wrapped.HttpCode != http.StatusInternalServerError || wrapped.Code != -1 {
		t.Fatalf("CreateJob error = %#v, want ErrWrap(code=-1, http=500)", err)
	}
}
