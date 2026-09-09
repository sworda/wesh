#!/usr/bin/env bash
# wesh 本地构建脚本：前端构建（pnpm -C web build）先于 go build——web/dist 经
# go:embed 内嵌进二进制，顺序不可颠倒（D-18）。产物写入仓库根 wesh（.gitignore 已忽略）。
#
# usage: scripts/build.sh [--skip-web] [output]
#   --skip-web  跳过前端构建（web/src 未变、dist 新鲜时用；仓库含 dist 占位保证可编译）
#   output      输出路径，默认仓库根 wesh
set -euo pipefail

SKIP_WEB=0
OUT="wesh"
while [ $# -gt 0 ]; do
    case "$1" in
        --skip-web) SKIP_WEB=1 ;;
        -*) echo "build: unknown flag: $1" >&2; exit 2 ;;
        *) OUT="$1" ;;
    esac
    shift
done

cd "$(dirname "$0")/.."

if [ "$SKIP_WEB" -eq 0 ]; then
    command -v pnpm >/dev/null 2>&1 || { echo "build: pnpm not found" >&2; exit 1; }
    pnpm -C web install --frozen-lockfile
    time pnpm -C web build
fi

# 与发布口径一致（.goreleaser.yml）：CGO_ENABLED=0 + -trimpath。
# 三元组显式注入：version 取 git describe；commit 取 HEAD；builtAt 取真实构建
# 时刻（date +%s 的 Unix 秒，formatBuiltAt 消费口径同源）。三者缺一不可——若
# 只注入 version，resolveVCS（cmd/wesh/main.go:48，以 commit=="none" 为闸）会
# 用 buildinfo 的 vcs.time 回填 builtAt，--version 显示 HEAD commit 时间而非
# 构建时刻（本地构建无需可复现性，按 "built" 字段语义显示真实构建时间）。
# 脏树时 commit 追加 -dirty（与 version 的 describe --dirty、裸 go build 的
# vcs.modified 口径一致）——二进制内容并不对应该裸 revision。判定用
# git status --porcelain（含 untracked：untracked 文件同样可能被 go:embed
# 编进二进制，口径比 describe --dirty 的 tracked-only 更严）。
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT=$(git rev-parse HEAD 2>/dev/null || echo none)
if [ "$COMMIT" != "none" ] && [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    COMMIT="$COMMIT-dirty"
fi
time CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.builtAt=$(date +%s)" \
    -o "$OUT" ./cmd/wesh
echo "build: $OUT (version $VERSION)"
