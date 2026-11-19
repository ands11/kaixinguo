# Android 原生播放器替换 WebView 播放器

## Context（背景）

手机版在真机上播放一直转圈（模拟器正常）。排查确认转圈对应"视频数据一直不到位且无 error 事件"，头号嫌疑是各手机 WebView 版本/网络环境差异，`<video>` 元素卡死时不重试不报错。用户决定直接切换原生播放器根治，范围与决策：

- **只做 Android**（Kotlin + Media3 ExoPlayer）。macOS 桌面、iOS 继续走现有 WebView 路径。mygo native modules 契约：无实现的平台构造器返回 `errors.ErrUnsupported`（docs/native-modules.md L41-43），App 据此自动回退 WebView，无需构建标签分支。
- **弹幕用 Go UI 层渲染**，控制条布局延续现有播放器（模仿红果短剧）。
- 本地代理 `proxy.go`（127.0.0.1 取流 + CENC 解密 + Range）**完全不动**；ExoPlayer 直接播放代理 URL，且 `DefaultLoadErrorHandlingPolicy` 自带网络重试，直接缓解真机转圈问题。

已核实的框架事实：
- `mygo native new -lang kotlin player` 生成 `modules/player/` 骨架。
- `ui.NativeView(c, v).Fill().Children(...)`：Children 绘制在平台视图之上（弹幕/控制条/覆盖层都放这里）；`v == nil` 时元素退化为普通盒子照常画 children（ui/api_gen.go L1836-1839）——测试用 fake player（视图指针返回 0）即可在 `ui.NewTester` 中渲染全部 UI。
- `Window.Invalidate()` 可从任意 goroutine 调用（content.go L156-159）。
- 模块视图回调在 Go 主线程按序执行；视图在 `App.WhenReady` 主线程创建，不在视图构建中创建。
- 参考模板：docs/native-modules.md L797-873 有完整 Player（AVPlayerView）示例。

## 实现步骤

### 1. 模块骨架与 Go 接口（modules/player/player.go）

```sh
mygo native new -lang kotlin player
```

```go
package player

// State 是播放器的播放状态。
type State string

const (
    StatePlaying   State = "playing"
    StateBuffering State = "buffering"
    StateEnded     State = "ended"
)

//mygo:native
type Player interface {
    native.View
    io.Closer
    // Load 装载 url 并定位到 start；换集复用（重置画面与弹幕）。
    Load(url string, start time.Duration)
    Play()
    Pause()
    Seek(to time.Duration) // 主线程同步调用，ExoPlayer.seekTo 本身异步
    SetRate(rate float64)
    // OnEvent 订阅状态变化；error 时 code 为 "io"/"decode"/"drm" 之一，其余为 ""。
    OnEvent(f func(s State, code string)) io.Closer
    // OnTime 订阅位置与时长（暂停时停发）。
    OnTime(f func(pos, dur time.Duration)) io.Closer
}
```

教程 hold 语义不入接口：Go 决定何时调 `Play()`。

### 2. Kotlin 实现（kotlin/src/main/kotlin/.../Player.kt）

- `kotlin/build.gradle.kts` 加 `androidx.media3:media3-exoplayer` + `media3-ui`（用 maven 最新 1.x 稳定版；minSdk 26 > Media3 的 21）。
- `class KxPlayer : FrameLayout, Player`；内含 `PlayerView(useController=false, surface_type=TextureView, resize_mode=fit)`，省去自管 Surface。
- `makePlayer()`：`ExoPlayer.Builder(ctx).build()`，失败抛 `NativeUnsupportedException` → Go 拿到 ErrUnsupported → 回退 WebView。
- `Player.Listener`：`onPlaybackStateChanged` + `onIsPlayingChanged` → playing/buffering/ended；`onPlayerError` → 按 `ERROR_CODE_IO_*`/`*DECODING*`/`*DRM*` 归类为 io/decode/drm。
- 位置上报：主线程 `Handler` postDelayed 自循环，66ms 一拍，仅 isPlaying 时发 `(currentPosition, duration)`；暂停/ended 补发终值一拍。
- `close()`：移除监听、清 handler、`player.release()`。

### 3. Go 侧接线

- **main.go**：app 加字段 `player playermod.Player`、`eventSub, timeSub io.Closer`、`dm *danmakuUI`；`WhenReady` 中构造一次常驻：成功 → 注册 OnEvent/OnTime 订阅；`errors.ErrUnsupported` → 保持 nil（桌面/iOS 自动走 WebView）。
- **play.go**：
  - `play()`：native 分支用 `a.player.Pause()` 替代 `evalJS("kx.stop()")`；`kx.setHold(true)` 改为"暂不调 Play"。
  - `p.onPage` 改名 `p.loaded`；startEpisode resolve 成功后 native 分支 `a.player.Load(src, start)`，web 分支保持 `evalJS(kx.play(...))`。
  - **事件解耦**：抽 `func (a *app) playerEvent(kind, ep string, t, d float64, code string)` 承载 playing/waiting/progress/ended/error 处理（现 onPlayerEvent L246-278）；desktop `/event` 解析后调用它，android 回调也调用它（ep 用 `a.cur.epID`）。theater/pip/chrome/danmaku/next/prev 仅 web 页面会发，留在 onPlayerEvent。
  - 抽 `func (a *app) pausePlayback()`，替换 view.go 路由离开处和 main.go OnSuspend 的 `evalJS("kx.pause()")`。
