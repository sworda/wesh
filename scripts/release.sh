#!/usr/bin/env bash
# wesh 发布脚本（D-14）：把所有发布前操作整合在单一脚本内，发布之前跑一次即可。
#
# usage: scripts/release.sh [--dry-run] vX.Y.Z
#   vX.Y.Z     将创建并推送的版本 tag（严格三段式，带 v 前缀）
#   --dry-run  干跑：只执行前置校验（四闸），打印后续步骤清单，不执行任何
#              测试/构建/tag 操作
#
# 顺序：前置校验 → 全量测试（与 CI 同口径）→ 前端构建（dist 新鲜，embed 链
#   本地验证）→ 长 fuzz ×2（每目标 10 分钟；崩溃即中止——语料已自动落对应包
#   testdata/fuzz/，修复后重跑本脚本）→ 负载矩阵（30 分钟上限）→ 确认闸 →
#   git tag + push（tag push 触发 .github/workflows/release.yml：pnpm build
#   先于 goreleaser，四平台全静态产物 + checksums.txt——D-01/D-03）。
#
# 前置校验四闸（T-09-09a，闸序钉死）：① tag 形态 ② tag 不存在 ③ 工作树干净
#   ④ 与远端同步（ahead 放行——发布物本就是本地新增提交/tag；fetch 失败或
#   无上游时该闸降级为跳过提示——不伪造失败、不阻塞干跑与后续闸）。
set -euo pipefail

usage() {
    echo "usage: scripts/release.sh [--dry-run] vX.Y.Z" >&2
    exit 2
}

die() {
    echo "release: $1" >&2
    exit 1
}

DRY_RUN=0
V=""
while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) DRY_RUN=1 ;;
        -*) usage ;;
        *)
            if [ -n "$V" ]; then usage; fi
            V="$1"
            ;;
    esac
    shift
done
if [ -z "$V" ]; then usage; fi

# preflight：发布前置校验四闸。钉死文案是干跑四态机械断言的分流依据；
# 闸序不可调换（形态/已存在两闸先于脏树闸）。
preflight() {
    # 闸①：tag 形态 vX.Y.Z
    if ! printf '%s\n' "$V" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
        die "invalid tag format (want vX.Y.Z)"
    fi
    # 闸②：tag 不可重复
    if git rev-parse -q --verify "refs/tags/$V" >/dev/null 2>&1; then
        die "tag already exists"
    fi
    # 闸③：脏树禁发布（发布物须与仓库内容一致）
    if [ -n "$(git status --porcelain)" ]; then
        die "working tree not clean"
    fi
    # 闸④：与远端同步（落后/分叉即拒，ahead 放行）。降级钉死：fetch 失败
    # （无网络/远端不可达）或无上游时降级为跳过提示——同源保证无法判定时
    # 不伪造失败、不阻塞干跑与后续闸。
    if ! git fetch --dry-run >/dev/null 2>&1; then
        echo "release: upstream check skipped (no network or upstream)"
        return 0
    fi
    if ! git rev-parse -q --verify '@{u}' >/dev/null 2>&1; then
        echo "release: upstream check skipped (no network or upstream)"
        return 0
    fi
    local behind
    behind=$(git rev-list --count 'HEAD..@{u}')
    if [ "$behind" -gt 0 ]; then
        die "branch is behind upstream by $behind commit(s); pull or rebase first"
    fi
}

# 全量测试：go vet + 全量 race 测试（与 CI go leg 同口径）
run_tests() {
    go vet ./...
    go test -race -count=1 ./...
}

# 前端构建：dist 新鲜，embed 链本地验证（前端构建先于 go build，P1 D-18）
build_web() {
    pnpm -C web install --frozen-lockfile
    time pnpm -C web build
}

# 长 fuzz：两目标两次独立调用（go 工具链 -fuzz 单包单目标约束）
run_fuzz() {
    go test -fuzz=FuzzDecodeHello -fuzztime=10m ./internal/proto/
    go test -fuzz=FuzzDecodeFileConfig -fuzztime=10m ./cmd/wesh/
}

# 负载矩阵：build tag 隔离的黑盒负载测试（30 分钟上限）
run_load() {
    go test -tags=load -count=1 -timeout=30m ./internal/server/
}

# 确认闸：回显将创建的 tag 与最近提交，应答非 yes 即中止（人因防线，
# 回显内容供发布者最后核对版本号与提交内容）。
confirm() {
    echo
    echo "About to create and push tag: $V"
    echo "Recent commits:"
    git log --oneline -5 | sed 's/^/  /'
    echo
    echo "Pushing this tag triggers .github/workflows/release.yml"
    echo "(pnpm build, then goreleaser: four-platform static binaries + checksums.txt)."
    echo
    printf 'Type "yes" to continue: '
    local answer
    if ! read -r answer; then
        die "aborted at confirm gate (no input)"
    fi
    if [ "$answer" != "yes" ]; then
        die "aborted at confirm gate"
    fi
}

preflight

if [ "$DRY_RUN" -eq 1 ]; then
    echo "release: dry run ok for $V; the steps below would run:"
    echo "  1. go vet ./... + go test -race -count=1 ./...   (full test suite)"
    echo "  2. pnpm -C web install --frozen-lockfile + pnpm -C web build   (frontend)"
    echo "  3. long fuzz 1/2: FuzzDecodeHello in ./internal/proto/ (10 minutes)"
    echo "  4. long fuzz 2/2: FuzzDecodeFileConfig in ./cmd/wesh/ (10 minutes)"
    echo "  5. load matrix in ./internal/server/ (build tag: load; 30-minute cap)"
    echo "  6. confirm gate: show tag + recent commits, proceed only on yes"
    echo "  7. git tag $V, push it to origin; release.yml takes over"
    exit 0
fi

for tool in git go pnpm; do
    command -v "$tool" >/dev/null 2>&1 || die "required tool not found: $tool"
done

run_tests
build_web
run_fuzz
run_load
confirm
git tag "$V"
# tag push 是唯一发布触发（D-01）；此后由 release.yml 接管，CI 侧不再有本地操作。
git push origin "$V"
