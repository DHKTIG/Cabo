# CABO 项目交接文档

> 最后更新：2026-10-29 23:47
> 给接手的人（或下一个模型）看的。读完这份就能直接继续干活，不需要回溯对话。

---

## 一、现在是什么状态

一个用 Go 写的单机版 CABO 卡牌游戏，**已经能跑、能玩完整一局**。

- 产物：`E:\Cabo\cabo.exe`（7.3 MB），**双击直接弹出一个原生窗口**，不经过浏览器。
- 窗口里是「你 vs 1–3 个电脑对手」，打到有人超过 100 分结束。
- 规则、AI、界面、动画全部完成；测试全绿（`go test ./...`）。
- 原生窗口基于 WebView2（系统自带运行时），前端仍然是 HTML/CSS/JS，编译时嵌入二进制。

**一句话：功能完整（含配乐与音效），视觉打磨过两轮，现在处于「可以交付给朋友玩」的阶段。**

---

## 二、怎么跑

### 用户视角

双击 `cabo.exe`，等 1–2 秒弹出窗口，直接开始。

| 参数 | 作用 |
|---|---|
| （无） | 原生窗口 |
| `-browser` | 改用系统浏览器打开（窗口起不来时的退路） |
| `-port 8848` | 固定端口，默认随机分配 |
| `-debug` | 开启调试：窗口内可右键「检查元素」，并输出渲染自检日志 |
| `-log` | 把请求日志写到 `%LOCALAPPDATA%\Cabo\cabo.log` |

窗口打不开会弹系统提示框，并自动退回浏览器模式。

### 开发视角

```bash
cd E:/Cabo

go test ./...                                              # 全部测试
go build -ldflags="-s -w -H windowsgui" -o cabo.exe ./cmd/cabo   # 出成品
go build -o cabo-dev.exe ./cmd/cabo                        # 出带控制台的调试版
```

- **`-H windowsgui` 必须加**，否则会附带一个黑色控制台窗口。
- 图标和版本信息由 `cmd/cabo/rsrc_windows_amd64.syso` 提供，**这个文件已经提交在仓库里，正常 build 不需要额外步骤**。只有改图标时才需要重新生成（见第八节）。
- 加了这个参数后 `fmt.Println` 和 `log` 的输出全部丢失，所以诊断信息走 `-log` 的文件。
- 环境要求：Go 1.22+（用了 `go:embed`）、Windows 10/11、WebView2 运行时（Win11 自带；本机版本 156.0.4314.50）。
- **不需要 cgo**。用的是 `github.com/jchv/go-webview2`（纯 Go 绑定），所以不需要 gcc。

---

## 三、项目结构

```
E:\Cabo\
├── cabo.exe                     成品（GUI 版）
├── go.mod / go.sum
├── README.md                    给玩家看的说明
├── HANDOVER.md                  本文件
├── cmd\cabo\
│   ├── main.go                  入口：解析参数、起本地服务、调窗口或浏览器
│   ├── window_windows.go        WebView2 原生窗口
│   ├── window_other.go          非 Windows 的占位实现
│   ├── cabo.ico                 应用图标源文件
│   └── rsrc_windows_amd64.syso  图标资源（rsrc 生成，已提交）
├── internal\
│   ├── engine\                  ★ 规则引擎，纯函数、零 I/O，最重要
│   │   ├── card.go              牌、点数、技能判定、牌组构成
│   │   ├── state.go             对局状态、发牌、推进、结算
│   │   ├── rules.go             Apply()：唯一的规则入口
│   │   ├── event.go             结构化事件，驱动前端动画
│   │   ├── view.go              视角裁剪，防止暗牌泄漏
│   │   └── rules_test.go        15 条规则用例
│   ├── ai\
│   │   ├── ai.go                启发式 AI
│   │   └── ai_test.go           60 个种子 × 全程牌数守恒
│   └── server\
│       ├── server.go            HTTP 接口 + 静态资源
│       ├── prefs.go             音量设置的持久化（写文件，见 5.6 节）
│       ├── errors.go
│       ├── server_test.go       端到端打完整局
│       └── static\              前端（编译时嵌入）
│           ├── index.html
│           ├── style.css        约 1120 行
│           ├── app.js           约 740 行
│           └── audio.js         音频层（合成 + 配乐），约 810 行
└── docs\
    ├── CABO-玩法详解.md         完整规则调研（含各版本差异）
    └── 开发准备清单.md          立项时的规划文档
```

