# CABO 项目总览与交接

> 更新日期：2026-11-25
> 本文是除 `README.md` 外的唯一项目文档，合并了原 `HANDOVER.md`、`docs/CABO-玩法详解.md`、`docs/开发准备清单.md`、`.trae/PROJECT_MEMORY.md` 和 `.trae/rules/git-commit-message.md`。
> `README.md` 面向玩家；本文面向开发、维护和继续迭代。

---

## 1. 当前状态

CABO 是一个用 Go 编写规则引擎与本地服务、用 Electron 承载桌面窗口的单机版卡牌游戏：玩家对战 1–3 个电脑对手，双击 exe 后打开独立应用窗口，不需要浏览器，也不需要安装 Go 或 Node。

当前已实现：

- 完整对局流程与第二版规则口径；
- 三档启发式 AI；
- 单局 HTTP 服务与内嵌前端；
- Electron 桌面窗口、单实例锁与子进程回收；
- Windows Portable EXE 打包；
- 透明水玻璃、3D 牌桌装饰、气雾角色视觉；
- 发牌、翻牌、飞牌、洗牌、置换、CABO、结算等动效；
- Web Audio 实时合成配乐与音效，无音频素材文件；
- 音量设置、生涯统计、连击和回合回顾；
- Windows 图标资源、CI、规则/AI/HTTP/持久化测试；
- Electron CDP 冒烟测试，验证真实窗口中的开局与回合流程。

2026-11-26 企业级重构后，Go 测试、vet、race、前端 lint/单测、覆盖率门禁、govulncheck 与 Electron 冒烟均通过。当前 Go 源码约 4701 行，前端约 4985 行，Go 侧有 70 个测试函数，前端有 4 个单测。

### 1.1 当前目录

```text
E:\Cabo\
├── README.md                         玩家说明
├── HANDOVER.md                       本文：唯一项目文档
├── LICENSE                           MIT
├── go.mod / go.sum
├── .github/                          CI、Issue 模板、Dependabot
├── cmd/
│   └── cabo/
│       ├── main.go                   入口、参数、本地服务
│       ├── error_windows.go          Windows 错误提示
│       ├── error_other.go            非 Windows 错误提示
│       ├── cabo.ico                  图标源文件
│       └── rsrc_windows_amd64.syso   已生成的 Windows 图标资源
├── desktop/
│   ├── main.cjs                      Electron 主进程
│   ├── package.json                  开发依赖、质量命令与便携打包配置
│   ├── package-lock.json             锁定 Electron 构建依赖
│   ├── tests/                        前端纯逻辑单测
│   └── scripts/
│       ├── dev.cjs                   构建 Go 服务并启动开发窗口
│       ├── build.cjs                 构建 Go 服务并生成便携 EXE
│       ├── lint.cjs                  从仓库根运行 ESLint
│       ├── coverage.cjs              race 与覆盖率门禁
│       └── smoke.cjs                 CDP 驱动真实 Electron 开局
├── internal/
│   ├── engine/                       纯规则引擎
│   │   ├── card.go                   牌、点数、能力、牌组
│   │   ├── state.go                  状态、发牌、推进、结算
│   │   ├── rules.go                  Apply：唯一规则入口
│   │   ├── event.go                  结构化事件
│   │   ├── view.go                   视角裁剪
│   │   └── rules_test.go
│   ├── ai/
│   │   ├── ai.go                     三档启发式 AI
│   │   └── ai_test.go                多种子完整对局、守恒与可复现性
│   ├── game/                         对局编排、连击/失误与结算幂等
│   │   ├── service.go
│   │   ├── progress.go
│   │   ├── service_test.go
│   │   └── progress_test.go
│   ├── storage/                      prefs/stats 版本迁移与原子持久化
│   │   ├── store.go
│   │   ├── prefs.go
│   │   ├── stats.go
│   │   └── storage_test.go
│   └── server/
│       ├── server.go                 HTTP 路由、鉴权与静态资源
│       ├── http.go                   方法、请求体、Origin/Host 与错误模型
│       ├── prefs.go                  设置 API 适配器
│       ├── stats.go                  权威统计只读 API
│       ├── server_test.go
│       ├── stats_test.go
│       ├── http_test.go
│       └── static/
│           ├── index.html
│           ├── style.css
│           ├── main.mjs               前端入口
│           ├── core.mjs / dom.mjs / api.mjs
│           ├── render.mjs / effects.mjs
│           ├── flow.mjs / stats.mjs
│           ├── audio.mjs
│           └── devshots.mjs           仅 ?dev=1 动态加载
├── docs/
│   ├── screenshot-start.png          旧版截图
│   ├── screenshot-play.png           旧版截图
│   └── icon-preview.png
└── artifacts/
    ├── app/
    │   └── cabo.exe                   当前 Electron 便携成品
    ├── logs/
    │   └── cabo.log                   历史运行日志
    ├── share/
    │   └── index.html.zip             当前五个前端文件的分享快照
    └── screenshots/
        └── .tmp/                      截图临时目录
```

`docs/` 中的两张界面截图仍是旧奶油色版本，不代表当前水玻璃美术。

---

## 2. 运行与构建

### 2.1 直接运行

- 成品：[`artifacts/app/cabo.exe`](artifacts/app/cabo.exe)，Windows Portable EXE。
- 双击后 Electron 自动选择回环端口，启动随包携带的 Go 服务，再创建 1280×860 的游戏窗口。
- 开发 Electron 窗口时，在 `desktop/` 执行 `npm run dev`；设置 `CABO_DEVTOOLS=1` 可打开开发者工具。
- 直接运行 Go 调试服务时，可使用 `-browser`、`-port N`、`-log` 等参数。

