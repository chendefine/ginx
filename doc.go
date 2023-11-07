package ginx

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iancoleman/strcase"
)

var (
	serveDoc      bool
	serviceName   = "http_service"
	namePkgPrefix = false
)

func SetNamePkgPrefix() {
	namePkgPrefix = true
}

func SetServeDoc(enable bool, name string) {
	serveDoc, serviceName = enable, name
}

func ServeDoc(router iRouter, pathx ...string) {
	if !serveDoc {
		return
	}

	var path = "/doc/pb"
	if len(pathx) > 0 {
		path = pathx[0]
	}

	serveDocProtobuf(router.Group(""), path)
}

func serveDocProtobuf(router iRouter, path string) {
	serve := func(c *gin.Context) {
		// c.Header("Content-Type", "application/protobuf")
		c.String(http.StatusOK, "%s", compileProtobuf())
	}
	router.GET(path, serve)
}

func compileProtobuf() string {
	if len(pbData) > 0 {
		return pbData
	}

	var b strings.Builder
	b.WriteString(`syntax = "proto3";`)
	b.WriteString("\n\n")
	b.WriteString("package ")
	b.WriteString(strcase.ToDelimited(serviceName, '_'))
	b.WriteString(";\n\n")
	b.WriteString(`import "google/protobuf/any.proto";`)
	b.WriteString("\n\n")

	for _, layout := range layouts {
		b.WriteString(layout.PbMessageString())
	}

	b.WriteString("service ")
	b.WriteString(strcase.ToCamel(serviceName))
	b.WriteString(" {\n")

	for _, service := range services {
		b.WriteString(service.PbMessageString())
	}
	b.WriteString("}\n")

	pbData = b.String()
	return pbData
}