代码量：Go 约 2260 行，前端约 2670 行。

---

## 四、核心设计（改之前务必先看）

### 4.1 规则引擎是纯函数

`engine.Apply(state, player, action) (State, error)` —— 不碰网络、文件、时钟，随机数通过注入的 `*rand.Rand` 提供。

**这条约束不要破坏。** 它让引擎可以 100% 单测、可以用种子精确回放、可以脱离 UI 跑模拟。

### 4.2 隐藏信息必须裁剪

引擎内部知道所有牌，但发给客户端的报文只能包含该玩家有权看到的信息：

```go
v := engine.ViewFor(s.state, 0)  // 0 号玩家的视角
```

**绝对不能直接 `json.Marshal(state)`** —— 那会把所有人的暗牌发出去。

`Slot.KnownBy []int` 记录「哪些玩家知道这张牌的内容」。牌被 SWAP 走时 `KnownBy` 会被重置，实现「半辅助记忆」：你开局看过的牌会一直显示，但一旦被别人换走，标记立刻清除。

### 4.3 事件驱动动画

引擎每次状态变化会写一条 `Event`（谁、做了什么、涉及哪张牌、指向哪个位置），服务端 `reply()` 投递一次后清空 `s.state.Events`。前端拿事件播动画，而不是解析文字日志。

事件类型见 `internal/engine/event.go`：`roundStart / peek / draw / discard / place / take / penalty / spy / swap / cabo / score / gameOver`。

### 4.4 先播动画，再切状态

`app.js` 里 AI 走的是 `applyViewAnimated()`：

```js
await playFlights(v.events);   // 用旧画面播完飞牌
setView(v, { skipCabo: true }); // 再切到新状态
```

这样「牌飞过去」和「牌落到堆里」是先后关系，而不是同时发生——顺序感就来自这里。**玩家自己的操作走同步路径**（`applyView`），保持跟手。

### 4.5 牌组：没有花色

牌面只有 `ID` 和 `Rank`，没有 `Suit` 字段。

| 数字 | 张数 |
|---|---|
| 0 | 3 |
| 1–12 | 各 4 |
| 13 | 3 |

合计 54 张，总点数 351，均值 6.5——和一副标准扑克（含两张王、方块 K 记 0）**统计上完全等价**。改牌组时注意保持这个分布，否则平衡性会变。

### 4.6 规则口径（固定为第二版）

| 规则点 | 采用 |
|---|---|
| 开局看牌 | 自选 2 张，看完位置不可变 |
| 技能映射 | 7/8 看自己，9/10 看对手，11/12 交换 |
| 从弃牌堆拿的牌 | 保持明牌 |
| 1 换多的判定 | 换出的牌**彼此**同点（不是与被拿走的牌同点） |
| 1 换多上限 | 3 张 |
| 翻错惩罚 | 收下弃牌堆顶牌；涉及 ≥3 张再罚抽 1 张 |
| 弃牌堆拿牌触发技能 | 不触发 |
| 喊 CABO 后 | 其他人各行动一次，喊者手牌锁定不可被换 |
| 平局 | 喊者胜 |
| 牌堆摸空 | 立即结束本轮计分 |
| 100 分重置 | 恰好 100，一场限一次 |
| 超过 100 分 | 打满整轮再结算 |