| 参数 | 作用 |
|---|---|
| 无参数（Electron 成品） | 启动独立桌面窗口 |
| `-browser` | Go 服务使用系统浏览器打开 |
| `-port N` | 固定本地端口；默认自动分配 |
| `-server` | 仅启动 HTTP 服务，供 Electron 调用 |
| `-log` | 额外记录每个 HTTP 请求 |

基础文件日志始终开启；`-log` 只控制 HTTP 请求日志。日志按顺序尝试写入用户缓存目录、exe 同目录和系统临时目录，文件名固定为 `cabo.log`。

### 2.2 环境要求

开发与构建：

- Go 1.27，工具链 go1.27.4；
- Node.js 24、npm；
- Windows 10/11。

运行成品：

- Windows 10/11；
- 不需要 Go、Node、系统浏览器或 WebView2；
- Electron 运行时已经打进 Portable EXE。

主要依赖：

- `electron`
- `electron-builder`
- `eslint` / `@eslint/js` / `globals`
- `golang.org/x/sys`

### 2.3 构建命令

在项目根目录执行：

```bash
go test ./...
cd desktop
npm ci
npm run lint
npm test
npm run coverage
npm run build
npm run test:smoke
CABO_EXE_PATH=../artifacts/app/cabo.exe npm run test:smoke
```

`npm run build` 会：

1. 把 Go 入口编译为 `desktop/.electron/cabo-server.exe`；
2. 用 electron-builder 生成 `artifacts/electron/cabo.exe`；
3. 复制为最终成品 `artifacts/app/cabo.exe`。

Go 服务仍使用 `-ldflags="-s -w -H windowsgui"` 构建。前端通过 `go:embed` 嵌入 Go 服务，修改前端后必须重新执行完整构建。国内网络下载中断时，可同时设置 `ELECTRON_MIRROR=https://npmmirror.com/mirrors/electron/` 和 `ELECTRON_BUILDER_BINARIES_MIRROR=https://npmmirror.com/mirrors/electron-builder-binaries/` 后重试；后者用于 electron-builder 的内置工具，不能用前者替代。

`npm run coverage` 运行 race detector，要求 64 位 GCC/clang；`--no-race` 仅用于诊断覆盖率，不能替代门禁。本机已安装官方 w64devkit 2.11.0 到 `D:\tools\w64devkit\w64devkit`，可用 `CC="D:/tools/w64devkit/w64devkit/bin/gcc.exe" PATH="/d/tools/w64devkit/w64devkit/bin:$PATH" npm run coverage`。门禁为 engine/game/storage ≥90%、ai/server ≥85%、总覆盖率 ≥80%。

### 2.4 图标资源

图标资源已经提交，正常构建不需要额外步骤。只有修改 `cabo.ico` 后才需要重新生成。

必须使用 `go-winres`，不要使用 MinGW `windres` 或 `rsrc`：

```bash
go install github.com/tc-hib/go-winres@latest
cd cmd/cabo
$(go env GOPATH)/bin/go-winres.exe simply --icon cabo.ico --product-name "CABO" --file-description "CABO 卡波" --product-version 1.0.0.0 --file-version 1.0.0.0
rm -f rsrc_windows_386.syso
cd ../..
cd desktop
npm run build
```

历史踩坑：

- MinGW `windres` 生成的资源可能让 Go 链接器报 `unknown relocation type 7`；
- `rsrc` 生成的资源可能让 GUI 版在进入 `main()` 前崩溃，进程内存停在约 14MB；
- 当前 Electron 成品启动后应同时看到 Electron 与 Go 服务进程，关闭窗口后 Go 服务应随之退出。

---

## 3. 核心架构

### 3.1 入口与运行链路

`desktop/main.cjs` 负责：

1. 申请单实例锁；
2. 选择一个空闲回环端口并生成随机 API 令牌；
3. 启动随包携带的 `cabo-server.exe -server -port N`，令牌只通过子进程环境变量传递；
4. 轮询 `/healthz`，服务就绪后创建 Electron `BrowserWindow`，页面通过 URL fragment 取得令牌并立即清除；
5. 窗口关闭或应用退出时请求 Go 子进程退出，等待其结束，5 秒后强制终止。

`cmd/cabo/main.go` 负责参数解析、日志、监听回环地址、信号关闭和挂载 `server.New()`。`-server` 模式不会打开浏览器或窗口。`internal/server` 只处理 HTTP/鉴权/输入边界；对局状态、AI 推进、连击与结算由 `internal/game` 持有，prefs/stats 由 `internal/storage` 持久化。

服务地址只监听 `127.0.0.1`，不直接对外网暴露。Electron 窗口启用 `contextIsolation`、`sandbox`，关闭 `nodeIntegration`，并拒绝外部导航和新窗口。

### 3.2 规则引擎是确定性的

`internal/engine` 不 import 业务外部包，不碰网络、文件、时钟或全局随机源。

核心入口：

```go
Apply(state State, player int, action Action) (State, error)
```

规则约束：

- `Apply` 先克隆状态，失败不会污染原状态；
- 随机数由 `NewGame` 注入的 `*rand.Rand` 提供；
- 发生器存放在不导出的 `State.rng` 中，不进入 JSON；
- `Clone` 是浅拷贝，随机流指针会继续沿用；
- 牌堆摸空后的洗牌发生在 `Apply` 内部；
- 同一种子 + 同一动作序列仍应得到同一牌序。

