# 播放器三布局改造（手机端）

## Context

手机端播放页目前只有一套 chrome（顶部退出/选集按钮 + 底部控制条，`view_native.go`）。参考红果短剧 App，需要按视频画幅区分三种布局：

- **图一** 竖屏视频：竖屏沉浸式——顶部 `← 第N集` / 右上 `1.25×`+`⋮`；中央播放键（暂停时）；右侧竖排互动栏（收藏/评论/点赞/分享带数字）；底部左 `弹` 开关+剧名+热评/AI 声明；红色细进度条；最底部 `选集·已完结·全60集 ⌃` 上拉抽屉把手 + 右下 `⛶` 全屏按钮。
- **图二** 横向视频竖持：视频 16:9 contain 铺宽置顶、下方黑区；视频右下浮 `⛶ 全屏观看` 按钮；视频下方右侧浮动收藏/评论按钮。
- **图三** 横向视频全屏：屏幕旋转横屏铺满——顶部左 `← 剧名 第N集`+顶部中时间码；中央暂停键；底部 `弹` 开关+弹幕输入框「发条友善的弹幕吧」+右 `1.25×`；白点细进度条。

**范围：仅手机端（`onPhone`）；桌面端布局不动。**

## 已确认的能力（探索结论）

| 能力 | 位置 |
|---|---|
| 视频画幅判定 | `a.player.VideoSize()` → `{Width, Height int64}`（`modules/player/player.go`）；fallback `Episode.Vertical`（openhongguo `catalog.go:356`） |
| 屏幕方向 | `mygo.SetOrientations(mygo.OrientationLandscape / OrientationPortrait / OrientationAny)`（mygo `device.go:113`） |
| 系统条隐藏 | `playerBars(on)` 已在播放页隐藏系统条（`view.go:1097`），横屏无需再处理 |
| 收藏数据层 | `favorites.toggle/add/remove/has` + `favOf(card)`（`favorites.go`），已有 `iconStar/iconStarOn` |
| 计数 | 收藏≈`Series.FollowCount`、点赞=`Series.DiggCount`、单集评论=`Episode.CommentCount`（catalog.go:274-278, 364-366）；格式化用 `heat()`（`data.go:300`） |
| 分享 | `mygo.Share(mygo.ShareOptions{...})`（mygo `device.go:135`） |
| 弹幕飞行 | `danmakuUI`（`danmaku_ui.go`）+ `bullet{at, text}`（`danmaku.go:38`）；SDK 无发送接口 → 发送=本地 append bullet 飞出（不持久化） |
| 倍速 | `playerSpeeds`/`rateMenu`/`setRate` 已有（`view_native.go`、`play.go:404`） |
| 选集 | `playerPanelBody`/`episodeChip`/`partPicker` 已有；phone 上现为右侧抽屉 `theaterPanel`，改为底部上拉 |

## 实现步骤

### 1. `icons.go` — 新图标
`iconHeart`（描边）/`iconHeartOn`（实心，点亮点色）、`iconComment`（气泡）、`iconShare`、`iconMore`（⋮）、`iconFullscreen`（四角 ⛶）、`iconChevronUp`。

### 2. 方向与全屏状态 — `main.go` + `play.go`
`app` 新字段：
- `videoLandscape bool` — 当前视频是否横向。在能拿到 VideoSize 的地方更新（`playerTime` 每 tick 便宜读取，或 VideoSize 变化事件）；换集（`startEpisode`）重置为按 `Episode.Vertical` 预判。
- `landscapeFull bool` — 处于横屏全屏（图三）。

助手方法（`play.go` 或新文件）：
- `enterLandscapeFull()`：`mygo.SetOrientations(mygo.OrientationLandscape)`
- `exitLandscapeFull()`：`SetOrientations(OrientationPortrait)`，置 `landscapeFull=false`
- `leavePlayer()` 与 `playerEscape()`：优先退出 `landscapeFull`；离开播放页时若在横屏先恢复 portrait。

### 3. 三布局分发 — 新文件 `view_mobile_player.go`，`view_native.go` 改分发
`nativeVideo()` 在 `onPhone && a.player != nil` 时分发：
- `landscapeFull` → 图三布局
- `videoLandscape` → 图二布局
- 其余 → 图一布局（替换现有 phone chrome；桌面与非 phone 走原 `playerBar` 路径不变）

