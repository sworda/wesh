// phase05-dom.mjs D5 的洪水驱动子进程（独立事件循环——主进程 Atomics.wait 阻塞
// 期间本进程 socket 持续 drain，不随主进程 stall，与被测 jsdom 客户端形成快/慢对照）。
// 用法：node phase05-flood-driver.mjs <port>
// stdout 协议：FLOOD_STARTED（Welcome 到达且 seq 已发）/ BYTES <n>（500ms 进度）/
// CLOSED <code>（被关闭——本场景预期永不出现：快端不被踢）。
const port = Number(process.argv[2]);
const enc = new TextEncoder();
const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, ['wesh.v1']);
ws.binaryType = 'arraybuffer';
let bytes = 0;
ws.onopen = () => ws.send(new Uint8Array([0x48, ...enc.encode(JSON.stringify({ version: 'wesh.v1', cols: 80, rows: 24 }))]));
ws.onmessage = (ev) => {
  const f = new Uint8Array(ev.data);
  if (f[0] === 0x30) { bytes += f.length; return; }
  if (f[0] === 0x57) {
    ws.send(new Uint8Array([0x30, ...enc.encode('seq 1 3000000\n')]));
    console.log('FLOOD_STARTED');
  }
};
setInterval(() => console.log(`BYTES ${bytes}`), 500).unref();
ws.onclose = (ev) => { console.log(`CLOSED ${ev.code}`); process.exit(0); };
ws.onerror = () => { console.log('WS_ERROR'); process.exit(1); };