不要为了“更纯”把洗牌挪到引擎外，也不要在引擎里读时钟、文件或全局随机源。

### 3.3 隐藏信息必须裁剪

引擎内部知道所有牌，但对外只能使用：

```go
view := engine.ViewFor(state, playerID)
```

禁止直接序列化完整 `State`，否则会泄露所有暗牌和牌堆顺序。

`Slot.KnownBy []int` 记录哪些玩家知道这张牌：

- 开局看牌后记录知识；
- Peek/Spy 成功后补充知识；
- 明牌对所有玩家可见；
- Swap 时牌的位置和朝向一起移动，`KnownBy` 重新计算；
- 暗牌被换走后，旧玩家对原位置的记忆立即失效。

服务端固定以玩家 0 视角响应。

### 3.4 事件驱动前端动画

引擎在状态变化时产生结构化 `Event`，前端不解析中文日志。

事件类型：

- `roundStart`
- `peek`
- `draw`
- `reshuffle`
- `discard`
- `place`
- `take`
- `penalty`
- `spy`
- `swap`
- `cabo`
- `score`
- `reset`
- `gameOver`

mutation 响应携带事件后由 `internal/game` 清空 `state.Events`；`GET /api/state` 是纯快照，不消费事件。`reshuffle.player` 固定为 `-1`，`value` 是洗回的牌数。

### 3.5 先播动画，再切状态

AI 行动走：

```js
await playFlights(view.events);
setView(newView, { flown: true });
```

这样飞牌先发生在旧画面上，状态随后更新。玩家自己的操作走同步路径，保持跟手。

AI 普通回合思考约 820ms，已摸牌进入 `place` 阶段后约 1050ms。飞牌普通约 700ms，Swap/CABO 约 900ms。

### 3.6 AI

AI 是参数化启发式，不做搜索树、概率推演或心理战，也不偷看牌堆。

三档主要区别：

- 轻松：更保守，较少置换；
- 标准：敢在信息不足时冒险，会吃低牌、干扰领先者；
- 较真：更早喊 CABO，更积极换牌和针对领先者。

AI 只读取 `KnownByPlayer(p)` 为真的暗牌信息。换多判定以最佳牌组是否实际找到为准，不能用“存在同点牌”代替完整校验。

### 3.7 HTTP 服务

路由：

| 路径 | 方法 | 作用 |
|---|---|---|
| `/healthz` | GET | 无鉴权健康检查，供 Electron 轮询 |
| `/api/new` | POST | 开局 |
| `/api/state` | GET | 获取当前视角纯快照 |
| `/api/action` | POST | 玩家动作 |
| `/api/ai` | POST | 推进一个 AI 动作 |
| `/api/next` | POST | 下一轮 |
| `/api/prefs` | GET/POST | 读取/保存音量设置 |
| `/api/stats` | GET | 读取服务端权威生涯统计 |

服务端只维护一局全局游戏。AI 不是后台自动循环，而是由前端逐步调用 `/api/ai`。统计不再接受客户端增量，轮次结算由 `internal/game` 计算并通过 `gameID:roundID` 台账幂等写入。

成功响应示例：

```json
{ "ok": true, "ready": true, "view": {}, "requestId": "..." }
```

错误响应使用 `error:{code,message,retryable}` 和 `requestId`，并按语义返回 400/401/403/405/409/413/415/500。API 校验 Host、Origin 与 Bearer 令牌；普通 JSON 请求限制 4 KiB，`action` 限制 16 KiB，并拒绝未知字段与尾随 JSON。

---

## 4. 当前游戏规则

### 4.1 牌组

牌没有花色，只有 `ID + Rank`：

| 数字 | 张数 | 能力 |
|---|---:|---|
| 0 | 3 | 无 |
| 1–6 | 各 4 | 无 |
| 7–8 | 各 4 | 看自己一张 |
| 9–10 | 各 4 | 看对手一张 |
| 11–12 | 各 4 | 交换任意两张 |
| 13 | 3 | 无 |

共 54 张，总点数 351，平均 6.5。改牌组时应维持该分布，否则平衡性会变化。

### 4.2 初始设置

- 2–4 人；
- 每人 4 张暗牌；
- 开局自选看 2 张；
- 看完盖回，位置不可调整；
- 之后依靠记忆和技能获取信息。

### 4.3 回合三选一

#### 从牌堆摸牌

摸到后可以：

1. 直接弃掉；
2. 换入自己一张牌，旧牌弃掉；
3. 如果是能力牌，发动能力，能力牌随后弃掉。

只有从牌堆摸到的牌能触发能力。

#### 拿弃牌堆顶

弃牌堆顶是明牌，可以：

- 1 换 1；
- 1 换 2；
- 1 换 3。

换出的牌必须彼此同点，不要求与拿进的牌同点。成功后，拿进的牌保持明牌。

如果换出的牌实际点数不一致：

- 收下弃牌堆顶；
- 参与换出的牌全部亮明；
- 涉及 3 张时，额外从牌堆罚抽 1 张；
- 罚抽前如果牌堆为空，会先洗弃牌堆，不能因此免罚。

#### 喊 CABO

喊 CABO 后：

- 喊者手牌锁定，不能被 Swap；
- 其他每位玩家各再行动一次；
- 此期间不能再次喊 CABO；
- 队列走完后摊牌结算。

### 4.4 结算

| 情况 | 本轮得分 |
|---|---:|
| 喊者且手牌并列最低 | 0 |
| 喊者但不是最低 | 手牌点数 +10 |
| 其他玩家 | 手牌点数 |