未实现：神风特攻队（两张 12 + 两张 13）彩蛋、线上版的 Burn 抢牌。两者都在引擎之外，加进来不用动核心规则。

---

## 五、前端设计系统

### 配色（浅色主题）

```
--bg        #FBF6EC   奶油底
--surface   #FFFFFF   卡片/面板
--coral     #FF7A59   主强调（按钮、品牌、自己）
--teal      #1FA79A   玩家 1
--violet    #8B6FE8   玩家 2
--amber     #E8A33D   玩家 3
--ink       #2F2A24   主文字
--ink-2     #7A7166   次要
--ink-3     #A79C8E   提示
```

**注意：曾经是深靛蓝主题，用户明确反馈「太压抑」后整体翻成浅色。** 以后做新界面优先浅色。

### 卡面色条编码

左侧 6px 竖条用颜色表示技能类型，右上角是技能标（看/窥/换）：

| 色值 | 含义 |
|---|---|
| `#E8A33D` 琥珀 | 0 分（最值钱） |
| `#93A1B5` 石板 | 1–6 |
| `#1FA79A` 青 | 7–8 透视 |
| `#8B6FE8` 紫 | 9–10 窥探 |
| `#F2823C` 橙 | 11–12 置换 |
| `#E8544A` 红 | 13 高危 |

**色条必须用 `linear-gradient` 画在 `background` 上，不能用绝对定位的 `::before`** —— 后者在圆角裁切下会渲染成一块独立的色块。

### 界面语言

**品牌名用英文 `CABO`，其余全部中文。**

- 英文：顶栏 logo、首页大标题、`<title>`（`CABO 卡波`）、窗口标题、按钮「喊 CABO」、CABO 横幅、exe 属性（`--product-name "CABO"`、`--file-description "CABO 卡波"`）。
- 中文：所有说明文字、提示、规则、结算面板。

曾经试过全部改成中文「卡波」，用户反馈还是想要 CABO 的英文，已改回。改 logo 文案时记得同步 `.logo` / `.brand-cabo` 的 `letter-spacing`——英文用 5px/12px，中文要缩到 3px/10px，否则两个字会拉得很开。

### 动效清单

发牌斜飞入落定、3D 翻牌、弃牌落入、牌堆弹跳、飞牌（WAAPI 弧线）、胶囊提示、技能脉冲、CABO 全屏横幅、座位呼吸光晕、结算条生长 + 数字滚动、对手思考小点。

### 5.5 运动语言（v12 重做）

**别再往单个动画里手写 `cubic-bezier(...)`**，用 `:root` 里这组变量：

| 变量 | 值 | 用在哪 |
|---|---|---|
| `--e-out` | `cubic-bezier(.22,.61,.36,1)` | 常规减速、进度条、过渡 |
| `--e-swing` | `cubic-bezier(.16,1.05,.3,1)` | 入场带回弹 |
| `--e-spring` | `cubic-bezier(.34,1.46,.64,1)` | 座位上浮这类明显过冲 |
| `--e-soft` | `cubic-bezier(.4,0,.2,1)` | 往复（思考小点） |
| `--e-in` | `cubic-bezier(.55,0,.68,.19)` | 退场 |

节奏：微反馈 ~160ms、入场 340–560ms、演出 900–2000ms。卡牌类动效一律带**过冲回落**（squash & stretch），这是「纸牌有重量」和「UI 元素在滑动」的区别。

**几个必须知道的实现约束（都是踩出来的）：**

