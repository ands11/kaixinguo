package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	playermod "github.com/kadaliao/kaixinguo/modules/player"
	"github.com/egoist/mygo"
)

// playback is the episode the player shows. It belongs to the main thread.
type playback struct {
	seriesID string
	number   int
	start    float64 // where to start, in seconds
	gen      int     // tells this start from later ones
	state    playState
	msg      string
	pending  bool // waits for the series to load

	epID   string  // the episode, once its series has loaded
	at     int64   // the playhead, in ms, as the player last told it
	dur    float64 // the episode's length, in seconds, as it said
	loaded bool    // the episode's URL reached the player
	paused bool    // the viewer paused it, and the player says nothing
	dmSent int     // how many of its 弹幕 lines the player has
}

type playState int

const (
	playResolving playState = iota // finding the episode's file
	playBuffering                  // the video loads
	playPlaying
	playFailed
)

// future is an episode's playback model being resolved, shared by the
// start of the episode and the prefetch before it.
type future struct {
	done chan struct{}
	pb   *model.Playback
	err  error
}

// resolve returns the episode's playback model, resolving it unless a
// prefetch did or is doing it.
func (a *app) resolve(ep model.Episode) *future {
	if f := a.futures[ep.ID]; f != nil {
		select {
		case <-f.done:
			if f.err == nil && !expired(f.pb) {
				return f
			}
		default:
			return f
		}
	}
	f := &future{done: make(chan struct{})}
	a.futures[ep.ID] = f
	go func() {
		defer close(f.done)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		f.pb, f.err = a.client.ResolvePlayback(ctx, ep.ID, sdk.PlaybackOptions{})
		if errors.Is(f.err, sdk.ErrDRMProtected) && f.pb != nil {
			f.err = nil // ContentKey decrypts it
		}
	}()
	return f
}

// expired reports a model whose CDN URLs expire within a minute.
func expired(pb *model.Playback) bool {
	return pb == nil || pb.ExpiresAt > 0 && time.Unix(pb.ExpiresAt, 0).Before(time.Now().Add(time.Minute))
}

// play opens the player on episode n of the series, from start seconds in.
func (a *app) play(seriesID string, n int, start float64) {
	a.gen++
	a.cur = &playback{seriesID: seriesID, number: n, start: start, gen: a.gen}
	switch path := a.router.Path(); {
	case path == "/play/"+seriesID:
	case strings.HasPrefix(path, "/play/"):
		// Another season, in the same place: Back leaves the player.
		a.router.Replace("/play/" + seriesID)
	default:
		a.router.Push("/play/" + seriesID)
	}
	if a.player != nil {
		a.player.Pause() // Load of the next episode replaces the video
	} else {
		a.ensureWebView()
		a.evalJS("kx.stop()")
	}
	a.holdPlayback(a.playerTutorialOn) // before the episode resolves and plays
	sp := a.seriesPage(seriesID)
	if sp.series == nil {
		a.cur.pending = true // fillPending starts it
		return
	}
	a.startEpisode(sp)
}

// fillPending starts the episode that waited for its series.
func (a *app) fillPending(sp *seriesPage) {
	if p := a.cur; p != nil && p.pending && p.seriesID == sp.id {
		p.pending = false
		a.startEpisode(sp)
	}
}

