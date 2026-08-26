#!/usr/bin/env bash
# Phase 07 UAT A2 控制脚本（Linux 侧）：专用一次性 nginx + wesh 实例生命周期
# 用法: a2-ctl.sh setup | variant exact|noexact | teardown
set -u
NGINX_DIR=/tmp/wesh-uat/a2-nginx
WESH_PORT=17699
NGINX_PORT=10013

conf() { # $1 = exact|noexact
  local exact="$1"
  {
  cat <<EOF
worker_processes 1;
pid $NGINX_DIR/nginx.pid;
error_log $NGINX_DIR/error.log warn;
events { worker_connections 256; }
http {
  access_log $NGINX_DIR/access.log;
  client_body_temp_path $NGINX_DIR/tmp/body;
  proxy_temp_path $NGINX_DIR/tmp/proxy;
  fastcgi_temp_path $NGINX_DIR/tmp/fastcgi;
  uwsgi_temp_path $NGINX_DIR/tmp/uwsgi;
  scgi_temp_path $NGINX_DIR/tmp/scgi;
  map \$http_upgrade \$connection_upgrade { default upgrade; "" close; }
  server {
    listen 0.0.0.0:$NGINX_PORT;
    auth_basic "wesh-a2";
    auth_basic_user_file $NGINX_DIR/htpasswd;
EOF
  [ "$exact" = "exact" ] && echo '    location = /wesh { return 308 /wesh/; }'
  cat <<'EOF'
    location /wesh/ {
      proxy_pass http://127.0.0.1:WESH_PORT_PH;
      proxy_http_version 1.1;
      proxy_set_header Host $http_host;
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection $connection_upgrade;
      proxy_read_timeout 3600s;
    }
  }
}
EOF
  } | sed "s/WESH_PORT_PH/$WESH_PORT/" > $NGINX_DIR/nginx.conf
}

case "${1:-}" in
  setup)
    # 预清理：历史泄漏实例——按端口属主清杀（覆盖孤儿 worker：master 被 kill -9 后
    # worker 存活持端口且 cmdline 无路径不可模式匹配；fuser -k 属主权限可及）
    fuser -k $NGINX_PORT/tcp >/dev/null 2>&1
    fuser -k $WESH_PORT/tcp >/dev/null 2>&1
    sleep 0.5
    rm -rf $NGINX_DIR; mkdir -p $NGINX_DIR/tmp
    # UAT 一次性 Basic 凭据（test-only；LAN 暴露窗口内的最小防护，phase06-pw 先例同口径）
    printf 'user:%s\n' "$(openssl passwd -apr1 'pass')" > $NGINX_DIR/htpasswd
    nohup /tmp/wesh-uat/wesh --bind 127.0.0.1 --port $WESH_PORT --base-path /wesh --writable --credential user:pass -- bash --norc --noprofile >/tmp/wesh-uat/a2-wesh.log 2>&1 &
    echo $! > /tmp/wesh-uat/a2-wesh.pid
    sleep 0.8
    grep -q "listening on" /tmp/wesh-uat/a2-wesh.log || { echo WESH_START_FAIL; tail -3 /tmp/wesh-uat/a2-wesh.log; exit 1; }
    conf exact
    nginx -t -p $NGINX_DIR -c nginx.conf 2>&1 | tail -1
    nginx -p $NGINX_DIR -c nginx.conf || { echo NGINX_START_FAIL; exit 1; }
    sleep 0.3
    ss -ltn | grep -q ":$NGINX_PORT " && echo NGINX_UP || { echo NGINX_FAIL; tail -5 $NGINX_DIR/error.log; exit 1; }
    ;;
  variant) # $2 = exact|noexact
    conf "$2"
    nginx -t -p $NGINX_DIR -c nginx.conf 2>&1 | tail -1
    nginx -p $NGINX_DIR -c nginx.conf -s reload || { echo NGINX_RELOAD_FAIL; exit 1; }
    sleep 0.3
    echo "RELOADED_$2"
    ;;
  probe)
    C=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 http://127.0.0.1:$NGINX_PORT/wesh 2>/dev/null)
    N=$(ps aux | grep -v grep | grep -c a2-nginx)
    L=$(ss -ltn | grep -c ":$NGINX_PORT ")
    W=$(ps aux | grep -v grep | grep -c "port $WESH_PORT")
    echo "http=$C nginx_procs=$N listen=$L wesh_procs=$W"
    ;;
  diag)
    tail -8 $NGINX_DIR/error.log 2>/dev/null
    echo ---
    ss -ltn | grep -E "$NGINX_PORT|$WESH_PORT" || echo NO_LISTENERS
    echo ---
    ps aux | grep -v grep | grep -E "a2-nginx|port $WESH_PORT" | head -5 || echo NO_PROCS
    ;;
  teardown)
    nginx -p $NGINX_DIR -c nginx.conf -s quit 2>/dev/null
    [ -f /tmp/wesh-uat/a2-wesh.pid ] && kill -9 "$(cat /tmp/wesh-uat/a2-wesh.pid)" 2>/dev/null
    rm -rf $NGINX_DIR /tmp/wesh-uat/a2-wesh.log /tmp/wesh-uat/a2-wesh.pid
    echo TORN_DOWN
    ;;
  *) echo "usage: a2-ctl.sh setup|variant exact|noexact|teardown"; exit 2;;
esac
