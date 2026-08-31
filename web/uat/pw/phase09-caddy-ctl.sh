#!/usr/bin/env bash
# Phase 09 UAT 控制脚本（Linux 侧）：专用一次性 Caddy + wesh 实例生命周期（09-08 D-15 双机实证）
# 拓扑（phase07-a2-ctl.sh 形态先例，nginx 换 Caddy）：Windows 工作站 Playwright
#   → LAN http://<linux-host>:10014（Caddy reverse_proxy）→ loopback wesh :17682
#   （一次性测试凭据经 WESH_CREDENTIAL env 递交——setup 时由 pw 侧经 ssh stdin
#   传入（WESH_UAT_CRED 覆盖机制两侧同源），不进 argv/ps 可见面；LAN 暴露面仅
#   存在于实证窗口期，T-09-08c）。
# Caddy 行为面（09-08 Linux 协议层实证锚点，Pitfall 6——与 nginx 默认语义相反，配方互抄必错）：
#   reverse_proxy 默认原样透传 Host（wesh Origin 同源校验天然过——本文件不配任何 Host 改写行）；
#   WS upgrade 内建自动处理；hijack 后无默认 WS idle 超时（65s 空闲存活实测，见 09-08 SUMMARY）。
# Caddy v2.11.4 官方静态二进制直装（GitHub release tar.gz——禁 apt 纪律不涉服务端软件；
#   仅实证环境 /tmp，不入仓）；二进制已存在则复用（setup 幂等）。
# 红线：一次性测试凭据只作构造材料，脚本输出不含凭据值。
# 用法: phase09-caddy-ctl.sh setup | probe | teardown
set -u
CADDY_DIR=/tmp/wesh-uat/caddy
CADDY_BIN=$CADDY_DIR/caddy
CADDYFILE=$CADDY_DIR/Caddyfile-lan
WESH_BIN=/tmp/wesh-uat/wesh
WESH_PORT=17682
CADDY_PORT=10014
WESH_LOG=/tmp/wesh-uat/caddy-wesh.log
CADDY_LOG=/tmp/wesh-uat/caddy-run.log
WESH_PIDF=/tmp/wesh-uat/caddy-wesh.pid
CADDY_PIDF=/tmp/wesh-uat/caddy-run.pid
CADDY_URL=https://github.com/caddyserver/caddy/releases/download/v2.11.4/caddy_2.11.4_linux_amd64.tar.gz

write_caddyfile() { # LAN 监听形态：裸 :PORT 站点地址 = 绑定全部网卡 + 匹配任意 Host。
  # 实证勘误（09-08 Windows 首跑捕获）：0.0.0.0:PORT 在 Caddy 中是字面 Host 匹配
  # （仅 Host: 0.0.0.0 命中），真实主机名请求落空 → Caddy 兜底空 200——Task 1
  # 「外部 Host 照常被服务」结论系 probe 恰好 curl 0.0.0.0 的假绿，与 nginx 语义相反。
  cat > $CADDYFILE <<EOF
{
	admin off
}

http://:$CADDY_PORT {
	reverse_proxy 127.0.0.1:$WESH_PORT
}
EOF
}

case "${1:-}" in
  setup)
    # 预清理（a2 先例）：历史泄漏实例按端口属主清杀
    fuser -k $CADDY_PORT/tcp >/dev/null 2>&1
    fuser -k $WESH_PORT/tcp >/dev/null 2>&1
    sleep 0.5
    [ -x $WESH_BIN ] || { echo WESH_BIN_MISSING; exit 1; }
    # Caddy 二进制部署（GitHub release 官方静态二进制；幂等——已存在则复用）
    mkdir -p $CADDY_DIR
    if [ ! -x $CADDY_BIN ]; then
      curl -fsSL --connect-timeout 15 -o $CADDY_DIR/caddy.tar.gz "$CADDY_URL" || { echo CADDY_DOWNLOAD_FAIL; exit 1; }
      tar -xzf $CADDY_DIR/caddy.tar.gz -C $CADDY_DIR caddy || { echo CADDY_EXTRACT_FAIL; exit 1; }
      rm -f $CADDY_DIR/caddy.tar.gz
    fi
    $CADDY_BIN version | grep -q '^v2\.11\.4 ' || { echo CADDY_VERSION_MISMATCH; exit 1; }
    write_caddyfile
    # rig 凭据（09-review WR-04）：pw 侧经 ssh stdin 递交（单一事实源——pw 侧 CRED/
    # WESH_UAT_CRED 覆盖机制两侧同源生效）；stdin 非 TTY 时读首行，空读（ctl 手跑/
    # 无管道）回落一次性默认 user:pass。经 WESH_CREDENTIAL env 递交 wesh（不进
    # argv——ps 不可见，README「凭据勿走 ps 可见面」生产指引同向）。
    RIG_CRED=user:pass
    if [ ! -t 0 ]; then
      RIG_CRED=$(head -n1)
      RIG_CRED=${RIG_CRED:-user:pass}
    fi
    # wesh 实例：loopback only——LAN 暴露面只经 Caddy（T-09-08c）
    WESH_CREDENTIAL=$RIG_CRED nohup $WESH_BIN --bind 127.0.0.1 --port $WESH_PORT --writable -- bash --norc --noprofile >$WESH_LOG 2>&1 &
    echo $! > $WESH_PIDF
    sleep 0.8
    grep -q "listening on" $WESH_LOG || { echo WESH_START_FAIL; tail -3 $WESH_LOG; exit 1; }
    nohup $CADDY_BIN run --config $CADDYFILE >$CADDY_LOG 2>&1 &
    echo $! > $CADDY_PIDF
    sleep 0.8
    ss -ltn | grep -q ":$WESH_PORT " || { echo WESH_LISTEN_FAIL; exit 1; }
    ss -ltn | grep -q ":$CADDY_PORT " && echo CADDY_UP || { echo CADDY_FAIL; tail -5 $CADDY_LOG; exit 1; }
    ;;
  probe) # 端口计数回读（Windows 侧 teardown 归零断言通道）
    C=$(ss -ltn | grep -c ":$CADDY_PORT " || true)
    W=$(ss -ltn | grep -c ":$WESH_PORT " || true)
    echo "listen_caddy=$C listen_wesh=$W"
    ;;
  teardown)
    [ -f $CADDY_PIDF ] && kill -9 "$(cat $CADDY_PIDF)" 2>/dev/null
    [ -f $WESH_PIDF ] && kill -9 "$(cat $WESH_PIDF)" 2>/dev/null
    rm -f $WESH_PIDF $CADDY_PIDF $WESH_LOG $CADDY_LOG $CADDYFILE
    echo TORN_DOWN
    ;;
  *) echo "usage: phase09-caddy-ctl.sh setup|probe|teardown"; exit 2;;
esac
