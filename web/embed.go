// Package web 以 go:embed 内嵌前端构建产物并提供静态伺服。
//
// 硬约束：//go:embed 不能引用包目录之外的文件，故 embed.go 必须与 dist/ 同级（放 web/）。
// 仓库提交 web/dist/index.html 占位，保证裸 clone 后 go build ./... / go test ./... 可编译；
// 真实产物由 pnpm -C web build 生成（构建顺序：pnpm 先于 go build，D-18）。
package web

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler 返回伺服内嵌 dist 的 http.Handler。
// 空路径回落 index.html；客户端接受 gzip 且存在 .gz 旁路时直发预压缩体
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
		// 同一 URL 对 gzip/非 gzip 客户端返回不同实体编码，两种表示都要带
		// Vary——否则中间缓存键不完整，可能把压缩体发给不接受 gzip 的客户端。
		w.Header().Set("Vary", "Accept-Encoding")
		if acceptsGzip(r.Header.Get("Accept-Encoding")) {
			if data, err := fs.ReadFile(sub, name+".gz"); err == nil {
				w.Header().Set("Content-Encoding", "gzip")
				// .gz 旁路对任意类型资源生效，按扩展名推断真实 MIME
				// （硬编码 text/html 会让 JS/CSS 被按文本解析）。
				if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				w.Write(data)
				return
			}
		}
		http.FileServerFS(sub).ServeHTTP(w, r)
	}), nil
}

// acceptsGzip 按 token 解析 Accept-Encoding，仅当 gzip 显式声明且 q>0 时为真。
// strings.Contains 整头匹配会把 "gzip;q=0"（显式拒绝）与 "x-gzip" 误判为接受。
// codings 大小写不敏感（RFC 9110 §12.5.3，与 headerHasToken 的 EqualFold 同惯例）。
// 参数畸形/解析失败按拒绝处理——保守回落明文伺服，语义安全。
func acceptsGzip(h string) bool {
	for _, t := range strings.Split(h, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(t), ";")
		if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
			continue
		}
		for _, p := range strings.Split(params, ";") {
			if k, v, ok := strings.Cut(strings.TrimSpace(p), "="); ok && k == "q" {
				f, err := strconv.ParseFloat(v, 64)
				return err == nil && f > 0
			}
		}
		return true // 裸 "gzip" 无 q 参数 = 接受
	}
	return false
}