- **翻牌必须从 `rotateY(88deg)` 起手**。之前从 -104° 开始，等于跳过了「侧成一条线」这个最关键的瞬间，牌面又全程清晰，看着只是牌在轻微倾斜。现在 0% 是一条窄缝 → 62% 越过正面带抬起 → 82% 回摆 → 100% 落定，配合 `.front::before` 的扫光。牌只有一个 `face`（暗牌内容根本不下发，做不了真假两面），所以 `.face { backface-visibility: hidden }` 是必要的。
- **同一个元素上不要放两个 `animation`**。CSS 里 `animation` 是整体覆盖的，后写的规则会把先写的直接顶掉。踩过两次：① `.seat.receive` 顶掉 `.seat.active` 的呼吸动画 → 呼吸光和 receive 光环拆到 `::before` / `::after` 两层；② `.discard-holder.landing` 的 scale 和 `.card.dropIn` 抢 transform → frame 只做 `landDip` 下沉、冲击环走 `::after`、牌的入场留给 `.card.dropIn`。
- **选中环要画在 `.card::after` 上，不能画在 `.front` 上**。开局挑 2 张看的时候四张牌全是背面，DOM 里根本没有 `.front` 元素，环画上去等于没有。
- **音量弹层必须挂到 `document.body`，不能留在 `.snd` 里**。两个独立致因：① `.topbar` 有 `backdrop-filter`，会造出独立层叠上下文，弹层被困在里面，而 `.table` 在 DOM 里更靠后 → 弹层被牌桌整个盖住；② 首页 `.panel` 是 `overflow-y:auto` 的滚动容器，弹层会被裁掉。改成 `position: fixed` + `placePop()` 按按钮 rect 定位（左右夹进视口、下方放不下自动朝上翻），`z-index: 95`（高于 `.overlay` 的 80）。
- **弹层显隐用 `visibility + opacity` 的 `.open` 类，不要用 `hidden` 属性**。`hidden` 有两个坑：作者写的 `display:flex` 会盖掉 UA 的 `[hidden]{display:none}`（害弹层永远展开着压在牌桌上）；而 `display` 的有无切换又会让元素脱离渲染树、脚本翻回可见时可能赶不上那一帧重绘。`visibility` 方案让元素常驻渲染树，切换只是属性变化，顺带还拿到淡入。
- **无头截图看不到「脚本执行后才翻为可见」的 fixed 元素**。实测弹层所有诊断值都对（`display:flex`、`opacity:1`、`visibility:visible`、`zIndex:95`、`elementFromPoint(中心)` 返回它自己），连 transition 关掉直接落终态也一样，像素区仍是空白；而首屏就可见的 fixed 元素能正常截到。这是 headless 的重绘缺陷，不是产品问题——**验证「点击才出现」的层叠问题别依赖无头截图，用 `elementFromPoint` 判定谁在最上层**。
- 内置浏览器面板若报 `NATIVE_BROWSER_VIEWPORT_UNAVAILABLE`（viewport 0×0、`visibilityState=hidden`），真实点击不可用，但 `evaluate_script` 和 `take_snapshot` 仍可用。
- **飞牌的弧度高度跟着距离走**（`app.js` 的 `flyCard`：`lift = clamp(22 + dist*0.17, 26, 98)`）。写死 66px 的话近距离飞行像飘、远距离像贴地滑。旋转方向也按 `dx` 符号走，所以 swap 的两张牌自然反向。
- **`.turn-chip` 的 `swap` 类只在真的换人时才挂**。每次 render 都重写 `className` 会把还没播完的动画掐断，用 `el.dataset.turn` 比对。
- **`.hidden` 是 `display:none`**，退场动画必须先播完再挂上 → 用 `hideOverlay(el)`（`app.js`），别直接 `classList.add('hidden')`。

发牌错峰在 `makeCard` 的 `idx * 62 + 'ms'`；AI 节奏仍在 `runAI()`（820ms / place 阶段 1050ms）；飞牌时长在 `FLIGHT_DUR`。

所有动画仍受 `prefers-reduced-motion` 的全局降级规则约束（`style.css` 末尾），新增的伪元素动画也一并覆盖。

**AI 节奏在 `app.js` 的 `runAI()` 里**：每步停 820ms，若已在 `place` 阶段停 1050ms。想调快慢只改这一处。

