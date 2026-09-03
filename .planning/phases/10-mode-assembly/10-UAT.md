---
status: complete
phase: 10-mode-assembly
source: [10-01-SUMMARY.md, 10-02-SUMMARY.md, 10-03-SUMMARY.md, 10-04-SUMMARY.md, 10-05-SUMMARY.md]
started: 2026-09-03T09:37:46Z
updated: 2026-09-03T09:56:00Z
---

## Current Test

[testing complete]

## Tests

### 1. --session-mode CLI flag 双枚举值解析入 cfg + 缺省终值 shared（SC1 前半）
expected: --session-mode=per-client/shared/缺省 三形态均解析正确且打印 listening on
result: pass
source: automated
coverage_id: 10-01-D1

### 2. 非法枚举值 CLI/TOML 双源 parse 期拒绝 exit 2，D-04 定案文案全文（SC2）
expected: --session-mode=banana 与 bad.toml session-mode=banana 均 exit 2 且文案全文一致（一闸双覆盖）
result: pass
source: automated
coverage_id: 10-01-D2

### 3. 优先级链 CLI flag > TOML > 内置默认 shared（D-03 env 层真空成立）
expected: flag 覆盖 TOML、TOML 覆盖内置默认、键缺席归 shared；env 层无影响
result: pass
source: automated
coverage_id: 10-01-D3

### 4. Options.SessionMode/SpawnFunc 接缝 + ValidateOptions 互斥校验 + New 零值归一 shared
expected: per-client×SpawnFunc=nil 拒绝 / shared×SpawnFunc≠nil 拒绝 / 零值归一 shared；全量 -race 绿
result: pass
source: automated
coverage_id: 10-01-D4

### 5. pty.Start ≡ StartWithSize(argv, opts, SpawnCols, SpawnRows) 单行委托，80×24 零第二副本
expected: Start 委托 StartWithSize，尺寸单一事实源保持
result: pass
source: automated
coverage_id: 10-01-D5

### 6. 零回归收口闸：v1.0 全量 Go 测试原样全绿且本 plan 零既有测试文件改动
expected: go test -race -count=1 ./... 五包全 ok；main_test.go diff 删除行 == 0
result: pass
source: automated
coverage_id: 10-01-D6

### 7. write-policy×per-client warn（CLI/TOML 双源同档触发、双 flag 名文案、shared/未显式两面不触发）
expected: --writable --write-policy=all --session-mode=per-client → warn 行（含双 flag 名）+ listening on 正常启动
result: pass
source: automated
coverage_id: 10-02-D1

### 8. warn 合并不遮蔽（非 loopback 安全警告与新 warn 同现；socket 早退透出）
expected: --no-auth 非 loopback 时 stderr 两类警告同现
result: pass
source: automated
coverage_id: 10-02-D2

### 9. per-client LookPath(argv0) 启动预检（SC4：缺失拒绝/可执行放行/shared 不预检/空串不触发）
expected: wesh --session-mode=per-client -- wesh-no-such-cmd-7f3a → exit 2 + not found in PATH 文案
result: pass
source: automated
coverage_id: 10-02-D3

### 10. ValidateOptions 三态互斥契约（两拒绝 + 两放行 + 零值归一）
expected: TestValidateOptions 五态表驱动全 PASS
result: pass
source: automated
coverage_id: 10-02-D4

### 11. Start ≡ StartWithSize 委托等价 + 132x43 尺寸真实到达 TIOCSWINSZ
expected: TestStartWithSizeDelegation 两面（委托等价 + 尺寸读回）PASS
result: pass
source: automated
coverage_id: 10-02-D5

### 12. TOML session-mode 铺底生效 + sessionModeSet 置位 + CLI 覆盖
expected: TOML 键生效且显式位置位；CLI flag 覆盖 TOML
result: pass
source: automated
coverage_id: 10-03-D1

