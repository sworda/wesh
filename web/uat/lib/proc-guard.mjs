// UAT 子进程清理兜底模块（零依赖，Node >= 22 原生 child_process/fs）——进程泄漏防线。
//
// 【由来】UAT 脚本（web/uat/phaseNN.mjs）spawn 真实 wesh 二进制做端到端断言；既
//   有清理依赖各场景自己的 try/finally，覆盖不到「脚本被 Ctrl-C / 超时 SIGTERM /
//   顶层未捕获异常直接终止」三条路径——这三种形态下 node 进程已死、wesh 子进程
//   被 init 收养转而长期存活（实测：21 个 /tmp/wesh-uat/wesh 实例滞留 31 天，各自
//   占着 PTY 与随机端口，二进制被 /tmp 清理后仍以 exe=(deleted) 形态空转）。本模块
//   即该漏洞的收口：登记 + 退出钩子强制收割 + 跑后 /proc 残留扫描核验。
//
// 【用法】脚本顶部一行接入，随后把 spawn( 调用点改为 spawnTracked(（drop-in 等价，
//   仅多一步登记）：
//     import { spawnTracked } from './lib/proc-guard.mjs';
//     const child = spawnTracked(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], {...});
//   模块被 import 即自动安装退出钩子（installProcGuard 幂等，重复 import 无副作用）。
//   跑完可选核验：leakedPids(WESH) 返回仍存活的同二进制进程 pid 数组（零命中即清洁）。
//
// 【收割语义】逐个 SIGKILL（不可忽略不可捕获——测试进程无优雅退出语义）；detached
//   子进程自成进程组且可能已有孙进程（run-all.mjs 的 node 子脚本 → 其 wesh 实例），
//   此类先 kill(-pgid) 整组收割再 kill(pid) 单点兜底——否则子脚本被 SIGKILL 后其
//   孙进程无人认领，正是本次泄漏的同源形态。
//
// 【红线】只收割本进程自己 spawn 的登记子进程——绝不按二进制路径跨进程 kill（并行
//   跑多个 UAT 脚本时会误杀他人实例，D-09 零污染纪律的同源约束）。leakedPids 仅做
//   只读核验，报告归调用方处置。
//
// 【运行】node web/uat/phaseNN.mjs（随脚本自动生效，无独立入口）

import { spawn } from 'node:child_process';
import fs from 'node:fs';

// 登记表：child -> { label, detached }（label 为命令行首参，仅诊断用、不进任何输出）
const tracked = new Map();

let guardInstalled = false;

// spawn 的 drop-in 包装：语义逐字等同 child_process.spawn，额外登记并在 exit 时自摘牌
export function spawnTracked(...args) {
  const child = spawn(...args);
  const detached = args[2]?.detached === true;
  tracked.set(child, { label: typeof args[0] === 'string' ? args[0] : '?', detached });
  child.once('exit', () => tracked.delete(child));
  return child;
}

// 强制收割：登记子进程逐个 SIGKILL（exit 钩子内同步投递即可——内核保证投递，父进程
// 退出后不残留长活实例）；detached 者额外整组收割，覆盖其孙进程
export function reapAll(reason) {
  if (tracked.size === 0) return 0;
  const n = tracked.size;
  for (const [child, meta] of tracked) {
    if (meta.detached) {
      try { process.kill(-child.pid, "SIGKILL"); } catch { /* 组已消亡或非组长 */ }
    }
    try { child.kill("SIGKILL"); } catch { /* 已退出 */ }
  }
  tracked.clear();
  if (reason) process.stderr.write(`[proc-guard] ${reason}——已收割 ${n} 个登记子进程\n`);
  return n;
}

// 残留扫描：遍历 /proc 读 exe 符号链接，命中 binary 前缀者即同二进制存活实例
// （二进制被删时为 "<path> (deleted)" 形态——前缀匹配天然覆盖，正是泄漏态特征）
export function leakedPids(binary) {
  const out = [];
  let entries;
  try { entries = fs.readdirSync('/proc'); } catch { return out; }
  for (const name of entries) {
    if (!/^[0-9]+$/.test(name)) continue;
    const pid = Number(name);
    if (pid === process.pid) continue;
    let exe;
    try { exe = fs.readlinkSync(`/proc/${pid}/exe`); } catch { continue; }
    if (exe.startsWith(binary)) out.push(pid);
  }
  return out;
}

// 退出钩子安装（幂等）：四类终止路径全覆盖——正常 exit / 交互信号 / 未捕获异常 / 未处理拒绝
export function installProcGuard() {
  if (guardInstalled) return;
  guardInstalled = true;

  process.on('exit', () => reapAll());

  for (const sig of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
    process.on(sig, () => {
      reapAll(`收到 ${sig}`);
      process.exit(128 + { SIGINT: 2, SIGTERM: 15, SIGHUP: 1 }[sig]);
    });
  }

  process.on('uncaughtException', (e) => {
    reapAll('未捕获异常');
    process.stderr.write(`[proc-guard] uncaughtException: ${e?.stack ?? e}\n`);
    process.exit(1);
  });

  process.on('unhandledRejection', (e) => {
    reapAll('未处理 Promise 拒绝');
    process.stderr.write(`[proc-guard] unhandledRejection: ${e?.stack ?? e}\n`);
    process.exit(1);
  });
}

installProcGuard();