飞牌时长在 `FLIGHT_DUR`：普通 700ms、swap 900ms、cabo 900ms。

飞牌的关键帧是五段的：起点缩到 0.45 并旋转 -16°，迅速放大到 0.94 出现，飞行中途上抛到 1.22 倍并带光晕（`filter: drop-shadow`），接近终点回到 1.04，最后缩小淡出。落点会回调 `onLand`，触发弃牌堆的 `landPop` 冲击波或座位的 `receive` 光晕。

所有动画都尊重 `prefers-reduced-motion`。

### 5.6 音频层（audio.js）

**没有使用任何音频素材文件**——所有声音都由 Web Audio 实时合成。理由：本机没有 ffmpeg／Python，无法转码或跑 AI 生成模型；而成品是「单个 exe 拷走就能玩」，几 MB 素材会把体积抬高一个量级。代价是音色偏合成器，不像真实采样。

体积成本只有约 40 KB 代码。

**结构**（`static/audio.js`，810 行，全文件包在一个 IIFE 里）：

- **总线**：`sfxBus` / `musicBus` → `ducker` → `master` → `limiter`（DynamicsCompressor，阈值 -10 dB）→ destination。另有一条 `revIn → ConvolverNode → master` 的混响支路。
- **混响**：没有 IR 文件，用「白噪声 × 幂衰减 + 单极低通 + 15 ms 预延迟」在运行时生成 2.1 秒的立体声冲激响应。
- **音色基元**：`osc()`（振荡器 + 可选滤波 + 包络）、`noise()`（噪声 + 带通/高/低通）、`pluck()`（音乐盒/木琴：基音 + 微非谐泛音 + 敲击瞬态）、`bell()`（非谐泛音 2.76/5.4/8.93）、`bassNote()`、`flick()`（纸牌甩动 = 三段噪声瞬态）、`whoosh()`、`thud()`。
- **23 个音效**：`ui select deselect deal settle flip draw discard place take land receive swap peek spy penalty error cabo start score tick win lose`。`play(name, arg)` 是唯一入口，内部 try/catch，音频出错不影响牌局。

**按键反馈的接法**：`app.js` 底部一条委托监听 `document.addEventListener('click', e => { if (e.target.closest('button')) snd('ui') })`，动态生成的操作按钮和键盘 Enter 触发的 click 都能覆盖。

**事件音效的关键约束**：AI 的动作是「先 `playFlights()` 播飞牌、再 `setView(v, {flown: true})`」。`playOne()` 里的 `flown` 用来判断**这条动作的运动音效是否已经响过**——飞过了就不重复发声，只补胶囊提示。玩家自己的动作走同步路径（没有飞牌），动作音由 `playOne` 发。`opts.skipCabo` 已改名为 `opts.flown`，同时管横幅和声音两件事。

**配乐**：D 大调 **3/4 拍圆舞曲**，100 BPM，16 小节循环（两个 8 小节段：`D G Bm A D G A Dsus` / `Bm A G D Bm A G A`）。选 3/4 是为了避开 4/4 + backbeat 的赌场/酒吧感，这跟 README 里「CABO 是记忆桌游不是赌场」一致。旋律不是写死的：每小节按当前和弦从 4 组动机 × 6 种节奏型里摇，音高只取和弦内音（经过音用 `SCALE` 吸附到调内），句尾延长。听感是「同一首曲子一直在变奏」，而不是随机音符。

三个模式由 `musicMode(view)` 在每次 `setView` 时决定：

| 模式 | 触发 | 速度 | 配器 |
|---|---|---|---|
| `menu` | 首页 / 结算 / 结束 | 76 | 稀疏音乐盒 + 和弦织体 |
| `table` | 正常对局 | 100 | 低音 + 三拍和弦 + 旋律 + 沙锤 |
| `tension` | 有人喊过 CABO 或分数 ≥ 80 | 112 | 旋律降八度、去沙锤、加底鼓脉冲 |