轮内平局时喊者得分最低，可视为喊者优先。

累计分数超过 100 时，打满整轮后游戏结束，累计最低分获胜。最终平局取累计最低分中下标较小的玩家，没有额外的喊者优先。

总分恰好 100 且该玩家尚未使用过重置时，代码会自动重置为 50，每场限一次，并产生独立 `reset` 事件。

### 4.5 牌堆耗尽

牌堆摸空时：

- 把弃牌堆洗回牌堆继续；
- 尽量在弃牌堆留一张明牌；
- 弃牌堆只剩一张时，整叠洗走；
- 牌堆和弃牌堆都空时才结算本轮。

实际对局中两堆同时耗尽很少见，主要用于保险。

### 4.6 固定规则决策

| 规则点 | 当前实现 |
|---|---|
| 开局看牌 | 自选 2 张，位置不可变 |
| 能力映射 | 7/8 看自己，9/10 看对手，11/12 交换 |
| 弃牌堆拿牌 | 保持明牌 |
| 1 换多判定 | 换出的牌彼此同点 |
| 1 换多上限 | 3 张 |
| 翻错惩罚 | 收下顶牌；涉及 3 张再罚抽 1 张 |
| 弃牌堆能力 | 不触发 |
| CABO 后行动 | 其他人各一次 |
| CABO 后手牌 | 喊者锁定，不可被 Swap |
| 轮内平局 | 喊者得分优先 |
| 最终平局 | 最低分中下标较小者 |
| 牌堆摸空 | 洗弃牌堆续上，尽量留一张明牌 |
| 两堆都空 | 结束本轮 |
| 100 分重置 | 恰好 100，自动重置为 50，每场一次 |
| 超过 100 分 | 打满整轮再结束 |

未实现：

- Kamikaze / QQKK 彩蛋；
- Burn 抢牌；
- 热座模式；
- 联机房间。

---

## 5. 规则调研与策略资料

### 5.1 CABO 的核心

CABO 是记忆 + 轻策略 + 心理博弈桌游。每人 4 张暗牌，开局只看 2 张，目标是把点数压低后喊 CABO。

乐趣来自信息不断变化：牌会被换走、被对手看过、被置换，玩家必须维护自己的记忆模型。

### 5.2 第一版与第二版差异

| 规则点 | 第一版 | 第二版 / 当前实现 |
|---|---|---|
| 人数 | 2–5 | 2–4 |
| 弃牌堆拿牌 | 之后可暗置 | 保持明牌 |
| 翻错惩罚 | 无额外惩罚 | 收牌，3 张再罚抽 |
| 漏喊 CABO | -5 | -10 |
| 计分 | 最低者固定 0 | 只有喊者最低得 0 |
| 一轮结束 | CABO | CABO 或两堆耗尽 |
| 100 分重置 | 无限次 | 每场一次 |

### 5.3 未实现变体

#### Kamikaze / 神风特攻队

喊 CABO 时四张牌正好为两张 12 + 两张 13：

- 喊者本轮 0 分；
- 其他玩家各 +50。

当前未实现。

#### Burn

线上版在回合结束后加入短时反应窗口，可烧自己的牌或对手的牌，并伴随成功/失败奖惩。当前未实现；未来需要并发仲裁、幂等和超时机制。

### 5.4 策略提示

- 记牌用“位置 + 点数 + 相邻关系”；
- 优先处理 12/13；
- 弃牌堆顶是 0–3 且自己有已知高牌时，1 换多通常稳定获利；
- Peek 优先看完全未知的牌；
- Spy 优先看刚被置换或领先者；
- 手牌总和较低且确定至少一家不比自己低时再喊 CABO；
- 故意留低牌可以诱导对手冒险换多；
- 对手弃出的牌会泄漏其手牌信息。

---

## 6. 当前前端设计

### 6.1 当前视觉

2026-11-23 的最新美术不是旧奶油色方案，而是：

- 中性灰白背景；
- 半透明水玻璃面板；
- 高光顶边、亮内环、底部厚度和模糊；
- `rotateX(24deg)` 的装饰性 3D 牌桌；
- 卡牌本身不参与牌桌的 3D 变换；
- 手牌透视扇形展开；
- SVG 气雾角色；
- 淡染卡面、左下技能色点、右上技能标签；
- CABO 使用纯 CSS 手写艺术字水玻璃 callout，悬浮在玩家信息栏上方。

核心视觉令牌见 `internal/server/static/style.css` 的 `:root`。

### 6.2 历史视觉方案

早期方案使用奶油底 `#FBF6EC`、珊瑚橘 `#FF7A59`、白色卡片和左侧 6px 技能色条。该方案已被整体替换。

保留历史记录是为了避免误把旧截图、旧 Trae 记忆或旧交接描述当成当前设计要求。后续视觉调整应基于当前灰白水玻璃语言，不要无确认地回退旧配色。

### 6.3 界面语言

- 品牌名使用英文 `CABO`；
- 其余功能文案使用中文；
- 页面 `<title>` 为 `CABO 卡波`；
- 按钮“喊 CABO”保留英文品牌词。

### 6.4 页面结构

`index.html` 包含：

- 顶栏：轮次、当前行动者、生涯、声音、重开；
- AI 座位区；
- 中央牌堆、待处理牌、弃牌堆；
- 自己状态条和手牌；
- 动态操作区；
- 特效层；
- 开场设置层；
- 结算层。

