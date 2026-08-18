// CORE-03 标题 sanitize 纯函数（UI-SPEC §Title Sync Contract 逐字契约 + D-03）：
// 剥离 C0(U+0000-001F)/DEL(U+007F)/C1(U+0080-009F) 控制码点 → 按 code point 截断前 128 个
// （Array.from 语义迭代，不拆 surrogate pair）→ 空串回退 'wesh'（不清空标签页标题）。
// 远程 OSC 0/2 内容是不可信输入（T-04-04 标题注入伪装主机名/路径钓鱼面）——
// document.title 的唯一写口必须经本函数，禁止旁路直写。
// 抽出为纯函数便于 node --test 直跑（Node 24 内建 type stripping，零新依赖，RESEARCH §A3）。
export function sanitizeTitle(raw: string): string {
  const stripped = Array.from(raw).filter((ch) => {
    const cp = ch.codePointAt(0)!;
    return !(cp <= 0x1f || cp === 0x7f || (cp >= 0x80 && cp <= 0x9f));
  });
  return stripped.slice(0, 128).join('') || 'wesh';
}
