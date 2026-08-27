#!/usr/bin/env bash
# Phase 07 UAT B1/B5 自动化证据脚本（Linux 侧运行）v2
# B1: 同 socket 并发启动 → plan 设计答案 EADDRINUSE exit 1；实测记录真实行为；
#     SIGKILL 真实残留 → 自动清理启动成功
# B5: TOML 语法变体加载 / 重复键 exit 2 / command=[] 等价缺席 exit 2
set -u
WESH=/tmp/wesh-uat/wesh
D=$(mktemp -d /tmp/wesh-uat/b1b5.XXXXXX)
PASS=0; FAIL=0
cleanup() { kill -9 $P1 $P3 $PV 2>/dev/null; rm -rf "$D"; }
trap cleanup EXIT
ok() { if [ "$2" = "$3" ]; then echo "  PASS  $1 ($2)"; PASS=$((PASS+1)); else echo "  FAIL  $1 (want=$3 got=$2)"; FAIL=$((FAIL+1)); fi; }
P1=""; P3=""; PV=""

echo "B1: 同 socket 并发中断语义"
SOCK=$D/wesh.sock
"$WESH" --socket "$SOCK" -- bash --norc --noprofile >"$D/a.log" 2>&1 &
P1=$!
for i in $(seq 1 50); do grep -q "listening on unix" "$D/a.log" && break; sleep 0.1; done
grep -q "listening on unix" "$D/a.log" || { echo "  FAIL  B1 setup: 实例1未启动"; cat "$D/a.log"; exit 1; }

# 实例2（存活实例同路径）：timeout 护栏防挂——rc=1=EADDRINUSE（plan 设计答案）；
# rc=124 且日志含 listening = 静默赢者（与设计背离，实录）
timeout 5 "$WESH" --socket "$SOCK" -- bash >"$D/b.log" 2>&1
RC=$?
if [ "$RC" = "1" ] && grep -q "address already in use" "$D/b.log"; then
  ok "B1a 存活实例同 socket 第二实例 exit 1 EADDRINUSE(设计答案)" "exit1-eaddrinuse" "exit1-eaddrinuse"
elif [ "$RC" = "124" ] && grep -q "listening on unix" "$D/b.log"; then
  ok "B1a 实测背离:第二实例静默赢者(unlink 存活 socket 后 listen 成功)" "silent-winner" "exit1-eaddrinuse"
else
  ok "B1a 第二实例行为不在预期枚举" "rc=$RC" "exit1-eaddrinuse"
fi
echo "  [证据] b.log 首行: $(head -1 "$D/b.log")"
# 若实例2 仍活着（静默赢者场景）补杀
pgrep -f "wesh[.]sock -- bash\$" | while read -r p; do [ "$p" != "$P1" ] && kill -9 "$p" 2>/dev/null; done

kill -9 $P1 2>/dev/null; wait $P1 2>/dev/null; P1=""
sleep 0.3
[ -S "$SOCK" ]
ok "B1c SIGKILL 后 socket 文件真实残留" "$?" "0"

"$WESH" --socket "$SOCK" -- bash --norc --noprofile >"$D/c.log" 2>&1 &
P3=$!
for i in $(seq 1 50); do grep -q "listening on unix" "$D/c.log" && break; sleep 0.1; done
grep -q "listening on unix" "$D/c.log"
ok "B1d 残留 socket 自动清理(listen 前 Remove)启动成功" "$?" "0"
kill -9 $P3 2>/dev/null; wait $P3 2>/dev/null; P3=""

echo "B5: TOML 语法变体与空 command"
cat >"$D/v.toml" <<'EOF'
bind = "127.0.0.1"
"port" = 0
command = [
  "bash",
  "--norc",
  "--noprofile",
]
EOF
"$WESH" --config "$D/v.toml" >"$D/v.log" 2>&1 &
PV=$!
for i in $(seq 1 50); do grep -q "listening on http" "$D/v.log" && break; sleep 0.1; done
grep -q "listening on http" "$D/v.log"
ok "B5a 多行数组+引用键配置正常加载" "$?" "0"
kill -9 $PV 2>/dev/null; wait $PV 2>/dev/null; PV=""

cat >"$D/dup.toml" <<'EOF'
bind = "127.0.0.1"
bind = "0.0.0.0"
command = ["bash"]
EOF
"$WESH" --config "$D/dup.toml" >"$D/dup.log" 2>&1
RC=$?
ok "B5b 配置重复键 exit 2(TOML 解析器规范拒绝)" "$RC" "2"
echo "  [证据] dup.log: $(head -1 "$D/dup.log")"

cat >"$D/empty.toml" <<'EOF'
bind = "127.0.0.1"
port = 0
command = []
EOF
"$WESH" --config "$D/empty.toml" >"$D/empty.log" 2>&1
RC=$?
ok "B5c command=[] 等价缺席 exit 2" "$RC" "2"
grep -qi "missing command" "$D/empty.log"
ok "B5d 错误类别 missing command(与 CLI 空 argv 同档)" "$?" "0"
echo "  [证据] empty.log: $(head -1 "$D/empty.log")"

echo "结果: $PASS PASS, $FAIL FAIL"
[ $FAIL -eq 0 ]
