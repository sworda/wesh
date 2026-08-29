// FuzzDecodeFileConfig（09-02，D-09/D-10）：TOML 配置解析面 fuzz 目标——经
// decodeFileConfig reader 委托以 bytes-in 直驱（loadFileConfig 是 path-in，接缝
// 提取是 fuzz 落地的必要前置，config.go 签名注释登记）。两不变量：
//  1. 任意输入不 panic；
//  2. err 非 nil 时 err.Error() 绝不含 FUZZ_PROBE_SECRET 探针值——值剥离红线
//     （config.go 文件头，SEC-01 启动面延伸）的 fuzz 断言形态；键名回显是合法
//     行为不在断言面，只断言值探针（config_test.go「value stripped」子测族同口径）。
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

func FuzzDecodeFileConfig(f *testing.F) {
	f.Add([]byte("port = 7681\nbind = \"127.0.0.1\"\n"))      // 合法键
	f.Add([]byte("credential = [\"FUZZ_PROBE_SECRET:x\"]\n")) // 值剥离红线探针
	f.Add([]byte("unknown-key = 1\n"))                        // 未知键拒绝面（严格模式）
	f.Add([]byte("port = \"not-a-number\"\n"))                // 类型不符面
	f.Add([]byte{0xff, 0xfe, 0x00})                           // 非 UTF-8/二进制
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := decodeFileConfig("fuzz.toml", bytes.NewReader(data))
		if err == nil {
			return
		}
		if strings.Contains(err.Error(), "FUZZ_PROBE_SECRET") {
			t.Fatalf("value red line broken: %v", err)
		}
	})
}
