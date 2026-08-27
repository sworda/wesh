package server

// proxy.go —— 反代信任提取层（07-03，SEC-07，D-15..D-20）。决策依据登记
//（origin.go 同位文件组织纪律：包级纯函数 + 注释头登记决策号）：
//
//   - D-15：SEC-07 语义收窄 = 只要审计归因——attach 链路 logEvent 记录
//     remote_user。GoTTY 共享进程模型下 per-client env 注入结构性不成立
//     （spawn 时无 HTTP 请求、多客户端共享一个 shell），不追 ttyd -H 模型。
//   - D-16：信任模型 = 裸信任 + 暴露面启动警告——--auth-header 配置即信任
//     该头（ttyd 同款）；非 loopback 无凭据时 validateStartup stderr 醒目
//     警告（cmd/wesh/main.go）；可信来源 IP 校验（--trusted-proxy）列 deferred。
//   - D-17：与认证体系正交——提取只做记录，不做任何认证决定；Basic/ticket/
//     share token 三通道语义零改动，伪造头不能绕过任何认证检查
//     （TestAuthHeaderNoAuthBypass 回归锁）。
//   - D-18：头名 = --auth-header 可配（单个）——反代生态头名不统一
//     （authelia 发 Remote-User、oauth2-proxy 发 X-Forwarded-User）。
//   - D-19：remote_user 值清洗 = 剥离 C0/C1/DEL 控制字符 + 截断 128 rune
//     （P4 D-03 标题 sanitize 同款纪律的 Go 移植，web/src/lib/title.ts；
//     logEvent 是 stderr 单行文本，控制字符注入伪造日志行的风险当期就存在，
//     T-07-03b）。与标题纪律的差异点：空结果返回空串（缺席即不出键），
//     无 'wesh' 回退。
//   - D-20：X-Forwarded-For 与 auth-header 共用信任闸（--auth-header 给定 =
//     「信任反代」总开关，零双轨）；XFF 取链首 IP；消费范围 = logEvent
//     remote 字段与 throttle/halfOpen per-IP 键同换（反代后 per-IP 计数
//     聚合回代理 IP 的旧限制解除；攻击者轮换 XFF 获独立节流配额的风险
//     D-20 已裁决接受，T-07-03d——07-review CR-02 起链首经 net.ParseIP
//     校验，垃圾值不再获独立配额而是回退共享 TCP 对端键，该风险面随之
//     进一步收敛）。
//
// 红线（D-03 随新字段延伸，SEC-01）：token/ticket/凭据任何形态（含 base64）
// 永不作为 remote_user 或任何 logEvent 字段出现——结构性保证：remoteUser
// 的提取源只能是配置头名对应的 HTTP 头，share token（/s/ 路径段）与 Hello
// ticket 在结构上不可能进入本提取路径（T-07-03c）；配置头名本身经 parse 期
// 凭据载体头名拒绝闸（Authorization/Proxy-Authorization/Cookie/Set-Cookie，
// cmd/wesh/main.go，07-review CR-03）——配置即破线的结构性缺口已封闭。
//
// X-Forwarded-For 与配置头名均为不可信输入（信任边界：reverse-proxy → wesh）
// ——只有 operator 配置 --auth-header 且部署保证 wesh 不直连暴露时才可采信
// （D-16 警告即该部署面的启动期提示）。

import (
	"net"
	"net/http"
	"strings"
)

// sanitizeRemoteUser 是 D-19 清洗纪律的 Go 移植（title.ts:10-18 sanitizeTitle
// 同款）：逐 rune 剥离 ch<=0x1f（C0）、ch==0x7f（DEL）、0x80<=ch<=0x9f（C1）
// 控制字符，并截断前 128 rune——剥离先于截断计数（控制字符不消耗预算）；
// 按 rune 迭代不碎多字节（Go range string 语义，对应 Array.from code point
// 迭代）。空结果返回空串——logEvent 不出 remote_user 键（与标题空串回退
// 'wesh' 的差异点：审计字段缺席即不出键，不回退占位值）。
// remote_user 是用户可控字段（wesh → stderr 信任边界，T-07-03b）——换行/
// 控制字符可伪造日志行，单行文本日志下剥离即结构性消除注入。
func sanitizeRemoteUser(s string) string {
	r := make([]rune, 0, len(s))
	for _, ch := range s {
		if ch <= 0x1f || ch == 0x7f || (ch >= 0x80 && ch <= 0x9f) {
			continue
		}
		r = append(r, ch)
		if len(r) >= 128 {
			break
		}
	}
	return string(r)
}

// proxyInfo 是反代信任配置（New 装配期固化、运行期只读，故 plain 字段无锁）：
// trust = 信任闸（--auth-header 给定即为真，D-20 单一开关零双轨）；
// userHeader = 可配用户头名（D-18）。零值（false, ""）= 不信任/无头名——
// 全部提取行为与现状逐字节一致（XFF 完全忽略、remote_user 不出键）。
type proxyInfo struct {
	trust      bool
	userHeader string
}

// clientIP 取 per-IP 计数键（throttle/halfOpen 共用）：trust 且 XFF 头非空 →
// 链首 IP（strings.Cut(",") 首段——链首最接近真实客户端，链尾恒为反代自己
// 无信息，RESEARCH Anti-Pattern 表；TrimSpace 清洗）；链首经 net.ParseIP
// 校验——非法值（空串/"unknown"/垃圾值/控制字符注入）一律与缺席同档回退
// TCP 对端现状取值（07-review CR-02：XFF 首段恒为客户端可控——标准追加式
// 反代 $proxy_add_x_forwarded_for 语义，未校验首段原样进 logEvent remote
// 字段可注入 C1/CSI 伪造日志行甚至终端转义序列；ParseIP 通过值字符集恒为
// [0-9a-fA-F:.]，结构性排除注入，D-19 sanitizeRemoteUser 同一威胁类在
// remote 路径的等价闸；同时收敛节流键卫生——垃圾键不再各自独占节流配额）。
// 回退形态：net.SplitHostPort host 部分（含端口直接当键会使每连接一个
// "新 IP"、上限形同虚设，Pitfall 6；失败回退 RemoteAddr 整串）。
func (p proxyInfo) clientIP(r *http.Request) string {
	if p.trust {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if ip := strings.TrimSpace(first); ip != "" && net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// remote 取 logEvent remote 字段值（D-20 换键点）：trust → clientIP(r)
// （XFF 链首换入，日志归因与节流计数同键——两消费不分叉）；否则
// r.RemoteAddr 原样（现状 host:port 形态逐字节保持）。
func (p proxyInfo) remote(r *http.Request) string {
	if p.trust {
		return p.clientIP(r)
	}
	return r.RemoteAddr
}

// remoteUser 提取 remote_user 日志字段值（D-15 审计归因唯一数据源）：
// trust 且配置头存在 → sanitizeRemoteUser(头值)；否则 ""（logEvent 不出键，
// 未配置/头缺席与现状逐字节一致）。多值头（重复头行）取 Header.Get 首值
// （EDGE_ABSENT 回退裁决，07-03 PLAN flagged_assumptions 登记）。
// D-17 正交：返回值只做 logEvent 记录，绝不进入任何认证/授权判定路径。
func (p proxyInfo) remoteUser(r *http.Request) string {
	if !p.trust {
		return ""
	}
	return sanitizeRemoteUser(r.Header.Get(p.userHeader))
}