func (a *app) startEpisode(sp *seriesPage) {
	p := a.cur
	ep, ok := sp.episode(p.number)
	if !ok {
		p.state, p.msg = playFailed, fmt.Sprintf("没有第 %d 集", p.number)
		return
	}
	if !ep.Playable {
		p.state, p.msg = playFailed, "这一集需要登录才能观看"
		return
	}
	if sp.series != nil && sp.series.Platform == model.PlatformUnknown {
		p.state, p.msg = playFailed, "这是小说或有声书，不能播放"
		return
	}
	p.state, p.msg = playResolving, ""
	p.paused = false
	p.epID, p.at = ep.ID, int64(p.start*1000)
	// The phone guesses the picture's shape from the episode until the
	// player tells the real one: a portrait drama plays 图一's layout.
	a.videoLandscape = false
	a.liked = false
	a.recordWatch(sp, p.number, p.start, 0, false)
	a.feedDanmaku() // as the episode resolves
	gen, start := p.gen, p.start
	f := a.resolve(ep)
	go func() {
		<-f.done
		src, err := a.streamURL(f)
		a.update(func() {
			if a.cur == nil || a.cur.gen != gen {
				return
			}
			if err != nil {
				a.cur.state, a.cur.msg = playFailed, friendly(err)
				delete(a.futures, ep.ID) // try afresh next time
				return
			}
			a.cur.state = playBuffering
			a.cur.loaded = true
			a.dm.reset()
			if a.player != nil {
				a.player.Load(src, time.Duration(start*float64(time.Second)))
				if !a.holdOn {
					a.player.Play()
				}
			} else {
				a.evalJS(fmt.Sprintf("kx.play(%q,%q,%s)", src, ep.ID, strconv.FormatFloat(start, 'f', 1, 64)))
			}
			a.flushDanmaku()
		})
	}()
}

// streamURL picks the best stream of the model and gives the proxy its
// key.
func (a *app) streamURL(f *future) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	pb := f.pb
	s, err := pb.Select("")
	if err != nil {
		return "", err
	}
	if s.HLS || s.Dash || pb.AudioURL != "" {
		return "", errors.New("暂不支持这种视频格式")
	}
	var key []byte
	if pb.Protected || s.Protected {
		k, err := sdk.ContentKey(pb)
		if err != nil {
			return "", err
		}
		if key, err = hex.DecodeString(k.Hex); err != nil {
			return "", err
		}
	}
	urls := []string{s.URL}
	if s.BackupURL != "" && s.BackupURL != s.URL {
		urls = append(urls, s.BackupURL)
	}
	return a.proxy.add(urls, key), nil
}

// step plays the episode d after the one playing, if there is one.
func (a *app) step(d int) {
	p := a.cur
	if p == nil {
		return
	}
	sp := a.series[p.seriesID]
	if sp == nil {
		return
	}
	if d > 0 {
		if id, n, ok := a.following(sp, p.number); ok {
			a.play(id, n, 0)
		}
		return
	}
	if _, ok := sp.episode(p.number + d); ok {
		a.play(p.seriesID, p.number+d, 0)
	}
}

// onPlayerEvent hears what the player page's video did.
func (a *app) onPlayerEvent(q url.Values) {
	p := a.cur
	if p == nil {
		return
	}
	sp := a.series[p.seriesID]
	// Controls belong to the player, not the episode that sent them:
	// between stop and the next play the page still names the old one.
	switch q.Get("type") {
	case "theater":
		if onPhone {
			return // the screen is the theater
		}
		a.toggleTheater()
		return
	case "pip":
		if onPhone {
			return // MyGo's small window is a desktop's
		}
		if a.web != nil {
			a.web.Page().TogglePictureInPicture()
		}
		return
	case "chrome":
		a.playerChrome = q.Get("on") != "0"
		return
	case "danmaku":
		a.setDanmaku(q.Get("on") != "0")
		return
	case "next":
		a.step(1)
		return
	case "prev":
		a.step(-1)
		return
	}
	ep, _ := sp.episode(p.number)
	if id := q.Get("ep"); id != "" && id != ep.ID {
		return // an episode that played before
	}
	a.playerEvent(q.Get("type"), atof(q.Get("t")), atof(q.Get("d")), "")
}