前端没有独立 FSM 库。服务端 `view.phase` 表示规则阶段，前端 `mode` 表示交互模式：

- `idle`
- `peek`
- `claim`
- `place`
- `peekSelf`
- `spy`
- `swap`

`busy`、`animating`、`aiRunning` 防止重入。前端使用原生 ES Module：`core/dom/api` 提供底层能力，`render/effects` 负责视图与动画，`flow` 编排交互和 AI，`main` 只接线；`audio.mjs` 提供幂等 `dispose()`，`devshots.mjs` 仅在 `?dev=1` 动态加载。

### 6.5 动效语言

统一使用 `:root` 中的缓动变量：

- `--e-out`
- `--e-soft`
- `--e-spring`
- `--e-swing`
- `--e-in`

卡牌类动画需要过冲回落，表现重量感。所有动画受 `prefers-reduced-motion` 控制。

已落地的主要动效：

- 发牌错峰落入；
- 3D 翻牌；
- 飞牌弧线；
- 弃牌落堆；
- 牌堆数量弹跳；
- 洗牌回堆；
- 技能脉冲；
- Swap 对向飞牌；
- CABO 完成粒子 + 水玻璃 callout；
- 圆形扩散转场（独立圆形层 `transform: scale()` 合成，不使用全屏 `clip-path`）；
- 一个动作三处联动；
- 弃牌堆横向摊开；
- 音量滑块吸附回弹；
- 结算回顾卡和分数滚动。

### 6.6 动效实现约束

- 同一元素不要同时挂两个会互相覆盖的 `animation`；
- 选中环画在 `.card::after`，不能画在暗牌不存在的 `.front` 上；
- 作者写过 `display` 的元素使用 `hidden` 时要补 `[hidden]{display:none}`；
- 退场先播动画，再挂 `.hidden`，使用 `hideOverlay()`；
- 飞牌元素用 `setTimeout(cleanup, dur+320)` 兜底，不能只依赖 `onfinish`；
- 飞牌弧度按距离计算：`lift = clamp(22 + dist*0.17, 26, 98)`；
- `.turn-chip.swap` 只在真的换人时添加，避免 render 重写 class 掐断动画；
- `element.animate().finished` 在页面不可见时可能永不 resolve，所有等待动画的流程都要有超时兜底；
- `circleWipe` 使用 1.5 秒超时和 `swapped` 标志，避免最小化后开始游戏卡死；
- 音量弹层挂在 `document.body`，避免被顶栏层叠上下文或滚动面板裁切；
- 动态数值必须先写入正确终值，动画只作为增强。

### 6.7 响应式

- 卡牌尺寸使用 `clamp()` 跟随视口高度；
- 矮窗、宽屏、窄屏有专门规则；
- 牌桌允许纵向滚动兜底；
- 原生窗口最小尺寸为 780×560；
- 改版式后应至少验证 780×560、780×500、1280×860。

当前可访问性已有中文 label、语义按钮、装饰 SVG `aria-hidden`、音频弹层 ARIA 和 reduced-motion。动态卡牌仍是 `div + click`，键盘聚焦、弹层焦点陷阱和动态提示 live region 尚未完善。

---

## 7. 累积式反馈

### 7.1 连击

只奖励玩家自己验证过的降分：

- `place` 到明牌位且新牌更小：+1；
- `takeDiscard` 换出的牌全部严格更大：+1；
- 罚牌：归零；
- 换入更大：归零；
- 安置到暗牌位：中性。

连击在 2/3/5/7 时升级颜色和音高。判定必须使用动作前 view，动作后状态已经切换。

### 7.2 回合回顾

`roundEvents` 只在客户端保存。服务端事件投递一次后清空，因此本轮历史只存在于前端。

每轮最多选 4 条：

- 生涯新低；
- CABO 赌赢/赌输；
- 一手换多；
- 谁用置换动了你；
- 记错次数；
- 零失误；
- 中途洗牌；
- 100 分重置。

回顾只能使用玩家看得见的信息，不能把暗牌面额写进回顾卡。

### 7.3 生涯统计

字段：

- `bestRound`：单轮最低点数；
- `bestStreak`：最长连击；
- `rounds`：完成轮数；
- `perfect`：零失误轮数；
- `cabos` / `cabosWon`：喊 CABO 次数与胜率。

“没有记录”使用 `-1`，因为 0 点是有效的最好成绩。

统计由 `internal/game` 在轮次结束时裁决并幂等写入，前端只读取权威结果；`roundEvents` 仅用于客户端回顾卡，刷新后为空不会影响统计。顶栏当前只常驻显示最低分和最长连击，其余在 tooltip 中。

---

## 8. 音频

所有音频由 Web Audio 实时合成，没有音频素材文件。这样保持单 exe、离线可用和体积可控。

### 8.1 总线

当前结构：

- `sfxBus -> master`
- `musicBus -> ducker -> master`
- `master -> limiter -> destination`
- `revIn -> ConvolverNode -> revOut -> master`

混响冲激响应在运行时生成：白噪声、幂衰减、单极低通、15ms 预延迟，约 2.1 秒立体声。

### 8.2 音效与配乐

- 29 种音效；
- D 大调、3/4 拍圆舞曲；
- 16 小节循环；
- 旋律按和弦与动机变奏，不是固定粘贴循环；
- `menu`：76 BPM，稀疏音乐盒；
- `table`：100 BPM，完整对局编制；
- `tension`：112 BPM，有人喊 CABO 或分数紧张时；
- 模式切换时 BPM 平滑滑动。

