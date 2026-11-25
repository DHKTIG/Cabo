# CABO 项目总览与交接

> 更新日期：2026-11-25
> 本文是除 `README.md` 外的唯一项目文档，合并了原 `HANDOVER.md`、`docs/CABO-玩法详解.md`、`docs/开发准备清单.md`、`.trae/PROJECT_MEMORY.md` 和 `.trae/rules/git-commit-message.md`。
> `README.md` 面向玩家；本文面向开发、维护和继续迭代。

---

## 1. 当前状态

CABO 是一个用 Go 编写的单机版卡牌游戏：玩家对战 1–3 个电脑对手，双击 exe 后通过 WebView2 打开原生窗口，不需要浏览器，也不需要安装 Go 或 Node。

当前已实现：

- 完整对局流程与第二版规则口径；
- 三档启发式 AI；
- 单局 HTTP 服务与内嵌前端；
- 透明水玻璃、3D 牌桌装饰、气雾角色视觉；
- 发牌、翻牌、飞牌、洗牌、置换、CABO、结算等动效；
- Web Audio 实时合成配乐与音效，无音频素材文件；
- 音量设置、生涯统计、连击和回合回顾；
- Windows 图标资源、CI、规则/AI/HTTP/持久化测试。

2026-11-25 目录整理后执行过 `go test ./...`，全部通过。当前 Go 源码约 2889 行，前端约 4871 行，Go 侧有 24 个测试函数。

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
│       ├── window_windows.go         WebView2 窗口
│       ├── window_other.go           非 Windows 占位实现
│       ├── cabo.ico                  图标源文件
│       └── rsrc_windows_amd64.syso   已生成的 Windows 图标资源
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
│   │   └── ai_test.go                多种子完整对局与牌数守恒
│   └── server/
│       ├── server.go                 HTTP API + go:embed
│       ├── prefs.go                  音量持久化
│       ├── stats.go                  生涯统计持久化
│       ├── errors.go                 规则错误哨兵
│       ├── server_test.go
│       ├── stats_test.go
│       └── static/
│           ├── index.html
│           ├── style.css
│           ├── app.js
│           ├── audio.js
│           └── devshots.js          仅 ?dev=1 加载
├── docs/
│   ├── screenshot-start.png          旧版截图
│   ├── screenshot-play.png           旧版截图
│   └── icon-preview.png
└── artifacts/
    ├── app/
    │   ├── cabo.exe                  当前 GUI 成品
    │   ├── cabo-dev.exe               当前调试成品
    │   └── cabo_webview2_data/        WebView2 便携缓存
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

- 成品：[`artifacts/app/cabo.exe`](artifacts/app/cabo.exe)
- 调试版：[`artifacts/app/cabo-dev.exe`](artifacts/app/cabo-dev.exe)

| 参数 | 作用 |
|---|---|
| 无参数 | 启动 WebView2 原生窗口 |
| `-browser` | 改用系统浏览器 |
| `-port N` | 固定本地端口；默认自动分配 |
| `-debug` | 允许窗口内右键检查元素 |
| `-log` | 额外记录每个 HTTP 请求 |

基础文件日志始终开启；`-log` 只控制 HTTP 请求日志。日志按顺序尝试写入用户缓存目录、exe 同目录和系统临时目录，文件名固定为 `cabo.log`。

WebView2 缓存固定放在 exe 同目录的 `cabo_webview2_data/`，所以 exe 和缓存应作为整体移动。

### 2.2 环境要求

- Go 1.27，工具链 go1.27.3；
- Windows 10/11；
- 系统 WebView2 运行时；
- 不需要 cgo、gcc、Node 或前端包管理器。

主要依赖：

- `github.com/jchv/go-webview2`
- `golang.org/x/sys`

### 2.3 构建命令

本地整理后的推荐输出位置：

```bash
go test ./...
go build -ldflags="-s -w -H windowsgui" -o artifacts/app/cabo.exe ./cmd/cabo
go build -o artifacts/app/cabo-dev.exe ./cmd/cabo
```

`-H windowsgui` 必须加，否则会附带黑色控制台窗口。前端通过 `go:embed` 嵌入二进制，修改前端后必须重新构建。

README 和 CI 仍保留根目录 `cabo.exe` 的输出约定；CI 只是构建并上传 Artifact，本地手动构建时可以把产物移动到 `artifacts/app/`。

### 2.4 图标资源

图标资源已经提交，正常构建不需要额外步骤。只有修改 `cabo.ico` 后才需要重新生成。

必须使用 `go-winres`，不要使用 MinGW `windres` 或 `rsrc`：

```bash
go install github.com/tc-hib/go-winres@latest
cd cmd/cabo
$(go env GOPATH)/bin/go-winres.exe simply --icon cabo.ico --product-name "CABO" --file-description "CABO 卡波" --product-version 1.0.0.0 --file-version 1.0.0.0
rm -f rsrc_windows_386.syso
cd ../..
go build -ldflags="-s -w -H windowsgui" -o artifacts/app/cabo.exe ./cmd/cabo
```

历史踩坑：

- MinGW `windres` 生成的资源可能让 Go 链接器报 `unknown relocation type 7`；
- `rsrc` 生成的资源可能让 GUI 版在进入 `main()` 前崩溃，进程内存停在约 14MB；
- 正常启动 WebView2 后，进程内存通常在 30MB 以上，可作为粗略验证。

---

## 3. 核心架构

### 3.1 入口与运行链路

`cmd/cabo/main.go` 负责：

