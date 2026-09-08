#!/usr/bin/env node
// mermaid 词法校验载具（G-14-34 防复发件——14-11 结构化校验只查结构完整性不查词法的漏检面堵口）。
//
// Linux 开发机禁浏览器/playwright/X11（CODEBUDDY.md 双机拓扑硬约束）：mermaid 语法验证只能走
// Node 端 mermaid.parse（jsdom 注入 window/document 后 dynamic import mermaid），
// 禁止 mmdc/mermaid-cli（依赖 puppeteer/chromium）。
//
// 用法：
//   node scripts/check-mermaid.mjs [file.md ...]   # 指定 .md 文件列表（相对 cwd）
//   node scripts/check-mermaid.mjs                 # 无参时默认扫描仓库 docs/ 目录全部 .md
//
// 输出：每块一行判定（文件名 + 块序号 + PASS/FAIL + diagramType）；FAIL 行附错误消息首行；
//       零 mermaid 块的文件输出跳过说明，不计失败。
// 退出码：任一块 FAIL → 1；全部 PASS（或无块）→ 0。
import { readFileSync, readdirSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

// mermaid.parse 内部经 DOMPurify 需要 window/document —— 先注入再动态 import
// （顺序不可颠倒：静态顶层 import 会先于注入求值而崩）
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { pretendToBeVisual: true });
globalThis.window = dom.window;
globalThis.document = dom.window.document;
if (!globalThis.navigator) {
  globalThis.navigator = dom.window.navigator;
}

const { default: mermaid } = await import('mermaid');

const scriptDir = fileURLToPath(new URL('.', import.meta.url));
const repoDocs = resolve(scriptDir, '../docs');

const files = process.argv.length > 2
  ? process.argv.slice(2)
  : readdirSync(repoDocs)
      .filter((f) => f.endsWith('.md'))
      .map((f) => join(repoDocs, f))
      .sort();

const MERMAID_BLOCK = /```mermaid\n([\s\S]*?)```/g;

let failed = false;
let total = 0;

for (const file of files) {
  const md = readFileSync(file, 'utf8');
  const blocks = [...md.matchAll(MERMAID_BLOCK)].map((m) => m[1]);
  if (blocks.length === 0) {
    console.log(`${file}: no mermaid blocks, skipped`);
    continue;
  }
  for (let i = 0; i < blocks.length; i++) {
    total++;
    const label = `${file} block ${i + 1}`;
    try {
      const r = await mermaid.parse(blocks[i]);
      const diagramType = r === true ? 'true' : (r?.diagramType ?? 'n/a');
      console.log(`${label}: PASS (diagramType=${diagramType})`);
    } catch (e) {
      failed = true;
      const firstLine = String(e.message ?? e).split('\n')[0];
      console.log(`${label}: FAIL — ${firstLine}`);
    }
  }
}

console.log(`\ntotal: ${total} block(s), ${failed ? 'FAILED' : 'all PASS'}`);
process.exit(failed ? 1 : 0);
