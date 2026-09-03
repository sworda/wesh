// FuzzDecodeFileConfig（09-02，D-09/D-10）：TOML 配置解析面 fuzz 目标——经
// decodeFileConfig reader 委托以 bytes-in 直驱（loadFileConfig 是 path-in，接缝
// 提取是 fuzz 落地的必要前置，config.go 签名注释登记）。两不变量：
//  1. 任意输入不 panic；
//  2. err 非 nil 时 err.Error() 的非键名上下文绝不含 FUZZ_PROBE_SECRET 探针
//     ——值剥离红线（config.go 文件头，SEC-01 启动面延伸）的 fuzz 断言形态；
//     键名回显（unknown keys (...) 键名清单 / key "..." 引号段）是合法行为
//     不在断言面，经 stripKeyNameEcho 剥除后断言（config_test.go「value
//     stripped」子测族同口径——探针在值位置、键名用普通词；fuzzer 可把探针
//     搬进键名位置：2026-08-30 发布长跑语料 ["FUZZ_PROBE_SECRET"] 表头实证，
//     全文字面断言误报一次）。
//
// 种子五类：合法键 / credential 探针键 / 未知键（严格模式拒绝面）/ 类型不符 /
// 非 UTF-8 二进制。种子与 testdata/fuzz 崩溃语料随常规 go test 作为普通单测
// 运行（零时长回归门，D-10）；60s CI 短跑门在 ci.yml fuzz job，发布前 10min
// 长跑由 09-09 发布脚本承载（D-14）。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// 键名回显豁免面：configErr 单写口（config.go:86）钉死错误文案形态
// `invalid config file %s: %s (%s)`，探针仅有的两处合法出现位是 detail 段的
// 键名上下文——unknown keys (KEY1, KEY2) 括号内键名清单、invalid toml 分支的
// key "KEY" line N 引号段（值剥离经「只取 Key()」实现，config.go:98-102）。
// 剥除后仍含探针即值透传形态——兜底分支固定文案结构性不含探针，该形态可达
// 即值剥离红线破口。fail-closed：形态不匹配时不剥除，探针残留即 FAIL。
func stripKeyNameEcho(msg string) string {
	if i := strings.Index(msg, "unknown keys ("); i >= 0 && strings.HasSuffix(msg, ")") {
		msg = msg[:i]
	}
	if i := strings.Index(msg, `key "`); i >= 0 {
		rest := msg[i+len(`key "`):]
		if j := strings.Index(rest, `"`); j >= 0 {
			msg = msg[:i] + rest[j+len(`"`):]
		}
	}
	return msg
}

// TestStripKeyNameEcho：剥除行为锁——键名两上下文剥除、值上下文保留（红线
// 断言有效性的直接形态证明：值透传形态剥除后探针残留仍 FAIL）。
func TestStripKeyNameEcho(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool // 剥除后仍含探针（true=值红线 FAIL 形态）
	}{
		{"unknown keys 键名清单（2026-08-30 语料形态）",
			"invalid config file fuzz.toml: unknown keys (FUZZ_PROBE_SECRET)", false},
		{"unknown keys 多键清单",
			"invalid config file fuzz.toml: unknown keys (FUZZ_PROBE_SECRET, no-auth)", false},
		{"invalid toml 键名引号段",
			`invalid config file fuzz.toml: invalid toml (key "FUZZ_PROBE_SECRET" line 3)`, false},
		{"invalid toml 无键名行号形态",
			"invalid config file fuzz.toml: invalid toml (line 1)", false},
		{"值透传形态（源行回显——Pitfall 5 破口）",
			`invalid config file fuzz.toml: cannot parse (line 1: credential = ["alice:FUZZ_PROBE_SECRET"])`, true},
		{"兜底固定文案",
			"invalid config file fuzz.toml: cannot parse (unrecognized toml error)", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Contains(stripKeyNameEcho(c.in), "FUZZ_PROBE_SECRET")
			if got != c.want {
				t.Errorf("stripKeyNameEcho(%q) 探针残留 = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func FuzzDecodeFileConfig(f *testing.F) {
	f.Add([]byte("port = 7681\nbind = \"127.0.0.1\"\n"))      // 合法键
	f.Add([]byte("credential = [\"FUZZ_PROBE_SECRET:x\"]\n")) // 值剥离红线探针
	f.Add([]byte("unknown-key = 1\n"))                        // 未知键拒绝面（严格模式）
	f.Add([]byte("port = \"not-a-number\"\n"))                // 类型不符面
	f.Add([]byte{0xff, 0xfe, 0x00})                           // 非 UTF-8/二进制
	// 10-03 PC-01（Pitfall 11「fuzz 语料/红线测试同 PR」纪律——session-mode
	// 新键入白名单与非法值 parse 拒绝同阶段落地）：
	f.Add([]byte("session-mode = \"shared\"\n"))     // 合法键——新键入白名单的 fuzz 面
	f.Add([]byte("session-mode = \"per-client\"\n")) // 合法键
	// 非法枚举：decodeFileConfig 层不报错（合法 string 类型），枚举拒绝归
	// parseArgs 闸（10-01 一闸双覆盖）——本种子断言面 = 两不变量不破坏。
	f.Add([]byte("session-mode = \"banana\"\n"))
	f.Add([]byte("session_mode = \"shared\"\n")) // 下划线形态 → 未知键拒绝面（D-03 键名修正的行为锁）
	f.Add([]byte("session-mode = 1\n"))          // 类型不符面（DecodeError 分支）
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := decodeFileConfig("fuzz.toml", bytes.NewReader(data))
		if err == nil {
			return
		}
		if msg := err.Error(); strings.Contains(msg, "FUZZ_PROBE_SECRET") {
			if strings.Contains(stripKeyNameEcho(msg), "FUZZ_PROBE_SECRET") {
				t.Fatalf("value red line broken: %v", err)
			}
		}
	})
}
