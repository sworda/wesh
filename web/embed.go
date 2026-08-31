// Package web 以 go:embed 内嵌前端构建产物并提供静态伺服。
//
// 硬约束：//go:embed 不能引用包目录之外的文件，故 embed.go 必须与 dist/ 同级（放 web/）。
// 仓库提交 web/dist/index.html 占位，保证裸 clone 后 go build ./... / go test ./... 可编译；
// 真实产物由 pnpm -C web build 生成（构建顺序：pnpm 先于 go build，D-18）。
package web

import (
	"bytes"
	"compress/gzip"
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

// WithCustomIndex 装饰静态 handler，落 --index 自定义首页的整页替换契约
// （09-04 OPS-03，D-05/D-06/D-07，09-UI-SPEC §Custom Index Contract + §4
// 定稿采纳预压）：index.html 路径（含空路径回落）返回启动读入的自定义字节
// （byte-identity——wesh 零注入零模板零校验，D-05；伺服字节与读入字节恒等），
// 其余一切路径照旧委托原 handler（FileServerFS——自定义页引用的相对路径资源
// → 404 是契约语义，T-09-04e）。装饰层在 sharePage 委托上游——/ 与
// /s/{token}/ 两通道经同一装饰实例单点统一（D-06，server.go Handler() 唯一
// 调用点）；不加 Cache-Control 新头（sharePage 注释既定纪律延伸，§6）。
//
// gzip 预压（§4）：装饰期对定长 page 预压一次缓存（compress/gzip
// BestCompression——stdlib 零新依赖；16MiB 上限下明文+压缩双份内存可接受），
// 运行期零压缩开销；Accept-Encoding 显式含 gzip → Content-Encoding: gzip
// 发预压体，否则明文 page；Vary: Accept-Encoding 恒发（Handler 同款纪律——
// 同一 URL 两表示，防中间缓存键不完整）；解析复用 acceptsGzip（零第二份
// Accept-Encoding 解析器）。Content-Type 按 .html 扩展名推断同款
// （mime.TypeByExtension——Handler 的 .gz 旁路同形态）。
func WithCustomIndex(h http.Handler, page []byte) http.Handler {
	var gzBuf bytes.Buffer
	// BestCompression（§4 定稿）；常量恒合法使 err 分支结构性不可达——防御性
	// 回落 NewWriter（默认级）保持预压不变量（gzBody 非 nil 可伺服）。
	zw, werr := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
	if werr != nil {
		zw = gzip.NewWriter(&gzBuf)
	}
	_, _ = zw.Write(page) // bytes.Buffer 写入结构性不失败
	_ = zw.Close()        // 同上；预压产物定长缓存，运行期只读
	gzBody := gzBuf.Bytes()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if name != "index.html" {
			h.ServeHTTP(w, r) // 其余一切路径照旧（相对资源 404 契约语义）
			return
		}
		w.Header().Set("Vary", "Accept-Encoding")
		if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		if acceptsGzip(r.Header.Get("Accept-Encoding")) {
			w.Header().Set("Content-Encoding", "gzip")
			w.Write(gzBody)
			return
		}
		w.Write(page)
	})
}
