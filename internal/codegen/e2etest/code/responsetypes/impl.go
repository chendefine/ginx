package responsetypes

import (
	"context"
	"net/http"
	"os"

	"github.com/chendefine/ginx"
)

type TestService struct {
	filePath string
}

func NewTestService() *TestService {
	f, _ := os.CreateTemp("", "e2e-*.bin")
	f.Write([]byte("binary-content"))
	f.Close()
	return &TestService{filePath: f.Name()}
}

func (s *TestService) Cleanup() { os.Remove(s.filePath) }

func (s *TestService) GetJSONData(_ context.Context, _ *GetJSONDataReq) (*GetJSONDataRsp, error) {
	count := 42
	return &GetJSONDataRsp{Message: "hello", Count: &count}, nil
}

func (s *TestService) GetJSONRef(_ context.Context, _ *GetJSONRefReq) (*GetJSONRefRsp, error) {
	return &GetJSONRefRsp{ID: 1, Name: "Alice"}, nil
}

func (s *TestService) DownloadPdf(_ context.Context, _ *DownloadPdfReq) (*ginx.FileRsp, error) {
	return ginx.FileResponse(s.filePath, "doc.pdf"), nil
}

func (s *TestService) DownloadImage(_ context.Context, _ *DownloadImageReq) (*ginx.FileRsp, error) {
	return ginx.FileResponse(s.filePath, "img.png"), nil
}

func (s *TestService) DownloadAudio(_ context.Context, _ *DownloadAudioReq) (*ginx.FileRsp, error) {
	return ginx.FileResponse(s.filePath, "audio.mp3"), nil
}

func (s *TestService) DownloadVideo(_ context.Context, _ *DownloadVideoReq) (*ginx.FileRsp, error) {
	return ginx.FileResponse(s.filePath, "video.mp4"), nil
}

func (s *TestService) DownloadBinary(_ context.Context, _ *DownloadBinaryReq) (*ginx.FileRsp, error) {
	return ginx.FileResponse(s.filePath, "data.bin"), nil
}

func (s *TestService) DownloadZip(_ context.Context, _ *DownloadZipReq) (*ginx.FileRsp, error) {
	return ginx.FileResponse(s.filePath, "archive.zip"), nil
}

func (s *TestService) GetTextContent(_ context.Context, _ *GetTextContentReq) (*ginx.StringRsp, error) {
	return ginx.StringResponse(http.StatusOK, "plain text content"), nil
}

func (s *TestService) ExportCsv(_ context.Context, _ *ExportCsvReq) (*ginx.StringRsp, error) {
	return ginx.StringResponse(http.StatusOK, "id,name\n1,Alice"), nil
}

func (s *TestService) GetHTMLPage(_ context.Context, _ *GetHTMLPageReq) (*ginx.StringRsp, error) {
	return ginx.StringResponse(http.StatusOK, "<h1>Hello</h1>"), nil
}

func (s *TestService) RedirectToHome(_ context.Context, _ *RedirectToHomeReq) (*ginx.RedirectRsp, error) {
	return ginx.RedirectResponse(http.StatusMovedPermanently, "/home"), nil
}

func (s *TestService) DeleteItem(_ context.Context, _ *DeleteItemReq) (*struct{}, error) {
	return &struct{}{}, nil
}

func (s *TestService) GetWithMultipleTypes(_ context.Context, _ *GetWithMultipleTypesReq) (*GetWithMultipleTypesRsp, error) {
	data := "json wins"
	return &GetWithMultipleTypesRsp{Data: &data}, nil
}

func (s *TestService) GetEmpty(_ context.Context, _ *GetEmptyReq) (*struct{}, error) {
	return &struct{}{}, nil
}

var _ ServerInterface = (*TestService)(nil)