// playerEvent handles what the player reports: the page's events, and the
// native player's state and playhead, which come as the same kinds.
func (a *app) playerEvent(kind string, t, d float64, code string) {
	p := a.cur
	if p == nil {
		return
	}
	sp := a.series[p.seriesID]
	var ep model.Episode
	if sp != nil {
		ep, _ = sp.episode(p.number)
	}
	switch kind {
	case "playing":
		p.state = playPlaying
		p.paused = false
		// Resolve the next episode while this one plays.
		if next, ok := sp.episode(p.number + 1); ok && next.Playable {
			a.resolve(next)
		} else if ns, ok := sp.nextSeason(); ok {
			a.seriesPage(ns.SeriesID) // its episodes, ready to go on
		}
	case "waiting":
		if p.state == playPlaying {
			p.state = playBuffering
		}
		a.watchStall(p)
	case "progress":
		p.dur = d
		a.recordWatch(sp, p.number, t, d, false)
		p.at = int64(t * 1000)
		a.feedDanmaku()
	case "ended":
		// The player says the episode ended once. A second notice for one
		// the app already moved on from must not step again, or it would
		// skip the episode it just started.
		if p.state != playPlaying {
			return
		}
		a.recordWatch(sp, p.number, 0, 0, true)
		// On to the next episode, or the next season's first.
		if id, n, ok := a.following(sp, p.number); ok {
			a.play(id, n, 0)
		}
	case "error":
		p.state, p.msg = playFailed, "视频加载失败，请重试"
		if ep.ID != "" {
			delete(a.futures, ep.ID)
		}
	}
}

// watchStall reloads the episode when the picture never comes back, as
// after the app went to the background and the stream's connection went
// with it: no frame for twelve seconds starts the episode over from the
// playhead, with a stream resolved afresh. Only the latest wait counts, so
// a picture that comes back clears the one before it.
func (a *app) watchStall(p *playback) {
	win := a.win
	if win == nil {
		return
	}
	gen := p.gen
	a.stallTick++
	tick := a.stallTick
	time.AfterFunc(12*time.Second, func() {
		win.Update(func() {
			if a.cur == nil || a.cur.gen != gen || tick != a.stallTick {
				return
			}
			if a.cur.state != playBuffering || a.holdOn {
				return
			}
			if sp := a.series[a.cur.seriesID]; sp != nil {
				if ep, ok := sp.episode(a.cur.number); ok {
					delete(a.futures, ep.ID) // a URL that may have expired
				}
			}
			a.play(a.cur.seriesID, a.cur.number, float64(a.cur.at)/1000)
		})
	})
}

// playerState hears the native player's state. It runs on the main thread.
func (a *app) playerState(s playermod.State, code string) {
	switch s {
	case playermod.StatePlaying:
		a.watchVideoSize()
		a.playerChromeComeback() // the bar hides itself while it plays
		a.playerEvent("playing", 0, 0, "")
	case playermod.StateBuffering:
		a.playerEvent("waiting", 0, 0, "")
	case playermod.StateEnded:
		a.playerEvent("ended", 0, 0, "")
	case playermod.StateError:
		a.playerEvent("error", 0, 0, code)
	}
}

// watchVideoSize learns the episode's picture shape once it plays, so the
// phone picks 图一 for a portrait picture and 图二 for a landscape one. It
// also keeps the 弹幕 flight's aspect, so the view's builds never ask the
// player for the size — a crossing of the language bridge every frame.
func (a *app) watchVideoSize() {
	if a.player == nil {
		return
	}
	if vs := a.player.VideoSize(); vs.Width > 0 && vs.Height > 0 {
		a.videoLandscape = vs.Width > vs.Height
		if a.dm != nil {
			a.dm.aspect = float32(vs.Width) / float32(vs.Height)
		}
	}
}

// enterLandscapeFull turns a phone on its side for the 全屏 player, 图三.
// enterLandscapeFull turns the phone on its side for a landscape drama,
// whose picture then fills the screen (图三). A portrait drama already
// fills the screen, so there it only puts the buttons away.
func (a *app) enterLandscapeFull() {
	if !a.videoLandscape {
		a.playerChrome = false
		a.dmEditing = false
		return
	}
	a.landscapeFull = true
	a.playerChrome = true
	a.playerChromeComeback()
	mygo.SetOrientations(mygo.OrientationLandscapeLeft)
	a.remountVideo()
}

// remountVideo takes the picture's native view off for a frame, so the
// framework places it again at the window's new size after a turn: the
// held view keeps the size it had before, and a picture fitted to it is
// small and off centre.
func (a *app) remountVideo() {
	win := a.win
	if win == nil {
		return
	}
	time.AfterFunc(600*time.Millisecond, func() {
		win.Update(func() {
			a.videoRemount = true
		})
	})
}

