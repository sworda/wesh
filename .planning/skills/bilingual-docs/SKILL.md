---
name: bilingual-docs
description: "wesh 项目文档双语产出契约：写完中文主版本后同步生成 .en.md 英文译本"
user-invocable: false
---

# 双语项目文档产出（中文主版本 + 英文 `.en.md` 译本）

本技能约束你写项目文档时的产出物。**中文是主版本**，英文译本与主版本并列存放。

## 适用范围

仅适用于以下 8 个规范文档路径（GSD 生成器的规范表路径）：

| 类型 | 主版本（中文） | 译本（英文） |
|---|---|---|
| readme | `README.md` | `README.en.md` |
| contributing | `CONTRIBUTING.md` | `CONTRIBUTING.en.md` |
| architecture | `docs/ARCHITECTURE.md` | `docs/ARCHITECTURE.en.md` |
| getting_started | `docs/GETTING-STARTED.md` | `docs/GETTING-STARTED.en.md` |
| development | `docs/DEVELOPMENT.md` | `docs/DEVELOPMENT.en.md` |
| testing | `docs/TESTING.md` | `docs/TESTING.en.md` |
| configuration | `docs/CONFIGURATION.md` | `docs/CONFIGURATION.en.md` |
| deployment | `docs/DEPLOYMENT.md` | `docs/DEPLOYMENT.en.md` |

其它文档（`CODEBUDDY.md`、`web/uat/pw/README.md` 等）不产译本。

## 产出流程

1. 按收到的 `doc_assignment` 写主版本 `X.md`（中文）。
2. **紧接着**写译本 `X.en.md`。两步都在本次调用内完成，不要另行委托。
3. 跑文末自查清单。

`mode: supplement` 时同理：给主版本补缺失章节之外，译本也要同步补。

## 语言切换行

两版都必须在 H1 之后插入**且只插一行**切换链接：

- 主版本 `X.md`：`**简体中文** | [English](X.en.md)`
- 译本 `X.en.md`：`**English** | [简体中文](X.md)`

格式：`**当前语言** | [另一语言](目标文件)`，竖线为半角 `|`，两侧各一个空格。

## 标记纪律（关键）

- 主版本 `X.md`：保留首行 `<!-- generated-by: gsd-doc-writer -->`。**`README.md` 是唯一例外**——它按设计不带标记，不要添加。
- 译本 `X.en.md`：**绝对不要**加 `<!-- generated-by: gsd-doc-writer -->`。该标记声明生成器所有权，译本不是生成器的规范产物，加上会造成虚假溯源。译本首行改用译本声明：

```
<!-- English translation of X.md. The Chinese X.md is the primary copy and is
     maintained by the documentation toolchain ("gsd-doc-writer"); keep this translation in
     sync whenever the primary copy changes. Not generated — do not add a gsd-doc-writer marker. -->
```

把 `X` 换成实际基名（`README.en.md`、`CONTRIBUTING.en.md`、`ARCHITECTURE.en.md` …）。

## 翻译完整性

译本必须与主版本**信息量对等**：每一个标题、段落、表格行、列表项、代码块都要有一一对应的英文。不得概括、省略、新增。

结构逐字保持：标题层级（`#`/`##`/`###`）数量与顺序一致；表格列数与行数一致；代码围栏数量一致且成对。

## 不翻译清单（逐字保留）

- 代码块内的命令、指令、配置键值、终端输出、URL
- flag 名（`--writable`）、环境变量（`WESH_CREDENTIAL`）
- 文件路径、函数名、类型名、包名、标识符、构建标签（`-tags=load`）
- 默认值、枚举字面量、退出码、端口号、版本号
- 专有名词：`shared`、`per-client`、`herdr`、`tmux`、`ttyd`、`xterm`、`jsdom`、`Playwright`

代码块内的**自然语言注释**（shell `#`、systemd/TOML/Dockerfile 注释、目录树说明行）应当翻译；命令与配置本身不得改动。

## mermaid 纪律

- 只翻译节点标签与边标签中的人类可读文字
- 节点 id、`subgraph`/`end` 结构、`-->` / `-.->` / `==>` 箭头、图形类型行（如 `graph TD`）一律不动
- 节点数与边数不变，图的行数应与主版本一致
- **边标签内绝对不得出现裸 `{`**（本项目历史上曾因此导致渲染失败）。带引号标签内的 URL 字面量如 `/s/{token}/` 属原文既有内容，保留

## 链接规则

- 主版本 `X.md`：内部文档链接指向中文主版本（`[CONFIGURATION.md](CONFIGURATION.md)`、`[README](../README.md)`）
- 译本 `X.en.md`：内部文档链接指向英文译本（`[CONFIGURATION.md](CONFIGURATION.en.md)`、`[README](../README.en.md)`）
- **唯一例外**是文首语言切换行，它跨语言指向对侧
- 外链（github.com / img.shields.io）与 `LICENSE` 一律不动

## `fix` 模式下的语言纪律

校验器发现某行失实时会以 `fix` 模式派回给你做外科修正。此时：

- **只改被点名的那一行**，不得重排、重写、重排版式（生成器对 fix 模式的硬约束）
- **保持该行原有语言**：修中文主版本的失实行用中文，修 `.en.md` 译本的失实行用英文。不要因为仓库语境是中文就把英文行改成中文
- 若主版本改了事实，译本同一句也须同步修正（否则两版漂移）
- 修正后复核：切换行唯一、标记纪律、链接方向三条仍成立

## 长度处理

单篇最长的是 `docs/DEPLOYMENT.md`（约 380 行 / 26 处代码围栏）。译本必须**整篇一次性写完**——宁可分两次 Write/Edit 续写，也不要为省事而删节或概括；返回前确认译本与主版本的代码围栏数、标题数、表格行数逐项相等。

## 随主版本联动的其它产出

- `.goreleaser.yml` 的 `archives.files` 须同时列出 `README.md` 与 `README.en.md`
- 两份 README 中描述发布包内容的那句话须同时提及 `README.md`（中文主版本）与 `README.en.md`（英文译本）

## 禁止事项

- **不要**在 `docs/` 下新建子目录。生成器判定「分组结构」的阈值是 `docs/` 下出现 **2+ 子目录**；一旦触发，规范中文文档会被迁移到 `docs/architecture/`、`docs/guides/` 等子目录，全部交叉链接失效。译本一律与主版本同目录、加 `.en.md` 后缀。
- **不要**使用 `.zh-CN.md` 后缀——中文已是主版本，无需后缀。
- **不要**删除主版本的 `<!-- generated-by -->` 标记，也不要给 `README.md` 添加它。
- **不要**把本技能、`agent_skills` 配置或生成器内部用语写进生成文档的正文。

## 自查清单

- [ ] 两份文件代码围栏数相等且为偶数
- [ ] 两份文件标题层级数量与顺序一致
- [ ] 两份文件各有且仅有一行语言切换链接，指向正确
- [ ] 译本首行是译本声明注释，全文无 `<!-- generated-by: gsd-doc-writer -->`
- [ ] 主版本保留了标记（`README.md` 除外）
- [ ] 译本内所有本地 `.md` 链接指向 `.en.md` 兄弟文件，仅切换行跨语言
- [ ] 不翻译清单里的内容逐字未变（尤其默认值、flag、退出码）
- [ ] mermaid 块节点 id 与边数与主版本一致，边标签无裸 `{`
- [ ] 未在 `docs/` 下新建子目录
