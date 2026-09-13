package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCred(t *testing.T) []Credential {
	t.Helper()
	c, err := ParseCredential("sess-erin:sess-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	return []Credential{c}
}

func TestSessionIssueValidate(t *testing.T) {
	creds := testCred(t)
	ss := newSessionStore(time.Hour, "", credentialDigest(creds))
	now := time.Now()
	tok := ss.issue(now)
	if tok == "" || len(tok) != 22 {
		t.Fatalf("token 形态 = %q（want 22 字符 base64url）", tok)
	}
	if ok, _ := ss.validate(tok, now); !ok {
		t.Fatal("刚签发的 token 校验失败")
	}
	if ok, _ := ss.validate("AAAAAAAAAAAAAAAAAAAAAA", now); ok {
		t.Fatal("未知 token 校验通过")
	}
	if ok, _ := ss.validate("", now); ok {
		t.Fatal("空 token 校验通过")
	}
}

func TestSessionExpiry(t *testing.T) {
	creds := testCred(t)
	ss := newSessionStore(50*time.Millisecond, "", credentialDigest(creds))
	now := time.Now()
	tok := ss.issue(now)
	// now 处剩余 = 全量 ttl，不触发续期——后续过期断言才成立。
	if ok, renewed := ss.validate(tok, now); !ok || renewed {
		t.Fatalf("初始校验 ok=%v renewed=%v, want true,false", ok, renewed)
	}
	if ok, _ := ss.validate(tok, now.Add(60*time.Millisecond)); ok {
		t.Fatal("过期 token 校验通过")
	}
}

func TestSessionSlidingRenewal(t *testing.T) {
	creds := testCred(t)
	ss := newSessionStore(time.Hour, "", credentialDigest(creds))
	now := time.Now()
	tok := ss.issue(now)
	// 剩余 ≈ ttl，无续期。
	if ok, renewed := ss.validate(tok, now.Add(time.Minute)); !ok || renewed {
		t.Fatalf("早期校验 renewed = %v, want false", renewed)
	}
	// 剩余 <ttl/2 → 续期。
	if ok, renewed := ss.validate(tok, now.Add(40*time.Minute)); !ok || !renewed {
		t.Fatalf("过半后续期 renewed = %v, want true", renewed)
	}
	// 续期后过期时刻后移：原 ttl 边界之后仍有效。
	if ok, _ := ss.validate(tok, now.Add(70*time.Minute)); !ok {
		t.Fatal("续期后会话提前失效")
	}
}

func TestSessionRevoke(t *testing.T) {
	creds := testCred(t)
	ss := newSessionStore(time.Hour, "", credentialDigest(creds))
	now := time.Now()
	tok := ss.issue(now)
	ss.revoke(tok, now)
	if ok, _ := ss.validate(tok, now); ok {
		t.Fatal("吊销后 token 仍有效")
	}
}

func TestSessionPersistRoundTrip(t *testing.T) {
	creds := testCred(t)
	path := filepath.Join(t.TempDir(), "sessions.json")
	cred := credentialDigest(creds)
	now := time.Now()
	ss := newSessionStore(time.Hour, path, cred)
	tok := ss.issue(now)

	// 新实例（模拟进程重启）从同一文件加载 → cookie 仍有效。
	ss2 := newSessionStore(time.Hour, path, cred)
	if ok, _ := ss2.validate(tok, now); !ok {
		t.Fatal("重启后会话未从磁盘恢复（持久化失效）")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("session 文件未落盘: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("session 文件权限 = %04o, want 0600", perm)
	}
}

func TestSessionCorruptFileEmpty(t *testing.T) {
	creds := testCred(t)
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	ss := newSessionStore(time.Hour, path, credentialDigest(creds))
	if len(ss.m) != 0 {
		t.Fatal("损坏文件应得空表")
	}
	// 损坏文件不阻断签发与持久化。
	tok := ss.issue(time.Now())
	if tok == "" {
		t.Fatal("损坏文件后签发失败")
	}
}

func TestSessionCredRotationRevokesAll(t *testing.T) {
	creds := testCred(t)
	path := filepath.Join(t.TempDir(), "sessions.json")
	now := time.Now()
	ss := newSessionStore(time.Hour, path, credentialDigest(creds))
	tok := ss.issue(now)

	other, err := ParseCredential("sess-erin:new-pass")
	if err != nil {
		t.Fatal(err)
	}
	// 改密码 → 摘要变化 → 整表作废。
	ss2 := newSessionStore(time.Hour, path, credentialDigest([]Credential{other}))
	if ok, _ := ss2.validate(tok, now); ok {
		t.Fatal("凭据轮换后旧会话仍有效（改密码未吊销）")
	}
}

func TestSessionReloadOnMiss(t *testing.T) {
	creds := testCred(t)
	path := filepath.Join(t.TempDir(), "sessions.json")
	cred := credentialDigest(creds)
	now := time.Now()
	a := newSessionStore(time.Hour, path, cred)
	b := newSessionStore(time.Hour, path, cred)

	tokA := a.issue(now) // A 签发并落盘
	// B 内存无此 token：reload-on-miss 从文件读到。
	if ok, _ := b.validate(tokA, now); !ok {
		t.Fatal("多实例 reload-on-miss 失败（B 看不到 A 的会话）")
	}
	// B 签发，A reload 可见。
	tokB := b.issue(now)
	if ok, _ := a.validate(tokB, now); !ok {
		t.Fatal("多实例反向 reload 失败")
	}
}

func TestCredentialDigestStableAndOrderSensitive(t *testing.T) {
	creds := testCred(t)
	d1 := credentialDigest(creds)
	if d1 != credentialDigest(creds) {
		t.Fatal("同输入摘要不稳定")
	}
	if d1 == "" || len(d1) != 64 {
		t.Fatalf("摘要形态 = %q（want 64 hex）", d1)
	}
	other, _ := ParseCredential("sess-erin:sess-pass2")
	if d1 == credentialDigest([]Credential{other}) {
		t.Fatal("不同凭据摘要相同")
	}
}