- **view_player.go**：视频区改为
  `if a.web != nil { ui.WebView... } else if a.player != nil { ui.NativeView(c, a.player).Fill().Children(func(){ 弹幕层; tap层; 控制条 }) }`。
  playerOverlay（转圈/失败层）、theater 抽屉、episodesEdge 不动。
- **danmaku.go**：`flushDanmaku` native 分支把 `tr.lines[p.dmSent:]` 交给 `a.dm.feed(...)`；web 分支不变；`setDanmaku` 同步开关渲染器。

### 4. 弹幕 Go 渲染器（新文件 danmaku_ui.go）

- 状态全放 app（`danmakuUI`：lanes、flying、widths 缓存、lastPos/lastWall/rate），不存元素；文本宽度用 `c.TextSystem().Layout(...)` 在构建帧内测量并按文本缓存。
- 移植网页版算法：车道数 `clamp(h/28, 3, 6)`；时长 `clamp(W*13, 5500, 10000)ms`；同车道右端 28px 间距；迟到 ≤1.5s 补飞；等待 ≤2.5s 丢弃。
- 时钟：OnTime 回调更新 lastPos/lastWall/rate（主线程）；估计播放时刻 = `lastPos + (now-lastWall)*rate`；seek/Load 时重置车道并回退 dmSent 重 feed。
- 动画驱动：有飞行/待飞弹幕时启动 goroutine，`time.Ticker(50ms)` **只调 `a.win.Invalidate()`**（已核实任意 goroutine 安全），构建帧按插值重算 x 并回收出屏行。goroutine 只引用 a.win，规避 mygo vet 的 goroutine 捕获检查。

### 5. 控制条 Go 重写（手机端）

- **tap 切显隐**：视频区最上层透明 `ui.Box().Absolute().Fill()` 捕获 Click 翻转 `a.playerChrome`（参照 episodesEdge 在 WebView 上的做法）；播放中 4s 无操作自动隐藏（手机无 hover）。
- **底部悬浮条**（模仿红果）：`ui.Row().Absolute().Bottom(16).Left(16).Right(16)` 半透明圆角背景；进度 `ui.Slider`（绑定 app 字段，拖动中不被 OnTime 覆盖，结束才 Seek——实现时核对 Slider Changed 语义）+ 快退10s/播放暂停/快进10s/上一集/倍速菜单/弹幕开关/下一集；缺的图标补进 icons.go；倍速菜单参照现有 seasonPicker 风格。
- 顶部 exitButton/playerToolbar 复用；`playerChrome` 在 android 路径由 Go 自管。手机端无音量条、无 PiP（与现网页版 touch 模式一致）。

### 6. 测试

- player_test.go 加 fake player（记录调用序列、可手动发 State/OnTime；`NativeView()` 返回 0 → NativeView 退化为普通盒子，Tester 里弹幕层/控制条可渲染）：覆盖 play→Load、状态流转、pausePlayback 双分支、教程 hold（释放后才 Play）、seek 重置弹幕。
- 回归：`go test ./...`、`go tool mygo vet .`、`CGO_ENABLED=0 go build ./...`（桌面 ErrUnsupported 回退零回归）。

### 7. 验证（端到端）

1. `mygo native check modules/player`（Windows + 本机 Android SDK 已就绪）。
2. `mygo dev -platform android` 或构建 APK 装模拟器：播放/暂停/seek/倍速/弹幕飞屏/转圈→playing 流转/OnSuspend 暂停/连续换集/历史续播。
3. 真机回归：原始"一直转圈"消失（ExoPlayer 自动重试），弱网失败时显示兜底层并可点重试。
4. macOS 端零改动验证留给用户在 Mac 上构建（本机无法验证，如实报告）。

## 关键文件

- 新建：`modules/player/player.go`、`modules/player/kotlin/**`（`mygo native new` 生成后填实现）、`danmaku_ui.go`、`view_player_android.go`（控制条）
- 修改：`play.go`、`view_player.go`、`main.go`、`danmaku.go`、`icons.go`、`player_test.go`

## 风险

- mygo vet 对构建值/goroutine 捕获的检查：ticker 只引用 a.win；状态全放 app 数据。
- Slider 拖动/提交语义、ExoPlayer 时钟漂移：以最近一次 OnTime 为锚点插值，rate/seek 变化重置锚点。
- Media3 版本以实现时 maven 最新 1.x 为准；Gradle 首次构建较慢，先跑通 `mygo native check` 再动 Go 侧。
