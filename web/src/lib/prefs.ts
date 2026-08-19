// FE-07 偏好白名单与 query/prefs 解析纯函数（node --test 可测——零 DOM 依赖，RESEARCH §A3）。
// 白名单与 Go 侧 proto.ValidClientOptionKey 语义同源（D-14：两侧同白名单，
// allowProposedApi 等危险面由同一白名单结构性排除）。

// 8 个 xterm 视觉键（ITerminalOptions 运行时可赋值）
export const XTERM_PREF_KEYS: readonly string[] = [
  'fontSize',
  'fontFamily',
  'cursorBlink',
  'cursorStyle',
  'scrollback',
  'lineHeight',
  'letterSpacing',
  'theme',
];

// FE-06 两开关（非 xterm 选项——只写前端开关量，禁止写 term.options）
export const BEHAVIOR_PREF_KEYS: readonly string[] = ['resizeOverlay', 'confirmBeforeUnload'];

// 解析 location.search：白名单内键逐键 JSON.parse，成功项记入 keys 集合并入 prefs；
// 白名单外键与非法 JSON 静默跳过（非法键记入 invalid 由调用方 console.warn——纯函数不打日志）。
// osc52 结构性不在白名单（D-12 安全不对称——URL query 永不可开启 OSC52）；
// 非法静默忽略是 D-16：用户侧输入不该让终端不可用。
export function parseQueryPrefs(search: string): {
  prefs: Record<string, unknown>;
  keys: Set<string>;
  invalid: string[];
} {
  const prefs: Record<string, unknown> = {};
  const keys = new Set<string>();
  const invalid: string[] = [];
  for (const [k, v] of new URLSearchParams(search)) {
    if (!XTERM_PREF_KEYS.includes(k) && !BEHAVIOR_PREF_KEYS.includes(k)) {
      continue; // 白名单外键（含 osc52）——结构性忽略
    }
    try {
      prefs[k] = JSON.parse(v);
      keys.add(k);
    } catch {
      invalid.push(k); // 非法 JSON——忽略并记录（调用方 warn）
    }
  }
  return { prefs, keys, invalid };
}

// 按键名分派 xterm 组与 behavior 组；osc52 不属于任一组——由调用方单独处理（D-12）
export function splitPrefs(prefs: Record<string, unknown>): {
  xterm: Record<string, unknown>;
  behavior: Record<string, unknown>;
} {
  const xterm: Record<string, unknown> = {};
  const behavior: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(prefs)) {
    if (XTERM_PREF_KEYS.includes(k)) {
      xterm[k] = v;
    } else if (BEHAVIOR_PREF_KEYS.includes(k)) {
      behavior[k] = v;
    }
  }
  return { xterm, behavior };
}

// theme 合并：部分 theme 与 base 调色板合并（未指定键保留 base）——RESEARCH §Pitfall 3 修正：
// xterm 运行时 theme 赋值对未指定键回退 xterm 内建默认而非保留现值；构造段（query 通道）与
// WELCOME 分支（prefs 通道）theme 合并同源经此函数，两通道行为一致。
// incoming 非普通对象（null/数组/非 object）返回 null——中和或忽略由调用方决定。
export function mergeTheme(base: Record<string, string>, incoming: unknown): Record<string, string> | null {
  if (typeof incoming !== 'object' || incoming === null || Array.isArray(incoming)) {
    return null;
  }
  return { ...base, ...(incoming as Record<string, string>) };
}
