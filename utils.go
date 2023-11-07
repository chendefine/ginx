package ginx

import (
	"path"
	"reflect"
	"runtime"
	"strings"
)

// copy from gin
func lastChar(str string) uint8 {
	if str == "" {
		panic("The length of the string can't be 0")
	}
	return str[len(str)-1]
}

// copy from gin
func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}

	finalPath := path.Join(absolutePath, relativePath)
	if lastChar(relativePath) == '/' && lastChar(finalPath) != '/' {
		return finalPath + "/"
	}
	return finalPath
}

func getFuncName(fn any) string {
	fullName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	fullNameSplit := strings.Split(fullName, "/")
	pkgFunc := fullNameSplit[len(fullNameSplit)-1]
	pkgExist[strings.Split(pkgFunc, ".")[0]] = markExist

	return pkgFunc
}
