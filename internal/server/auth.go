package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
)

// Credential 是一组启动时预哈希的凭据（SHA-256 摘要对，32B 定长）。
//
// 不变量：字段不导出，只能经 ParseCredential 构造——保证比较路径上的操作数
// 永远是预哈希摘要而非明文。SHA-256 等长化消除长度侧信道：
// subtle.ConstantTimeCompare 官方明示"If the lengths of x and y do not match
// it returns 0 immediately"，直接比较明文会泄露凭据长度（Pitfall 1）。
//
// 导出原因：03-04 main.go 经 Options.Credentials 传入（跨包消费）。
type Credential struct {
	userHash [sha256.Size]byte
	passHash [sha256.Size]byte
}

// ParseCredential 解析 "user:pass" 形态凭据：strings.Cut 切首个 ':'
// （密码可含 ':'；user 不可含——RFC 7617 user-id 约束）。无冒号或空 user
// 报错；空 pass 合法（"user:" → passHash 为空串摘要，文档化决策不额外禁止）。
// 供 03-04 --credential flag 的 fs.Func 回调在 parse 期校验（导出）。
func ParseCredential(s string) (Credential, error) {
	u, p, ok := strings.Cut(s, ":")
	if !ok || u == "" {
		return Credential{}, fmt.Errorf("credential must be user:pass")
	}
	return Credential{
		userHash: sha256.Sum256([]byte(u)),
		passHash: sha256.Sum256([]byte(p)),
	}, nil
}

// matchCredential 逐组轮询、位或累积——不短路不 break，循环恒跑满全部组：
// 耗时与组数恒定正交，无"第几组匹配"的组序号时序泄露（RESEARCH Pattern 2）。
// ConstantTimeCompare 的操作数均为 sha256.Sum256 的 32B 定长摘要，禁止传入
// 原始字符串（Pitfall 1 长度泄露）。
//
// planner erratum 修正：RESEARCH Pattern 2 行 288-297 定稿代码为
// `matched &= ...` 且 matched 初值 0——`0 & x` 恒 0，结果永为 false（正确
// 凭据永远拒绝）。本实现为修正形态 `matched |= user比较 & pass比较`，保持
// "耗时与组数正交"设计意图；TestCredentialMatch 多组各自命中锁死该回归。
//
// 无凭据模式（creds 空）调用方不进此函数；防御性语义下空列表亦返回 false。
func matchCredential(creds []Credential, user, pass string) bool {
	uh := sha256.Sum256([]byte(user))
	ph := sha256.Sum256([]byte(pass))
	matched := 0
	for _, c := range creds {
		matched |= subtle.ConstantTimeCompare(uh[:], c.userHash[:]) &
			subtle.ConstantTimeCompare(ph[:], c.passHash[:])
	}
	return matched == 1
}
