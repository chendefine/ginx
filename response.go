package ginx

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 非 JSON 响应接口. handler 若返回实现此接口的类型, ginx 跳过默认 JSON 包装,
// 直接调用 WriteTo 让业务写入响应. 返回的 error 会被记录到 gin 的 Errors 中.
type Response interface {
	WriteTo(c *gin.Context) error
}

// --- File ---

// FileRsp 文件下载响应.
type FileRsp struct {
	FilePath string
	FileName string
}

// FileResponse 构造文件下载响应.
func FileResponse(filePath, fileName string) *FileRsp {
	return &FileRsp{FilePath: filePath, FileName: fileName}
}

// WriteTo 实现 Response.
func (r *FileRsp) WriteTo(c *gin.Context) error {
	c.FileAttachment(r.FilePath, r.FileName)
	return nil
}

// --- Redirect ---

// RedirectRsp 重定向响应.
type RedirectRsp struct {
	Code     int
	Location string
}

// RedirectResponse 构造重定向响应.
func RedirectResponse(code int, location string) *RedirectRsp {
	code = normalizeHTTPStatus(code, http.StatusFound)
	return &RedirectRsp{Code: code, Location: location}
}

// WriteTo 实现 Response.
func (r *RedirectRsp) WriteTo(c *gin.Context) error {
	c.Redirect(r.Code, r.Location)
	return nil
}

// --- String ---

// StringRsp 纯文本响应, 构造时即完成 format, 避免写入阶段二次格式化.
type StringRsp struct {
	Code int
	Body string
}

// StringResponse 构造纯文本响应. body/args 会在这里就完成 Sprintf.
func StringResponse(code int, body string, args ...any) *StringRsp {
	code = normalizeHTTPStatus(code, http.StatusOK)
	if len(args) > 0 {
		body = fmt.Sprintf(body, args...)
	}
	return &StringRsp{Code: code, Body: body}
}

// WriteTo 实现 Response.
func (r *StringRsp) WriteTo(c *gin.Context) error {
	c.String(r.Code, "%s", r.Body)
	return nil
}

// --- Data (raw bytes) ---

// DataRsp 原始字节响应.
type DataRsp struct {
	Code        int
	ContentType string
	Data        []byte
}

// DataResponse 构造原始字节响应.
func DataResponse(code int, contentType string, data []byte) *DataRsp {
	code = normalizeHTTPStatus(code, http.StatusOK)
	return &DataRsp{Code: code, ContentType: contentType, Data: data}
}

// WriteTo 实现 Response.
func (r *DataRsp) WriteTo(c *gin.Context) error {
	c.Data(r.Code, r.ContentType, r.Data)
	return nil
}

func normalizeHTTPStatus(code, fallback int) int {
	if code >= 100 && code <= 599 {
		return code
	}
	return fallback
}
