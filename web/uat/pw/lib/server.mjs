// wesh 服务端生命周期管理（SSH 到 Linux 侧）。
// run.sh 包装捕获 wesh 退出状态到远端 /tmp/wesh-uat/exit.status。
// 配置经环境变量（见 README.md）：WESH_UAT_SSH / WESH_UAT_SSH_PORT / WESH_UAT_REMOTE_DIR。
import { execFile } from 'node:child_process';

const SSH_DEST = process.env.WESH_UAT_SSH || '';
const SSH_PORT = process.env.WESH_UAT_SSH_PORT || '22';
const REMOTE_DIR = process.env.WESH_UAT_REMOTE_DIR || '/tmp/wesh-uat';

export const TARGET_HOST =
  process.env.WESH_UAT_TARGET_HOST || (SSH_DEST.includes('@') ? SSH_DEST.split('@')[1] : SSH_DEST);
export const TARGET_PORT = parseInt(process.env.WESH_UAT_TARGET_PORT || '7681', 10);

function requireSsh() {
  if (!SSH_DEST) {
    throw new Error(
      'WESH_UAT_SSH 未设置（形态 user@host；端口经 WESH_UAT_SSH_PORT，默认 22）。双机运行模型见 web/uat/pw/README.md',
    );
  }
}

export function ssh(remote, timeoutMs = 25000) {
  requireSsh();
  return new Promise((resolve, reject) => {
    execFile(
      'ssh',
      ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', '-p', SSH_PORT, SSH_DEST, remote],
      { timeout: timeoutMs, windowsHide: true, maxBuffer: 4 * 1024 * 1024 },
      (err, stdout, stderr) => {
        if (err) reject(new Error(`ssh failed(${err.code ?? err.message}): ${stderr.toString().trim()} ${stdout.toString().trim()}`));
        else resolve(stdout.toString());
      },
    );
  });
}

const runSh = () => `#!/bin/bash
cd ${REMOTE_DIR}
rm -f ${REMOTE_DIR}/exit.status
./wesh "$@"
echo $? > ${REMOTE_DIR}/exit.status
`;

export async function ensureRunSh() {
  const b64 = Buffer.from(runSh()).toString('base64');
  await ssh(`bash -lc 'echo ${b64} | base64 -d > ${REMOTE_DIR}/run.sh && chmod +x ${REMOTE_DIR}/run.sh'`);
}

export async function stopWesh() {
  // [w] 括号技巧：模式以正则形态出现在自身 cmdline 中但不匹配自身，防 pkill 自杀
  await ssh(`bash -lc 'pkill -f "[w]esh-uat/run\\.sh"; pkill -f "[.]\\/wesh --"; for i in $(seq 1 20); do ss -tln | grep -q ":${TARGET_PORT} " || exit 0; sleep 0.5; done; exit 0'`);
}

// argsTail: 传给 run.sh 的全部参数（含 --writable ... -- <cmd>）
export async function startWesh(argsTail) {
  await stopWesh();
  await ssh(`bash -lc '(setsid nohup ${REMOTE_DIR}/run.sh ${argsTail} > ${REMOTE_DIR}/server.log 2>&1 < /dev/null &)'`);
  for (let i = 0; i < 30; i++) {
    try {
      const out = await ssh(`bash -lc 'ss -tln | grep ":${TARGET_PORT} " || true'`, 10000);
      if (out.includes(`:${TARGET_PORT}`)) return;
    } catch { /* retry */ }
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`wesh 未在 15s 内监听 ${TARGET_PORT}；server.log: ` + (await ssh(`tail -5 ${REMOTE_DIR}/server.log`).catch(() => '?')));
}

export async function ensureNormal(credArgs) {
  try {
    const out = await ssh(`bash -lc 'ss -tln | grep ":${TARGET_PORT} " || true'`, 10000);
    if (out.includes(`:${TARGET_PORT}`)) return;
  } catch { /* fallthrough */ }
  await startWesh(`--writable --credential ${credArgs} --insecure-http -- bash`);
}

export async function exitStatus(timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const out = await ssh(`bash -lc 'cat ${REMOTE_DIR}/exit.status 2>/dev/null || true'`, 10000);
      const m = out.trim().match(/^(\d+)$/);
      if (m) return parseInt(m[1], 10);
    } catch { /* retry */ }
    await new Promise((r) => setTimeout(r, 500));
  }
  return null;
}
