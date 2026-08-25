// 轻量断言与结果收集
export class Check {
  constructor(id, name) {
    this.id = id;
    this.name = name;
    this.asserts = [];
    this.notes = [];
  }
  ok(cond, label, extra = '') {
    this.asserts.push({ label, pass: !!cond, extra });
    console.log(`  ${cond ? 'PASS' : 'FAIL'} ${label}${extra ? ' — ' + extra : ''}`);
    return cond;
  }
  note(text) {
    this.notes.push(text);
    console.log(`  note: ${text}`);
  }
  get pass() {
    return this.asserts.every((a) => a.pass) && this.asserts.length > 0;
  }
  summary() {
    const failed = this.asserts.filter((a) => !a.pass);
    return {
      id: this.id,
      name: this.name,
      pass: this.pass,
      total: this.asserts.length,
      failed,
      notes: this.notes,
    };
  }
}

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
