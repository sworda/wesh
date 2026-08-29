// FuzzDecodeHello / FuzzDecodeResize（09-02，D-09/D-10）：proto 帧解码面 fuzz
// 目标——WS 远程输入面（Hello 是握手首帧，fuzzer 的远程攻击面替身）。直挂既有
// 导出函数，bytes-in 纯函数零改造。两不变量：
//  1. 任意输入不 panic；
//  2. 解码成功 ⇒ Cols/Rows 恒在 [1,1000]（ClampDim 契约，proto.go ClampDim）。
//
// 帧拆分型字节面（data[0] 类型字节 + data[1:] 载荷）在 server.go 有
// len(data)==0 前置守卫（:838/:1006），无独立目标必要——DecodeResize 即稳态
// 帧解码面。
//
// 种子五类：合法 / 负值超大 / 截断 / 空载荷 / 类型混乱。种子与 testdata/fuzz
// 崩溃语料随常规 go test 作为普通单测运行（零时长回归门，D-10）；60s CI 短跑
// 门在 ci.yml fuzz job，发布前 10min 长跑由 09-09 发布脚本承载（D-14）。
package proto_test

import (
	"testing"

	"github.com/sworda/wesh/internal/proto"
)

func FuzzDecodeHello(f *testing.F) {
	f.Add([]byte(`{"version":"wesh.v1","cols":80,"rows":24}`))                  // 合法
	f.Add([]byte(`{"version":"wesh.v1","cols":-1,"rows":999999,"ticket":"x"}`)) // 负值超大+可选键
	f.Add([]byte(`{"version":`))                                                // 截断
	f.Add([]byte{})                                                             // 空载荷（server 侧空帧另有 len 守卫，本层只证不 panic）
	f.Add([]byte(`{"version":"wesh.v1","cols":1e999,"rows":true}`))             // 类型混乱
	f.Fuzz(func(t *testing.T, data []byte) {
		hp, ok := proto.DecodeHello(data)
		if !ok {
			return
		}
		if hp.Cols < 1 || hp.Cols > 1000 || hp.Rows < 1 || hp.Rows > 1000 {
			t.Fatalf("ClampDim broken: %+v from %q", hp, data)
		}
	})
}

func FuzzDecodeResize(f *testing.F) {
	f.Add([]byte(`{"cols":80,"rows":24}`))      // 合法
	f.Add([]byte(`{"cols":-1,"rows":999999}`))  // 负值超大
	f.Add([]byte(`{"cols":`))                   // 截断
	f.Add([]byte{})                             // 空载荷
	f.Add([]byte(`{"cols":1e999,"rows":true}`)) // 类型混乱
	f.Fuzz(func(t *testing.T, data []byte) {
		cols, rows, ok := proto.DecodeResize(data)
		if !ok {
			return
		}
		if cols < 1 || cols > 1000 || rows < 1 || rows > 1000 {
			t.Fatalf("ClampDim broken: %dx%d from %q", cols, rows, data)
		}
	})
}