噪声类音效需要 makeup gain，否则带通滤波后可能比振荡器类安静约 10dB。AI 已飞过的动作使用 `flown` 标记，避免运动音效重复。

### 8.3 音量持久化

Electron 每次启动都会选择新的本地端口，浏览器眼里的 origin 也会变化，localStorage 不能可靠跨启动保存。

真正持久值写入：

- `%LOCALAPPDATA%\Cabo\prefs.json`
- `%LOCALAPPDATA%\Cabo\stats.json`

前端仍用 localStorage 做首屏即时缓存，服务端返回后覆盖。写入前限制请求体并夹紧数值；`internal/storage` 以 v2 文档持久化，兼容旧平铺 JSON，损坏文件自动隔离备份，写入采用同目录临时文件、`fsync`、原子替换。

---

## 9. 持久化与安全边界

### 9.1 当前持久化

- 对局状态不落盘，关闭即丢失；
- 音量写 `prefs.json`，生涯统计写 `stats.json`；
- 存储层使用进程内互斥锁、版本迁移和原子替换；
- 损坏文件改名为 `*.corrupt-<timestamp>` 后回退默认值；
- 统计文档同时保存结算台账，避免重复结算。

### 9.2 统计信任模型

服务端根据对局状态计算本轮点数、最长连击、失误、CABO 胜负，并通过 `gameID:roundID` 结算台账幂等写入；`/api/stats` 只接受 GET，客户端不能注入增量。存储失败时结算保留在内存待重试，重开新局前会先尝试落盘。

### 9.3 HTTP 边界

- 只监听 `127.0.0.1`，校验 Host 与 Origin；
- Electron 为每次启动生成 Bearer 令牌，Go 子进程通过环境变量接收；
- API 严格限制方法、Content-Type、请求体大小和 JSON 字段；
- 规则错误返回 400/409，鉴权与边界错误返回 401/403/405/413/415；
- Electron 窗口启用隔离与沙箱，拒绝外部导航和新窗口；
- Go 支持信号优雅关闭，Electron 等待子进程退出并设置 5 秒强制终止兜底；
- 多标签页仍共享同一局状态，这是桌面单机工具的已知限制。

这是桌面单机工具，不是公开多用户服务。若要联机，必须增加房间、身份、并发仲裁和输入校验。

### 9.4 npm 构建依赖审计

`electron-builder@26.18.0` 的上游依赖仍引用受 `GHSA-g3hv-75wg-m249` 影响的 `minimatch`，当前 `npm audit` 报告 6 个 moderate。问题位于开发期构建工具链，不会打包进成品运行时；CI 以 `npm audit --audit-level=high` 阻断高危及以上漏洞。

不要直接运行 `npm audit fix --force`：它会降级到 `electron-builder@25.1.0`。升级 electron-builder 后应重新审计；在官方依赖提供修复前，构建只处理仓库内受信任文件。

---

## 10. 初始开发决策

以下 17 项来自立项时的规则规格化，当前大部分已落地：

| # | 歧义点 | 当前选择 |
|---:|---|---|
| 1 | 开局看牌 | 自选 2 张 |
| 2 | 是否调整顺序 | 不调整 |
| 3 | 能力映射 | 7/8 自看，9/10 看对手，11/12 交换 |
| 4 | 弃牌堆拿牌 | 保持明牌 |
| 5 | 1 换多基准 | 换出牌彼此同点 |
| 6 | 1 换多上限 | 3 张 |
| 7 | 翻错惩罚 | 收牌；3 张再罚抽 |
| 8 | 弃牌堆能力 | 不触发 |
| 9 | CABO 后对手 | 各行动一次 |
| 10 | CABO 后自己 | 锁定 |
| 11 | 轮内平局 | 喊者优先 |
| 12 | 牌堆摸空 | 洗回续上，尽量留明牌 |
| 13 | 100 分重置 | 恰好 100，每场一次 |
| 14 | 超过 100 | 打满整轮 |
| 15 | 2 人特殊规则 | 暂不实现 |
| 16 | Kamikaze | 默认不启用 |
| 17 | Burn | 第二阶段 |

### 10.1 信息可见性模型

| 层级 | 内容 | 可见范围 |
|---|---|---|
| 公开 | 弃牌堆、牌堆数量、手牌数量、分数、当前回合 | 所有玩家 |
| 私有 | 自己已知的牌、 pending 牌 | 仅自己 |
| 隐藏 | 对手暗牌、牌堆顺序 | 仅引擎 |

采用“半辅助记忆”：已知牌显示，但牌被换走后知识立即失效。不要改成永久记住所有见过的牌，否则会改变游戏性质。

### 10.2 核心数据模型

```go
Card   { ID, Rank }
Slot   { Card, FaceUp, KnownBy }
State  { Players, Deck, Discard, Pending, Phase, Turn, Caller, CaboQueue, Events, rng }
Action { Indices, Targets }
Event  { Kind, Player, Card, Target, Target2, Value }
```

`Card.ID` 用于追踪牌去向和守恒测试。当前采用状态快照，没有完整事件溯源或回放系统。

### 10.3 状态转移概览

```text
peek
  -> 每人看完两张
  -> turn

turn
  -> draw deck       -> place
  -> take discard    -> turn
  -> call CABO       -> CaboQueue

place
  -> discard pending -> turn
  -> swap in one     -> turn
  -> use power       -> turn

CaboQueue
  -> 其他玩家各行动一次
  -> roundEnd / gameOver
```

### 10.4 后续扩展的技术难点