换模式时 `bpm` 每步朝目标挪 1.2%，是滑过去而不是跳过去。

**音量设置存哪**：见 5.7。

### 5.7 音量持久化（prefs.go）

**踩坑：光靠 localStorage 不够。** WebView2 每次启动随机分配端口，浏览器把 `127.0.0.1:61862` 和 `127.0.0.1:52341` 当成两个不同来源，localStorage 因此每次启动都归零——用户静音过一次，下次打开又是满音量（或者反过来）。

所以真正的持久值写在 `%LOCALAPPDATA%\Cabo\prefs.json`，走 `GET/POST /api/prefs`。前端仍留一份 localStorage 作为首屏的即时缓存，但**服务端返回后会覆盖它**（`prefsTouched` 标志防止用户刚拖完滑块就被远端旧值刷回去）。

写文件前 `io.LimitReader(4096)` + 数值夹紧到 0–1，路径不接受外部输入。

---

---

## 六、未完成 / 可以继续做的
| 优先级 | 事项 | 说明 |
|---|---|---|
| 中 | **应用图标** | 目前是 Windows 默认 exe 图标。`windres` 在 `E:\MinGW\bin\windres`，做法：生成 `.ico` → 写 `cabo.rc`（`1 ICON "cabo.ico"`）→ `windres -O coff -i cabo.rc -o rsrc_windows_amd64.syso` → 放进 `cmd/cabo/` 后重新 build。图标建议用珊瑚底 + 白色菱形（和牌背同一套语言） |
| 低 | 热座模式 | 现在只有「人 vs AI」。**做过一版（视角跟着行动者 + 设备交接屏），用户要求撤掉了**，别再直接重做，先问清楚不要它的原因 |
| 低 | 联机对战 | 现在 `server.Server` 只维护一局全局游戏，多标签页会互相干扰。要做联机得改成房间模型 |
| 低 | Burn 抢牌 | 线上版机制，需要并发仲裁，属于第二阶段 |
| 低 | 神风特攻队 | QQKK 彩蛋，加在 `endRound()` 里即可 |
| 低 | 窗口图标验证 | 图标已嵌入 exe（字节级验证过），`IconId: 1` 对应 go-winres 生成的资源 ID 1。但没能亲眼确认标题栏/任务栏的显示效果——见第七节「沙箱限制」 |

---

## 七、环境与踩坑记录（能省很多时间）

### 7.1 已踩过的坑

**Python 发请求**：`urllib` 连续发约 160 个请求后会被本机重置连接（Windows 端口层问题，不是服务端 bug）。
→ 用 `http.client.HTTPConnection` 复用长连接，不要用 urllib。

**后台进程**：用 `cmd &` 起的进程会在 shell 退出时被回收。
→ 用工具的 `run_in_background`。

**桌面截图**：PowerShell 的 `Add-Type` 和 `Reflection.Assembly.Load` 都被安全策略拦截，无法截桌面。
→ 想验证 WebView 渲染，用 `-debug -log` 看 `%LOCALAPPDATA%\Cabo\cabo.log` 里的自检输出。

**无头浏览器截图**：Edge headless 会拍到动画中间态（元素透明、面板半透明）。
→ 加 `--force-prefers-reduced-motion --virtual-time-budget=6000` 让动画瞬时完成。

**CSS 类名冲突**：踩过两次——`.me`（玩家区 flex column）撞 `.score-row.me` 导致结算行竖排；`.primary`（弹窗按钮 width:100%）撞 `.btn.primary` 导致操作按钮被拉成通栏。
→ 通用词类名一定要加作用域。

**CSS 优先级**：`.btn:hover:not(:disabled)` 权重 0,3,0 高于 `.btn.primary` 的 0,2,0，hover 时把金色渐变整个替换成透明白，文字还是深棕 → 看不见。
→ 现在改成 CSS 变量分层（基类只用 `var(--btn-bg)`，变体只覆盖变量），只有一条 hover 规则。