**图一 phonePortraitVideo**（替换现有 `playerBar` 手机分支）：
- 顶部 Row：左 `← 第N集`（`exitButton` 改造为带集号的返回）；右 `🕐1.25×`（点击弹出现有 `rateMenu`）+ `⋮`（菜单：弹幕开关）
- 中央：`state != playPlaying` 时大播放键（胶囊形 iconPlay/iconPause）
- 右侧竖排 rail（Absolute Right，垂直居中偏下）：收藏⭐（实心=已收藏）+`heat(FollowCount)`；评论💬+`heat(CommentCount)`；点赞❤+`heat(DiggCount)`（本地 toggle）；分享↗（`mygo.Share`，文案=剧名）
- 底部左：`弹` 开关（沿用 `setDanmaku`）、剧名 `Title ›`（点击回详情页）、`🔥热评/内容由AI生成` 行（`Series.Subtitles` 有则显示，否则显示 AI 声明静态行）
- 底部红细进度条：4px 高、圆角、红色填充（`cur.dur` 比例），不可拖
- 最底部 Row：`选集 · {episodesLabel} ⌃` 把手（打开底部选集抽屉）+ 右侧 `⛶`（进入图三横屏全屏）
- tap 层行为不变（点击出/收 chrome）

**图二 phoneLandscapeVideo**：
- 视频仍由 NativeView 铺满（播放器自身 contain）；叠加层按 VideoSize 算出视频矩形（复用 `dmView` 中同样的 contain 计算），`⛶ 全屏观看` 胶囊按钮钉在视频矩形右下内侧；点击 → `enterLandscapeFull()`
- 视频矩形下沿之下、右侧浮动竖排：收藏⭐+计数、评论💬
- 失败/加载 overlay 不变

**图三 phoneFullscreenVideo**（横屏）：
- 顶部 Row：左 `← 剧名 第N集`；中 `clock(at)` 时间码
- 中央：暂停时大暂停/播放键
- 底部 Row：`弹` 开关 + 弹幕输入框（TextField，placeholder「发条友善的弹幕吧」；发送 → `a.dm.feed([]bullet{{at: 当前playhead, text}})` 本地飞出并清空）+ 右 `1.25×`
- 白点进度条：细线 slider（复用 `seekSlider`，白色轨道+白点把手）
- 退出：左上 ← 或系统返回 → `exitLandscapeFull()`

**选集底部抽屉**：phone 上将 `theaterPanel` 分支改为 bottom sheet（从底部滑出，复用 `playerPanelBody`；`panelOpen` 状态沿用；点把手/收起按钮关闭）。保留现有 `panelReveal` 滚动到当前集逻辑。

### 4. 互动接线
- 收藏：`card{ID: sp.id, Title: sp.series.Title, Cover: sp.series.CoverURL, Episodes: len(sp.episodes)}` → `a.favs.toggle(favOf(...))`
- 点赞：`liked bool` 本地状态（不持久化），计数 `DiggCount + (liked?1:0)`
- 评论/分享为只读/系统分享；评论点击弹轻提示「评论功能暂未开放」(toast 或短暂文字)
- `⋮` 菜单：弹幕开关（+已收藏状态展示），样式复用 `rateMenu` 的浮层写法

### 5. 回归与验证
- `go test ./...`（fakePlayer 路径，需补 `videoLandscape`/`landscapeFull` 相关最小测试或保证不回归）
- `go tool mygo vet .`（app 根目录）
- `go tool mygo build -platform android -o build/android .` → 装 emulator-5554 截图验证：
  - 竖屏剧（183,445→202,772）→ 图一布局截图
  - 横屏剧 → 图二布局（全屏观看按钮位置）
  - 点全屏观看 → 图三布局（横屏+输入框+进度条）
  - adb: `$env:LOCALAPPDATA\Android\Sdk\platform-tools\adb.exe`，截图缩放比 461/1080
- 真机 K7V4HYS8PZLZ8LON 安装验证（用户手动操作）

## 关键文件
- 改：`view_native.go`（分发）、`main.go`（字段）、`play.go`（方向判定/escape/leavePlayer）、`icons.go`、`view_player.go`（phone 选集抽屉、playerEscape 微调）
- 新：`view_mobile_player.go`（三布局 + 底部抽屉 + 互动栏）
- 复用：`rateMenu/seekSlider/playerIconButton`（view_native.go）、`playerPanelBody/episodeChip`（view_player.go）、`favorites`、`heat()`、`mygo.Share`、`mygo.SetOrientations`