- Burn：并发抢牌、幂等、超时、仲裁；
- 联机：房间模型、身份 token、断线重连；
- 回放：seed + 动作序列，或完整事件流；
- 存档：状态快照与已知牌信息一起保存；
- 计时器：必须正确停止，避免泄漏；
- 随机性：继续使用注入随机源。

---

## 11. 测试

### 11.1 当前测试

- `internal/engine/rules_test.go`：规则、视角裁剪、100 分重置、最终平局、Spy、CABO 锁牌与换牌知识失效；
- `internal/ai/ai_test.go`：60 个种子完整对局、牌数守恒和同种子可复现性；
- `internal/game`：动作编排、AI 推进、连击/失误、结算幂等与存储失败重试；
- `internal/storage`：旧版迁移、损坏隔离、原子写、并发事务和文件错误路径；
- `internal/server`：HTTP 完整对局、静态资源、鉴权、Host/Origin、方法、Content-Type、请求体与错误 envelope；
- `desktop/tests`：回顾卡信息边界、连击档位、令牌与错误归一化单测；
- `desktop/scripts/smoke.cjs`：CDP 驱动真实 Electron 完成开局、看牌、进入回合、默认/紧凑视口、按钮样式和进程/临时目录清理；`CABO_EXE_PATH` 直接验证 Portable EXE。

AI 测试每一步断言：

```text
牌堆 + 弃牌堆 + 所有手牌 + pending = 54
```

这条断言曾抓出换出牌没有进入弃牌堆的真实 bug。

### 11.2 已验证与未覆盖

已验证：

- 基本发牌、摸牌、换牌、技能；
- 1 换多与翻错惩罚；
- CABO 计分；
- 牌堆耗尽洗牌；
- 多局牌数守恒；
- HTTP 完整对局与服务端权威统计；
- prefs/stats 往返、损坏恢复和并发写；
- 100 分重置、最终平局、CABO 后 Swap 锁、独立 Spy；
- HTTP 方法、鉴权、Origin/Host 与请求体边界；
- 前端模块语法、lint、纯逻辑单测和真实 Electron 冒烟；
- race 与覆盖率门禁（CI 使用 64 位编译器）。

尚未专门覆盖：

- 多标签页并发操作；
- 真实断电、磁盘满与 Windows 文件锁故障注入；
- 音频离线 PCM 指纹；
- 代码签名与 SmartScreen 发布流程。

### 11.3 规则改动后的最低验证

1. `go test ./internal/engine ./internal/ai ./internal/game ./internal/storage ./internal/server`；
2. `go vet ./...`；
3. `cd desktop && npm run lint && npm test`；
4. `cd desktop && npm run coverage`；
5. 修改 Go 入口或前端后执行 `cd desktop && npm run build`；
6. 执行 `npm run test:smoke`，并验证最终 Portable EXE；
7. 验证 780×560 和 1280×860 布局。

---

## 12. CI、发布与仓库约定

### 12.1 CI

`.github/workflows/ci.yml` 在 Windows 上执行：

1. `gofmt -l .` 检查；
2. `go vet ./...` 与 `go test ./...`；
3. `govulncheck` 阻断；
4. 安装 Node 24 与 `desktop/package-lock.json` 锁定的 npm 依赖；
5. 前端 ESLint 与 Node 单测；
6. race detector 与分包/总覆盖率门禁；
7. `npm audit --audit-level=high` 阻断高危及以上 npm 漏洞；
8. 构建 Electron Portable EXE；
9. 分别运行开发态与 `CABO_EXE_PATH` 成品冒烟；
10. 上传 `artifacts/app/cabo.exe` 为 `cabo-windows-amd64` Artifact。

Dependabot 每月更新 Go 模块、`/desktop` npm 依赖和 GitHub Actions。

### 12.2 发布

- exe 不纳入 Git；
- 分发走 GitHub Releases 或 CI Artifact；
- 仓库为 public，使用 MIT；
- README 截图应在改版后更新；
- `.gitattributes` 统一 LF，并标记二进制资源。

### 12.3 忽略规则

当前忽略：

- `.workbuddy-ai/`
- `.trae/`
- `cabo.log`
- `cabo_webview2_data/`（历史缓存）
- `node_modules/`
- `desktop/node_modules/`
- `desktop/.electron/`
- `desktop/dist/`
- `artifacts/electron/`
- `*.exe`

因此 `artifacts/` 中的 exe、日志、WebView 历史缓存和 Electron 构建中间目录不会进入 Git；分享 zip 当前仍是未跟踪文件。

### 12.4 提交信息

仓库约定：

```text
<type>(<scope>): <中文摘要>
```

type 使用：

- `feat`
- `fix`
- `refactor`
- `chore`
- `docs`

body 用中文分条说明改动原因。原 Trae 提交规则文件只是空模板，没有额外规则，其约定已合并到此处。

---

## 13. 高频踩坑

### 13.1 引擎与 AI

- 不要把完整 State 发给前端；
- 不要在 `Apply` 中使用时钟、文件或全局随机源；
- Clone 的 rng 是浅拷贝指针，不要意外替换成新随机流；
- 洗牌必须发生在引擎内部，保证动作序列可重放；
- 换多判定要找实际最佳牌组，不能只判断存在同点；
- AI 不能读取未授权的暗牌；
- 改引擎后必须跑 AI 守恒测试。

### 13.2 前端与动效