**rAF 依赖**：数字滚动用 `requestAnimationFrame`，在 rAF 不推进的环境（后台标签页、无头）会永远停在初始值。
→ 先把正确值写进 DOM，动画只作增强。

**飞牌元素泄漏**：`anim.onfinish` 在部分环境不触发。
→ 必须加 `setTimeout(cleanup, dur + 320)` 兜底。

**⚠️ 生成图标资源只能用 `go-winres`，不能用 `windres` 也不能用 `rsrc`**：

- MinGW 6.3 的 `windres -O coff` 会产生 Go 链接器不认的重定位类型，直接编译失败（`unknown relocation type 7`）。
- `rsrc` 能编译通过，但**生成的 `.syso` 会让 `-H windowsgui` 版本在进入 `main()` 之前就崩溃**——双击闪退、无窗口、无日志、进程内存停在 14MB 左右。这个坑排查了很久：用 `os.WriteFile` 在 main 第一行打标记才确认它根本没进 main。
- 正确做法：
  ```bash
  go install github.com/tc-hib/go-winres@latest
  cd cmd/cabo
  $(go env GOPATH)/bin/go-winres.exe simply --icon cabo.ico \
      --product-name "CABO" --file-description "CABO 卡波" \
      --product-version 1.0.0.0 --file-version 1.0.0.0
  ```
  会生成 `rsrc_windows_amd64.syso` 和 `rsrc_windows_386.syso`，**把 386 那个删掉**（只留 amd64）。
- 验证方法：GUI 版启动后进程内存应该在 30MB 以上（WebView2 已加载）。如果停在 14MB 左右，就是资源文件的问题。

**WebView2 的 JS 注入不可靠**：`w.Init()` 底层调 `AddScriptToExecuteOnDocumentCreated` 但**丢弃了返回值**，失败也不报错；`w.Dispatch + w.Eval` 同样没成功。
→ 别指望用它做自检。要确认页面有没有加载，看 `-log` 的请求日志：出现 `GET /style.css`、`GET /app.js`、`GET /api/state` 就说明 HTML/CSS/JS 全部加载且 JS 执行了。

**日志文件会被僵尸进程占用**：程序每次启动都用 `os.Create` 覆盖 `cabo.log`。如果上一次的 cabo.exe 没退干净，新进程写不了日志（不致命，但会看不到诊断信息）。
→ 排查前先 `taskkill /F /IM cabo.exe`。

**沙箱限制（仅影响本机调试，不影响成品）**：从 `E://Cabo` 目录启动 exe 时 `net.Listen` 会报 `winapi error #10106`（Winsock 提供者初始化失败），拷到 `C://Users//...//Temp` 下就正常。
→ 调试时把 exe 拷到临时目录跑。用户正常双击运行不受影响。

### 7.2 测试策略

三层，`go test ./...` 全绿：

1. **规则单元测试**（`internal/engine`）—— 15 条，覆盖 1 换多、翻错惩罚、CABO 计分、平局、牌堆耗尽。
2. **AI 对局守恒**（`internal/ai`）—— 4 个 AI 用 60 个种子互打，每步断言「牌堆 + 弃牌堆 + 所有手牌 + 待安置牌 = 54」。**这条断言抓出过两个真实 bug**：换出的牌没进弃牌堆。改动引擎后一定要跑。
3. **HTTP 端到端**（`internal/server`）—— 通过真实接口打完整一局，验证无非法状态、无死循环、结束条件正确。

---

## 八、分发时会被 SmartScreen 拦

双击 exe 可能弹「Windows 已保护你的电脑 / 发布者：发布者未知」。**这不是病毒，是 exe 没有代码签名、也没有信誉积累**，Windows SmartScreen 对「未知发布者」的默认行为。

