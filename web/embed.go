// Package web 以 go:embed 内嵌前端构建产物并提供静态伺服。
//
// 硬约束：//go:embed 不能引用包目录之外的文件，故 embed.go 必须与 dist/ 同级（放 web/）。
// 仓库提交 web/dist/index.html 占位，保证裸 clone 后 go build ./... / go test ./... 可编译；
// 真实产物由 pnpm -C web build 生成（构建顺序：pnpm 先于 go build，D-18）。
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler 返回伺服内嵌 dist 的 http.Handler。
// 空路径回落 index.html；Accept-Encoding 含 gzip 且存在 .gz 旁路时直发预压缩体
// （构建期 gzip -k -9 产出，运行时零压缩开销），否则明文伺服。
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(dist, "dist") // 剥掉 dist/ 前缀
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			if data, err := fs.ReadFile(sub, name+".gz"); err == nil {
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(data)
				return
			}
		}
		http.FileServerFS(sub).ServeHTTP(w, r)
	}), nil
}