- 同一元素的两个 `animation` 会整体覆盖；
- `.hidden` 是 `display:none`，退场动画要先播放；
- `onfinish` 不一定触发，飞牌要定时清理；
- 页面不可见时 WAAPI 和 rAF 可能停住，等待动画要超时；
- 先写正确 DOM 值，再播放动画；
- 弹层注意层叠上下文、overflow 和 z-index；
- 通用 class 名如 `.me`、`.primary` 容易冲突，要加作用域；
- hover 样式用 CSS 变量分层，避免覆盖按钮变体；
- 改前端必须重新 build，go:embed 不会自动读磁盘。

### 13.3 音频

- 音频初始化失败不能影响牌局；
- 噪声类音效注意 makeup gain；
- AI 飞牌后的动作不要重复发运动音效；
- 音量必须走服务端持久化，不能只靠 localStorage；
- 不要加入音频素材文件，保持单 exe 离线可玩。

### 13.4 Electron、浏览器与截图

- Electron 的 CDP 冒烟测试使用 Node 24 内置 `WebSocket`，不需要额外测试依赖；
- 在受限工作目录启动 Electron 可能静默退出，可把 `node_modules/electron/dist` 复制到系统临时目录后运行；
- 开发态冒烟测试使用 `--disable-gpu`，避开工具环境的 GPU 启动问题；Portable EXE 验证使用默认启动参数；
- 无头 Edge 每次使用独立 `--user-data-dir`；
- 验版式加 `--force-prefers-reduced-motion`；
- 验动效不要加 reduced-motion；
- 页面不可见时 CSS 动画和 rAF 会停在中间帧；
- 无头截图可能看不到脚本执行后才显示的 fixed 元素，可用 `elementFromPoint` 判断层叠；
- 关闭游戏后再复制日志，避免文件占用。

### 13.5 环境

- Python `urllib` 连续请求可能被重置，批量 HTTP 验证用长连接；
- 后台进程使用工具的 `run_in_background`，不要依赖 `cmd &`；
- Go 构建 exe 在 Git Bash 中注意 Windows 路径转换；
- Electron 成品包含 Chromium 运行时，体积和内存明显高于旧 WebView2 版；
- 修改 Go 入口或前端后，必须重新执行 `npm run build`，不能只跑 `go build`。

---

## 14. 已知限制

- 只维护一局游戏，多窗口会互相干扰；
- 没有对局存档；
- 只有人 vs AI；
- AI 是启发式，不做概率搜索；
- 只在 Windows 上验证；
- Electron 成品包含 Chromium 运行时，体积和内存高于旧 WebView2 版；
- 没有代码签名，首次下载可能触发 SmartScreen；
- 动态卡牌键盘可访问性不完整；
- 文档截图落后于当前美术；
- 分享 zip 是前端快照，不是独立运行包。

SmartScreen 不是病毒警告。处理方式：

1. 单次点击“仍要运行”；
2. 降低当前用户 SmartScreen 检查；
3. 购买 OV/EV 代码签名证书。

Defender 排除项对 SmartScreen 无效，自签名证书也不能建立公开信任。

---

## 15. 后续方向

| 优先级 | 事项 | 说明 |
|---|---|---|
| 中 | Burn | 需要并发仲裁、幂等、超时 |
| 中 | Kamikaze | 可在结算阶段增加彩蛋 |
| 中 | 联机 | 必须重构成房间模型 |
| 低 | 热座 | 做过又被撤掉，重做前先问清原因 |
| 低 | 对局存档 | 保存 State 与玩家已知信息 |
| 低 | 可访问性 | 键盘选牌、焦点陷阱、live region |
| 低 | 测试补强 | 100 分重置、最终平局、Swap 锁、HTTP 输入边界 |
| 低 | 文档截图 | 用当前水玻璃美术重截 |

如果继续开发：

1. 先跑 `go test ./...` 建立绿色基线；
2. 明确只改目标项，不顺手改配色或 surrounding code；
3. 改规则先看 `internal/engine/rules.go`；
4. 改界面先看 `internal/server/static/`；
5. 改完重新构建并验证布局；
6. 保持当前浅色水玻璃视觉，除非用户明确要求换肤。

---

## 16. 历史 Trae 记忆摘要

原 `.trae/PROJECT_MEMORY.md` 来自 `.workbuddy-ai/memory` 的 2026-10-29 至 11-02 每日记录浓缩。它保留了以下高价值约束，现已并入本文：

- 引擎随机流、视角裁剪和事件机制；
- AI 先飞牌再切状态；
- 旧奶油色视觉及卡面色条实现；
- 动效覆盖、hidden、飞牌清理和 WAAPI 超时；
- Web Audio 零素材方案；
- localStorage 因随机端口不可靠；
- 统计空值使用 -1；
- 截图、无头浏览器和 WebView 调试经验；
- 热座曾实现又被撤销；
- Burn、Kamikaze、联机为后续方向。

其中视觉部分属于历史方案，当前实现以第 6 节为准。

---

## 17. 规则资料来源

原规则调研整理于 2026-10-29，参考：

- Cabo 官方规则页：<https://cabocards.com/how-to-play/>
- Cabo 官网：<https://cabocards.com/>
- 豆瓣游戏条目：<https://www.douban.com/game/36665750/>
- 豆瓣规则整理：<https://www.douban.com/note/862503479/>
- 什么值得买规则整理：<https://post.m.smzdm.com/zzz/p/a34qpwkn>
- 百度相关规则整理：<https://mbd.baidu.com/newspage/data/dtlandingsuper?nid=1749618466625398667>

CABO 不同版本存在规则冲突。本项目以第 4 节的当前实现为准，调研资料用于理解背景和后续变体，不覆盖代码事实。