已确认的事实（2026-10-30 查）：
- 文件没有 `Zone.Identifier` 数据流（不是"来自网络"的锁定问题）
- `Get-AuthenticodeSignature` 返回 `NotSigned`
- Smart App Control 为 `Off`，不是它在拦
- `HKCU\...\AppHost\EnableWebContentEvaluation` 未设置（默认开启「检查应用和文件」）

处理办法，按侵入性排序：

1. **单次放行**：弹窗里点「仍要运行」。
2. **关掉这个用户的 SmartScreen 应用检查**（无管理员权限）：
   `Set-ItemProperty 'HKCU://Software//Microsoft//Windows//CurrentVersion//AppHost' -Name EnableWebContentEvaluation -Value 0`
   代价是下载来的未知程序也不再提示，安全等级下降。
3. **Defender 排除项**：只对病毒扫描有效，**对 SmartScreen 无效**，别搞混。
4. **根治**：买代码签名证书（OV/EV）。EV 证书能立刻建立信誉；OV 证书首次仍可能提示，要等信誉积累。自签名证书没用——不在信任链里，反而更麻烦。

如果只是自己玩，方案 1 或 2 就够了。

## 九、下一步怎么接

如果用户说「继续」：

1. 先 `go test ./...` 确认基线是绿的。
2. 问清楚要做什么。剩下的方向：联机、Burn 抢牌、神风特攻队彩蛋。（图标、音效、难度已完成；热座做过又被撤掉了。）
3. 改引擎的话，**先看 `internal/engine/rules.go` 的 `Apply`**，那是唯一的规则入口；改完必须跑 `internal/ai` 的守恒测试。
4. 改界面的活基本都在 `internal/server/static/` 三个文件里，改完 `go build` 重新出 exe（静态资源是编译时嵌入的，不重新 build 看不到变化）。
5. 视觉上保持浅色系和「珊瑚橘 + 白色卡片」这套语言，别再回到深色。

### 布局适配

窗口可以随意缩放，但布局有最低高度要求。现在的做法：

- 卡牌尺寸用 `clamp()` 跟随视口高度：`--hand-w: clamp(58px, 11.2vh, 96px)`、`--seat-w: clamp(42px, 7.4vh, 62px)`、`--pile-w: clamp(52px, 7.8vh, 66px)`。手牌区、牌堆、弃牌堆都引用这三个变量，改一处全跟着变。
- `@media (max-height: 620px)` 会进一步压缩内边距和卡牌尺寸。
- 原生窗口设了最小尺寸 `w.SetSize(780, 560, webview2.HintMin)`——只设置下限，不会改变初始大小。
- `.table` 留了 `overflow-y: auto` 兜底，极端情况下可以滚动到按钮。

**曾经踩过的坑**：早期版本卡牌是固定 96px，窗口缩到 870×580 时底部的手牌和「看牌」按钮会被裁掉（`overflow: hidden`），游戏根本开不了。改样式后务必在 780×500 和 1280×860 两个尺寸都截图确认按钮可见。

### 换图标

```bash
cd cmd/cabo
# 改 cabo.ico 后重新生成资源（必须用 go-winres，原因见第七节）
$(go env GOPATH)/bin/go-winres.exe simply --icon cabo.ico \
    --product-name "CABO" --file-description "CABO 卡波" \
    --product-version 1.0.0.0 --file-version 1.0.0.0
rm -f rsrc_windows_386.syso
cd ../..
go build -ldflags="-s -w -H windowsgui" -o cabo.exe ./cmd/cabo
```

图标是程序生成的（珊瑚渐变圆角方块 + 白色菱形，和牌背同一套语言），生成脚本没有留在仓库里，需要改的话照 `docs/icon-preview.png` 的效果重画即可。

如果用户说「还是不好看」：

- 先问具体是哪个元素（颜色？布局？动效？）
- 只调那一个，不要整体翻新——已经翻过两轮，每次都牵动大量 CSS