### 13. 优先级链 flag > TOML > 内置默认 shared（env 真空）——TOML 层证据
expected: TestConfigPrecedence session-mode 三腿合一 PASS
result: pass
source: automated
coverage_id: 10-03-D2

### 14. redlines：下划线键未知键拒绝 / banana 与 CLI 同文案 / 类型不符 invalid toml 分支
expected: session_mode → unknown keys exit 2；banana → 与 CLI 同文案；类型不符 → invalid toml 分支
result: pass
source: automated
coverage_id: 10-03-D3

### 15. FuzzDecodeFileConfig 五新种子（零时长回归门 + 30s 短跑零崩溃零红线破口）
expected: 十种子全过；30s fuzz 3.04M execs PASS
result: pass
source: automated
coverage_id: 10-03-D4

### 16. CONFIGURATION.md 五处 + 29→30 计数 + README 一句 + --help 口径一致（D-05）
expected: 文档五处落地、键计数 30、README 一句、--help 与文档同一事实叙述
result: pass
source: automated
coverage_id: 10-04-D1

### 17. 零回归双证据：全量 -race 五包全绿 + 既有协议 UAT 八脚本原样重跑全过（脚本零修改）
expected: go test -race ./... 五包 ok；web/uat/phase02-09.mjs 八脚本 exit 全 0 且 PASS 计数与基线一致
result: pass
source: automated
coverage_id: 10-04-D2

### 18. 冒烟三命令：CLI/TOML banana 双源 exit 2 同文案；per-client 与缺省两形态 listening on
expected: 三命令进程级直证全中
result: pass
source: automated
coverage_id: 10-04-D3

### 19. WR-01 闭合——SC4 预检 --cwd 感知对齐（放行/拒绝/裸名/shared 对照四面）
expected: per-client × --cwd × ./run.sh（0755）→ listening on（修复前误拒）；缺失/无执行位 → exit 2 not executable；裸名缺失 → not found in PATH 零漂移；shared 对照零漂移
result: pass
source: automated
coverage_id: 10-05-D1

### 20. WR-02 闭合——ValidateOptions 前移至分岔块尾部、pty.Start 之前（守卫触发零资源占用）
expected: 单调用点计数门 == 1；V(1328) < P(1334) < L(1342) 位序断言成立
result: pass
source: automated
coverage_id: 10-05-D2

### 21. 零回归收口闸：-race 五包全 ok + 八 UAT 脚本原样全过 + append-only 零删除行 + banana 双源冒烟
expected: main HEAD（含 WR-01/02 修复）上全量验证首跑全绿，PASS 计数对齐 10-04 基线
result: pass
source: automated
coverage_id: 10-05-D3

### 22. 自动化覆盖确认（21/21 交付物 auto-passed）
expected: 21 项交付物全部由通过的自动化测试确定性覆盖（classify-coverage 全量 auto_passed、present 为空）。用户确认接受自动化证据作为 UAT 通过依据；如有异议指出具体条目即转人工检查点。
result: pass
note: 用户授权全量自动化复验代替人工确认（「能自动化测试的你就自动化测试」）——2026-09-03 Linux 侧独立复跑全部通过：静态三闸（build 0.769s / vet 干净 / GOROOT gofmt 零输出）、全量 -race 五包 ok（58.6s）、冒烟矩阵 12/12（banana CLI/TOML 双源同文案 rc=2、三形态 listening on、write-policy×per-client warn 双 flag 名、WR-01 六形态、warn 合并不遮蔽）、八 UAT 协议脚本 exit 全 0 且 PASS 计数对齐基线（12/18/10/28/23/34/21/18）、文档口径（CONFIGURATION session-mode×5 + 共 30 键/全部 30 个配置键 + 装配中×2、README×1、--help 一致）。唯一环境适配：本机默认 bind 解析非 loopback 被安全闸拒绝，listening on 腿显式 --bind 127.0.0.1（产品行为正确——安全闸生效本身即为证据）。

## Summary

total: 22
passed: 22
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
