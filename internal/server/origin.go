package server

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// NormalizeOrigin 把 --origin 值规范化为小写 scheme://host[:port]（去默认端口），
// 供 03-04 main.go 的 --origin fs.Func 回调 parse 期校验（导出）；规范化串喂
// 03-03 的 AcceptOptions.OriginPatterns 与 originAllowed 集合查找。
//
// 拒绝形态（D-12 精确比较）：
//   - scheme 仅 http/https 且 Host 非空；
//   - path（裸 "/" 除外）/query/fragment/userinfo 任一非空即拒；
//   - 含 glob 字符 *?[\\ 即拒——coder/websocket 的 OriginPatterns 走 path.Match
//     glob 语义，拒掉模式字符使其退化为精确比较；
//   - 默认端口剥离（http:80/https:443）——浏览器 Origin 序列化省略默认端口
//     （RFC 6454），不剥则配置了 --origin https://foo.com:443 永不命中
//     （Pitfall 3 默认端口不对称）。
//
// 错误文案含原输入：flag 解析期报错面向部署者，需可定位是哪条值出问题。
func NormalizeOrigin(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("origin must be scheme://host[:port]: %q", s)
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("origin must not contain path/query/fragment/userinfo: %q", s)
	}
	if strings.ContainsAny(s, "*?[\\") {
		return "", fmt.Errorf("origin must not contain glob characters: %q", s)
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = "" // 默认端口剥离（RFC 6454 序列化省略，Pitfall 3）
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	return u.Scheme + "://" + host, nil
}

// originAllowed 与 coder/websocket accept.go:228-264 库内建语义逐项对齐
// （RESEARCH Pattern 4 一手核实）：
//
//	① 无 Origin 头放行（D-13：非浏览器客户端 curl/Node 零摩擦；CSWSH 威胁
//	   模型只约束浏览器，浏览器必发 Origin）；
//	② Origin 与 r.Host 同源（strings.EqualFold 大小写不敏感）放行；
//	③ 入站 Origin 经 NormalizeOrigin 同一规范化逻辑（小写 host + 剥默认
//	   端口）后精确查 allowed 集合，命中放行；
//	④ 否则拒绝。
//
// Origin: null（沙箱 iframe 等）：url.Parse("null") 后 u.Host 为空、不同源、
// 规范化失败 → 拒绝。这是正确行为——null Origin 是 CSWSH 常见载体。
// 规范化失败按拒绝处理（保守方向）。空集合时行为 = 库默认（仅无 Origin/同源
// 放行）——零配置零行为漂移（D-12）。
func originAllowed(r *http.Request, allowed map[string]struct{}) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(r.Host, u.Host) {
		return true
	}
	n, err := NormalizeOrigin(origin)
	if err != nil {
		return false
	}
	_, ok := allowed[n]
	return ok
}
