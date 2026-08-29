package server

// health.go —— 08-03 OPS-06 /healthz 探活端点（D-07 免认证窄例外 / D-09 根路径
// 固定 / D-10 200+状态 JSON / D-11 关停 503 draining）。
//
// 红线（D-10 裁决面，T-08-03a）：body 恒为粗粒度容量四字段
// （status/clients/max_clients/session_active）——无版本号、无客户端身份、
// 无内部错误细节（枚举 oracle 面；version 只在需认证的 /metrics build_info，
// 08-04）。
//
// D-07 防例外蔓延：本 handler 不挂任何认证包装——整站 Basic 闸唯一窄例外
// （探活器 k8s liveness/反代健康检查结构性带不了凭据，且端点零敏感信息双前提
// 成立才开）；注册点在 Handler() 认证两分支之外唯一一处（server.go 注释登记），
// 新端点不得以此为例外先例。
//
// D-12：与主服务同端口，securityHeaders 包裹自动继承（server.go Handler 返回点）。

import (
	"encoding/json"
	"net/http"
)

// healthzHandler 为 GET /healthz 的处理函数：draining 置位（Shutdown 入口，
// D-11）→ 503 + status="draining"，否则 200 + status="ok"；body 经
// encoding/json Marshal 匿名 struct（键逐字 status/clients/max_clients/
// session_active——禁止手写 JSON 拼接，Don't Hand-Roll 纪律）；数据源全部
// hubMu 外 atomic 读（registry.n / sessionAlive）+ New 装配期固化只读
// （maxClients）——R-07 纪律，handler 不取 hubMu。
func (s *Server) healthzHandler(w http.ResponseWriter, _ *http.Request) {
	status := "ok"
	code := http.StatusOK
	if s.draining.Load() { // D-11：Shutdown 入口置位（与 s.exiting 同源触发点）
		status, code = "draining", http.StatusServiceUnavailable
	}
	body, err := json.Marshal(struct {
		Status        string `json:"status"`
		Clients       int64  `json:"clients"`
		MaxClients    int    `json:"max_clients"`
		SessionActive bool   `json:"session_active"`
	}{
		Status:        status,
		Clients:       s.registry.n.Load(),
		MaxClients:    s.maxClients,
		SessionActive: s.sessionAlive.Load(),
	})
	if err != nil {
		// 纯标量匿名 struct 的 Marshal 结构性不可达失败——防御性 500，不泄露细节。
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(body)
}