// exitLandscapeFull puts the screen back upright, to 图一 or 图二.
func (a *app) exitLandscapeFull() {
	if !a.landscapeFull {
		return
	}
	a.landscapeFull = false
	a.playerChrome = true
	a.playerChromeComeback()
	mygo.SetOrientations(mygo.OrientationPortrait)
	a.remountVideo()
}

// playerTime hears the native player's playhead. It runs on the main
// thread: the 弹幕 clock follows every tick, and history records at the
// page's three-second cadence.
func (a *app) playerTime(pos, dur time.Duration) {
	p := a.cur
	if p == nil || p.state != playPlaying && p.state != playBuffering {
		return
	}
	p.dur = dur.Seconds()
	p.at = int64(pos / time.Millisecond)
	a.dm.clock(pos)
	// The playhead moves every tick, and the page's timecode and progress
	if a.win != nil {
		a.win.Invalidate()
	}
	if now := time.Now(); now.Sub(a.lastProgress) >= 3*time.Second {
		a.lastProgress = now
		a.playerEvent("progress", pos.Seconds(), dur.Seconds(), "")
	}
}

// atof is what a query value says, else zero.
func atof(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// pausePlayback stops at the playhead: the native player pauses, and the
// player page's video does.
func (a *app) pausePlayback() {
	if a.player != nil {
		a.player.Pause()
		return
	}
	a.evalJS("kx.pause()")
}

// resumePlayback goes on with the episode the app paused when it went to
// the background, from where it stopped.
func (a *app) resumePlayback() {
	p := a.cur
	if a.player == nil || p == nil || !p.loaded || a.holdOn {
		return
	}
	a.player.Play()
}

// holdPlayback holds playback for the player's tutorial, and lets it go
// after. The page's video pauses and resumes; the native player only
// misses its play, which the end of the hold gives it.
func (a *app) holdPlayback(on bool) {
	if a.player != nil {
		a.holdOn = on
		if !on && a.cur != nil && a.cur.loaded {
			a.player.Play()
		}
		return
	}
	if on {
		a.evalJS("kx.setHold(true)")
	}
}

// togglePlay plays or pauses the episode playing.
func (a *app) togglePlay() {
	if a.player != nil {
		if p := a.cur; p != nil && p.loaded && !a.holdOn {
			if p.state == playPlaying && !p.paused {
				a.player.Pause()
				p.paused = true // the player reports no pause of its own
				a.playerChrome = true // the bar comes back with a pause
			} else {
				a.player.Play()
				p.paused = false
				a.playerChromeComeback()
			}
		}
		return
	}
	a.evalJS("kx.toggle()")
}

// seekBy goes d seconds on from the playhead, back or forth.
func (a *app) seekBy(d float64) {
	if a.player != nil {
		if p := a.cur; p != nil && p.loaded {
			t := max(0, float64(p.at)/1000+d)
			a.player.Seek(time.Duration(t * float64(time.Second)))
			a.dm.seeked()
		}
		return
	}
	a.evalJS(fmt.Sprintf("kx.seek(%s)", strconv.FormatFloat(d, 'f', -1, 64)))
}

// playerSpeeds are the playback speeds, as the page's menu has them.
var playerSpeeds = [7]float64{0.5, 0.75, 1, 1.25, 1.5, 2, 3}

// nudgeRate steps the speed d places along the speeds table.
func (a *app) nudgeRate(d int) {
	if a.player == nil {
		a.evalJS(fmt.Sprintf("kx.nudgeRate(%d)", d))
		return
	}
	i := len(playerSpeeds) - 1
	for j, r := range playerSpeeds {
		if r == a.playerRate {
			i = j
			break
		}
	}
	if n := i + d; 0 <= n && n < len(playerSpeeds) {
		a.setRate(playerSpeeds[n])
	}
}

// setRate plays at r times the normal speed. The native player follows at
// once; the page's own menu sets its speed.
func (a *app) setRate(r float64) {
	a.playerRate = r
	if a.dm != nil {
		a.dm.rate = r // the flight glides at the playback's speed
	}
	if a.player != nil {
		a.player.SetRate(r)
	}
}

func (a *app) recordWatch(sp *seriesPage, n int, pos, dur float64, finished bool) {
	if sp == nil || sp.series == nil {
		return
	}
	w, _ := a.hist.get(sp.id)
	w.SeriesID, w.Title, w.Cover = sp.id, sp.series.Title, sp.series.CoverURL
	w.Episode, w.Position, w.Duration, w.Total = n, pos, dur, len(sp.episodes)
	if season := sp.seasonIndex(); season > 0 {
		w.Season = season
	}
	w.Finished = finished
	w.Want = false
	// The store writes snapshots in the background; copy the map so this
	// update cannot race that write or change a snapshot after the fact.
	eps := make(map[int]episodeWatch, len(w.Episodes)+1)
	for i, ew := range w.Episodes {
		eps[i] = ew
	}
	w.Episodes = eps
	e := w.Episodes[n]
	e.Position = pos
	if dur > 0 {
		e.Duration = dur // a start of the episode has no duration yet
	}
	if finished || dur > 0 && pos >= dur*0.98 {
		e.Finished = true
	}
	w.Episodes[n] = e
	// A whole series is done when the last episode ends, or when every
	// episode has been watched.
	if finished && w.Total > 0 && n >= w.Total {
		w.Done = true
	}
	watched := 0
	for _, ew := range w.Episodes {
		if ew.Finished {
			watched++
		}
	}
	if w.Total > 0 && watched >= w.Total {
		w.Done = true
	}
	a.hist.record(w)
}

// resumePoint is where to go on from: the series (this season or the
// next), the episode and the second.
func (a *app) resumePoint(sp *seriesPage) (id string, n int, start float64, label string) {
	w, ok := a.hist.get(sp.id)
	if !ok || w.Episode <= 0 || len(sp.episodes) == 0 {
		return sp.id, 1, 0, "播放第 1 集"
	}
	if w.Finished {
		if _, ok := sp.episode(w.Episode + 1); ok {
			return sp.id, w.Episode + 1, 0, fmt.Sprintf("播放第 %d 集", w.Episode+1)
		}
		if ns, ok := sp.nextSeason(); ok {
			return ns.SeriesID, 1, 0, "播放" + seasonLabel(ns)
		}
		return sp.id, 1, 0, "从头再看"
	}
	return sp.id, w.Episode, max(0, w.Position-3), fmt.Sprintf("继续看第 %d 集", w.Episode)
}

// ensureWebView makes the player's web view, once.
func (a *app) ensureWebView() {
	if a.web != nil || a.win == nil {
		return
	}
	web, err := a.win.NewWebView(mygo.WebViewOptions{
		URL:             a.proxy.playerURL(),
		BackgroundColor: "#000000",
		Page: mygo.PageOptions{
			Autoplay:         mygo.AutoplayAllow,
			PictureInPicture: true,
		},
	})
	if err != nil {
		if a.cur != nil {
			a.cur.state, a.cur.msg = playFailed, "无法创建播放器："+err.Error()
		}
		return
	}
	a.web = web
	a.js = make(chan string, 16)
	a.evalJS(fmt.Sprintf("kx.setDanmaku(%t,true)", a.danmakuOn))
	go func(page *mygo.Page, q <-chan string) {
		for js := range q {
			_, _ = page.Eval(js) // one at a time, in order
		}
	}(web.Page(), a.js)
	web.Page().OnDidFinishLoad(func() {
		a.update(func() {
			a.webReady = true
			pending := a.pendingJS
			a.pendingJS = nil
			for _, js := range pending {
				a.evalJS(js)
			}
		})
	})
}

// evalJS runs js in the player page once it has loaded.
func (a *app) evalJS(js string) {
	if a.web == nil {
		return
	}
	if !a.webReady {
		a.pendingJS = append(a.pendingJS, js)
		return
	}
	select {
	case a.js <- js:
	default: // the page is stuck; drop it rather than block the UI
	}
}