1. 解析参数；
2. 在 `127.0.0.1` 上启动本地 HTTP 服务；
3. 创建 `server.New()`；
4. Windows 下尝试打开 WebView2，失败时退回系统浏览器；
5. 捕获 panic 并写入日志、弹出错误提示。

服务地址只监听回环地址，不直接对外网暴露。

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

服务端成功响应后清空 `state.Events`，避免重复播放。`reshuffle.player` 固定为 `-1`，`value` 是洗回的牌数。

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

| 路径 | 作用 |
|---|---|
| `/api/new` | 开局 |
| `/api/state` | 获取当前视角 |
| `/api/action` | 玩家动作 |
| `/api/ai` | 推进一个 AI 动作 |
| `/api/next` | 下一轮 |
| `/api/prefs` | 音量设置 |
| `/api/stats` | 生涯统计 |

服务端只维护一局全局游戏。AI 不是后台自动循环，而是由前端逐步调用 `/api/ai`。

响应统一为：

```json
{ "ok": true, "ready": true, "view": {} }
```

规则错误通常返回 HTTP 200 + `ok:false` + 中文错误信息。`new/prefs/stats` 请求体限制为 4096 字节；`action` 目前没有同样的限长。

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
- CABO 使用座位旁金色气球字。

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

`busy`、`animating`、`aiRunning` 防止重入。

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
- CABO 完成粒子 + 气球字；
- 圆形扩散转场；
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

刷新后重新进入结算时，`roundEvents` 为空，必须跳过上报，否则会重复计数。顶栏当前只常驻显示最低分和最长连击，其余在 tooltip 中。

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

WebView2 每次随机端口，浏览器眼里的 origin 每次不同，localStorage 不能可靠跨启动保存。

真正持久值写入：

- `%LOCALAPPDATA%\Cabo\prefs.json`
- `%LOCALAPPDATA%\Cabo\stats.json`

前端仍用 localStorage 做首屏即时缓存，服务端返回后覆盖。写入前限制请求体并夹紧数值。

---

## 9. 持久化与安全边界

### 9.1 当前持久化

- 对局状态不落盘，关闭即丢失；
- 音量写 `prefs.json`；
- 生涯统计写 `stats.json`；
- 损坏文件回退默认值；
- 当前文件写入不是原子替换。

### 9.2 统计信任模型

服务端完全信任客户端上报的统计增量，没有根据服务端对局历史重新校验，也没有幂等去重。当前单机场景可接受，联机或排行榜必须先 redesign。

### 9.3 HTTP 边界

- 只监听 `127.0.0.1`；
- 单进程用全局互斥锁串行化；
- 无鉴权；
- 无 Origin/CSRF 检查；
- 多数 API 未限制 HTTP 方法；
- `action` 请求体未限长；
- 多标签页会共享同一局状态。

这是桌面单机工具，不是公开多用户服务。若要联机，必须增加房间、身份、并发仲裁和输入校验。

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

- `internal/engine/rules_test.go`：17 项规则测试；
- `internal/ai/ai_test.go`：60 个种子自动完整对局；
- `internal/server/server_test.go`：HTTP 完整对局、静态资源、prefs、未开局拒绝；
- `internal/server/stats_test.go`：统计合并、持久化、脏输入。

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
- HTTP 完整对局；
- prefs/stats 往返。

尚未专门覆盖：

- 100 分重置；
- 最终平局；
- CABO 后 Swap 锁；
- 独立 Spy 成功路径；
- HTTP 方法限制；
- 超大 action 请求；
- race detector；
- 覆盖率门槛。

### 11.3 规则改动后的最低验证

1. `go test ./internal/engine`；
2. `go test ./internal/ai`；
3. `go test ./internal/server`；
4. `go vet ./...`；
5. 修改前端后重新 build；
6. 验证 780×560 和 1280×860 布局。

---

## 12. CI、发布与仓库约定

### 12.1 CI

`.github/workflows/ci.yml` 在 Windows 上执行：

1. `gofmt -l .` 检查；
2. `go vet ./...`；
3. `go test ./...`；
4. `govulncheck`，只报告不阻断；
5. GUI 构建；
6. 上传 `cabo-windows-amd64` Artifact。

Dependabot 每月更新 Go 模块和 GitHub Actions。

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
- `cabo_webview2_data/`
- `*.exe`

因此 `artifacts/` 中的 exe、日志和 WebView 缓存不会进入 Git；分享 zip 当前仍是未跟踪文件。

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

### 13.4 WebView2 与截图

- WebView2 JS 注入历史上不可靠，页面是否加载应看请求日志；
- 无头 Edge 每次使用独立 `--user-data-dir`；
- 验版式加 `--force-prefers-reduced-motion`；
- 验动效不要加 reduced-motion；
- 页面不可见时 CSS 动画和 rAF 会停在中间帧；
- 无头截图可能看不到脚本执行后才显示的 fixed 元素，可用 `elementFromPoint` 判断层叠；
- 从某些沙箱工作目录启动 exe 可能报 `winapi error #10106`，可复制到临时目录验证；
- 关闭游戏后再复制日志，避免文件占用。

### 13.5 环境

- Python `urllib` 连续请求可能被重置，批量 HTTP 验证用长连接；
- 后台进程使用工具的 `run_in_background`，不要依赖 `cmd &`；
- Go 构建 exe 在 Git Bash 中注意 Windows 路径转换；
- 改完 GUI 程序后确认进程达到 30MB 级，粗略判断 WebView2 是否加载。

---

## 14. 已知限制

- 只维护一局游戏，多窗口会互相干扰；
- 没有对局存档；
- 只有人 vs AI；
- AI 是启发式，不做概率搜索；
- 只在 Windows 上验证；
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
