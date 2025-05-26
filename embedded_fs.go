package zapm

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var webContent embed.FS

// 获取嵌入式文件系统
func getEmbeddedFileSystem() http.FileSystem {
	// 由于嵌入的路径包含"web"前缀，我们需要创建一个子文件系统
	subFS, err := fs.Sub(webContent, "web")
	if err != nil {
		panic(err)
	}
	return http.FS(subFS)
}
