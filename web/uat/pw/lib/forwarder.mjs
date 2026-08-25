// TCP 转发器：浏览器 ↔ wesh 服务端之间的可控断点。
// killNet() 毁掉全部在飞连接（双端 RST → 浏览器 WS 合成 1006），此后新连接即接即毁；
// restore() 恢复正常转发。用于在浏览器侧忠实模拟断网的可观测语义（真实 OS 网卡栈时序
// 仍属平台豁免，见根 CODEBUDDY.md）。
import net from 'node:net';

export class Forwarder {
  constructor(listenPort, targetHost, targetPort = 7681) {
    this.listenPort = listenPort;
    this.targetHost = targetHost;
    this.targetPort = targetPort;
    this.up = false;
    this.pairs = new Set();
    this.server = null;
  }

  start() {
    return new Promise((resolve, reject) => {
      this.server = net.createServer((client) => {
        if (!this.up) { client.destroy(); return; }
        const upstream = net.connect(this.targetPort, this.targetHost);
        const pair = { client, upstream };
        this.pairs.add(pair);
        const cleanup = () => {
          this.pairs.delete(pair);
          client.destroy();
          upstream.destroy();
        };
        client.on('error', cleanup);
        upstream.on('error', cleanup);
        client.on('close', cleanup);
        upstream.on('close', cleanup);
        client.pipe(upstream);
        upstream.pipe(client);
      });
      this.server.on('error', reject);
      this.server.listen(this.listenPort, '127.0.0.1', () => {
        this.up = true;
        resolve();
      });
    });
  }

  killNet() {
    this.up = false;
    for (const p of [...this.pairs]) {
      p.client.destroy();
      p.upstream.destroy();
    }
    this.pairs.clear();
  }

  restore() {
    this.up = true;
  }

  async stop() {
    this.killNet();
    if (this.server) await new Promise((r) => this.server.close(r));
  }
}
