#!/usr/bin/env bash
# Phase 07 UAT B6 可自动化面（Linux 侧）：--open × TLS 组合与 opener 异常兜底
# B6a: DISPLAY 设置 + stub xdg-open 记录 argv → --open --tls-cert/--tls-key 打开 https:// ro 链接
# B6b: stub xdg-open 返回非零（桌面异常 D-27）→ 仅 stderr 警告、服务不阻断（GET / 200）
# macOS open 真实弹窗面 = 人工残余（无 Mac 环境，本脚本不覆盖）
set -u
WESH=/tmp/wesh-uat/wesh
D=$(mktemp -d /tmp/wesh-uat/b6.XXXXXX)
PASS=0; FAIL=0
P=""
cleanup() { [ -n "$P" ] && kill -9 $P 2>/dev/null; rm -rf "$D"; }
trap cleanup EXIT
ok() { if [ "$2" = "$3" ]; then echo "  PASS  $1 ($2)"; PASS=$((PASS+1)); else echo "  FAIL  $1 (want=$3 got=$2)"; FAIL=$((FAIL+1)); fi; }

# 自签证书（1 天有效，UAT 一次性）
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$D/key.pem" -out "$D/cert.pem" -days 1 -subj "/CN=localhost" >/dev/null 2>&1
[ -s "$D/cert.pem" ] && [ -s "$D/key.pem" ] || { echo "  FAIL  自签证书生成失败"; exit 1; }

# stub xdg-open：记录 argv 到 $D/open.log，退出码由环境变量 STUB_RC 决定（脚本内固化）
mkdir -p "$D/bin"
cat > "$D/bin/xdg-open" <<EOF
#!/usr/bin/env bash
echo "\$@" >> "$D/open.log"
exit \${STUB_XDG_RC:-0}
EOF
chmod +x "$D/bin/xdg-open"

echo "B6: --open × TLS 组合与 opener 异常兜底"
# B6a: TLS + --open（无 --writable → ro 链接）；DISPLAY 置位使 opener 被调用
DISPLAY=:0 STUB_XDG_RC=0 PATH="$D/bin:$PATH" "$WESH" --bind 127.0.0.1 --port 0 --tls-cert "$D/cert.pem" --tls-key "$D/key.pem" --open -- bash --norc --noprofile >"$D/a.log" 2>&1 &
P=$!
for i in $(seq 1 50); do grep -q "listening on https" "$D/a.log" && break; sleep 0.1; done
grep -q "listening on https" "$D/a.log"
ok "B6a TLS 实例启动(scheme=https)" "$?" "0"
sleep 0.5
[ -s "$D/open.log" ]
ok "B6b opener 被调用(open.log 非空)" "$?" "0"
OPENURL=$(head -1 "$D/open.log" 2>/dev/null)
case "$OPENURL" in
  https://*) ok "B6c --open × TLS 打开 https:// 链接" "https" "https";;
  *)         ok "B6c --open × TLS 打开 https:// 链接" "${OPENURL%%:*}" "https";;
esac
# ro/rw 形态：无 --writable → 打开 ro 链接（与启动打印的 ro 行一致即证）
ROURL=$(grep "share read-only:" "$D/a.log" | awk '{print $NF}')
[ "$OPENURL" = "$ROURL" ]
ok "B6d opener argv == 启动打印的 ro 分享链接(无 --writable)" "$?" "0"
kill -9 $P 2>/dev/null; wait $P 2>/dev/null; P=""

# B6b: opener 返回非零 → 仅警告不阻断
rm -f "$D/open.log"
DISPLAY=:0 STUB_XDG_RC=1 PATH="$D/bin:$PATH" "$WESH" --bind 127.0.0.1 --port 0 --open -- bash --norc --noprofile >"$D/b.log" 2>&1 &
P=$!
PORT=""
for i in $(seq 1 50); do PORT=$(sed -n 's/.*listening on http:\/\/[^ ]*:\([0-9]*\).*/\1/p' "$D/b.log" | head -1); [ -n "$PORT" ] && break; sleep 0.1; done
[ -n "$PORT" ]
ok "B6e opener 非零时服务仍启动(listening 行在)" "$?" "0"
# B6f 轮询化（07-10）：listening 行出现与 goroutine 警告行落盘之间存在到达
# 竞态——opener 非零退出经 goroutine Wait 异步告警，即时 grep 可能早于落盘。
# 50×0.1s 轮询与 B1 setup/B6a 既定形态一致；断言面（B6f 语义）不变。
for i in $(seq 1 50); do grep -qi "warn" "$D/b.log" && break; sleep 0.1; done
grep -qi "warn" "$D/b.log"
ok "B6f stderr 警告行存在(D-27 不阻断)" "$?" "0"
CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$PORT/" 2>/dev/null)
ok "B6g GET / 200(服务未阻断)" "$CODE" "200"
kill -9 $P 2>/dev/null; wait $P 2>/dev/null; P=""

echo "结果: $PASS PASS, $FAIL FAIL"
[ $FAIL -eq 0 ]
