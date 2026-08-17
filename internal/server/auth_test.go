package server

import (
	"crypto/sha256"
	"testing"
)

// TestParseCredential 锁定凭据解析形态（SEC-01）：首个 ':' 切分（密码可含
// ':'，user 不可含——RFC 7617 user-id 约束）；无冒号/空 user 报错；
// 空 pass 合法（"user:" → passHash 为空串摘要，不额外禁止，文档化决策）。
// 同包白盒：断言不导出字段确为预哈希摘要而非明文（不变量的直接证据）。
func TestParseCredential(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		user    string // 期望的明文 user（用于校验预哈希值）
		pass    string
	}{
		{name: "正常 user:pass", in: "alice:s3cret", user: "alice", pass: "s3cret"},
		{name: "密码含冒号", in: "alice:pass:with:colon", user: "alice", pass: "pass:with:colon"},
		{name: "空 pass 合法", in: "alice:", user: "alice", pass: ""},
		{name: "无冒号报错", in: "nocolon", wantErr: true},
		{name: "空 user 报错", in: ":pass", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseCredential(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseCredential(%q) 未报错，want error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCredential(%q) err = %v, want nil", tt.in, err)
			}
			if c.userHash != sha256.Sum256([]byte(tt.user)) {
				t.Errorf("userHash 非 sha256(%q) 预哈希值——不变量破坏（Pitfall 1：必须是 32B 定长摘要）", tt.user)
			}
			if c.passHash != sha256.Sum256([]byte(tt.pass)) {
				t.Errorf("passHash 非 sha256(%q) 预哈希值——不变量破坏（Pitfall 1）", tt.pass)
			}
		})
	}
}

// TestCredentialMatch 锁定常数时间比较语义（SEC-01）：单组正/错 user/错 pass/
// 空列表 false；多组（3 组）各自命中 true；跨组错配 false；空 user/pass 对
// 非空凭据 false。多组各自命中直接锁死 planner erratum 回归（RESEARCH
// Pattern 2 的 &= 初值 0 恒 false——永拒正确凭据）。
// 常数时间属性由构造保证（subtle + 定长摘要 + 不短路循环），时序测量断言
// 不可移植不做（backstop truth 对应物，以代码形态走查为证）。
func TestCredentialMatch(t *testing.T) {
	mustParse := func(s string) Credential {
		c, err := ParseCredential(s)
		if err != nil {
			t.Fatalf("ParseCredential(%q) err = %v", s, err)
		}
		return c
	}

	t.Run("单组", func(t *testing.T) {
		creds := []Credential{mustParse("alice:s3cret")}
		if !matchCredential(creds, "alice", "s3cret") {
			t.Errorf("正确凭据未命中")
		}
		if matchCredential(creds, "mallory", "s3cret") {
			t.Errorf("错 user 命中")
		}
		if matchCredential(creds, "alice", "wrong") {
			t.Errorf("错 pass 命中")
		}
	})

	t.Run("空凭据列表", func(t *testing.T) {
		if matchCredential(nil, "alice", "s3cret") {
			t.Errorf("空列表命中（无凭据模式调用方不进此函数，防御性语义亦须为 false）")
		}
	})

	t.Run("多组各自命中", func(t *testing.T) {
		creds := []Credential{
			mustParse("alice:p1"),
			mustParse("bob:p2"),
			mustParse("carol:p3"),
		}
		for _, tt := range []struct{ user, pass string }{
			{"alice", "p1"},
			{"bob", "p2"},
			{"carol", "p3"},
		} {
			if !matchCredential(creds, tt.user, tt.pass) {
				t.Errorf("(%q,%q) 未命中——多组逐组轮询语义破坏（erratum 回归闸）", tt.user, tt.pass)
			}
		}
		if matchCredential(creds, "alice", "p2") {
			t.Errorf("跨组错配（user1+pass2）命中——凭据组边界破坏")
		}
	})

	t.Run("空 user/pass 对非空凭据", func(t *testing.T) {
		creds := []Credential{mustParse("alice:s3cret")}
		if matchCredential(creds, "", "s3cret") {
			t.Errorf("空 user 命中")
		}
		if matchCredential(creds, "alice", "") {
			t.Errorf("空 pass 命中")
		}
		if matchCredential(creds, "", "") {
			t.Errorf("空 user+pass 命中")
		}
	})
}
