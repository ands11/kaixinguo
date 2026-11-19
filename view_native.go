package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

var (
	barText   = ui.RGBA(255, 255, 255, 0.72)
	barBright = ui.RGBA(255, 255, 255, 0.92)
)

// nativeVideo is what the phone's player shows instead of the player's
// page: the platform's view of the episode, with the 弹幕 and the bar the
// app draws itself over it. A tap on the picture brings the bar and the
// top buttons, and another puts them away.
func (a *app) nativeVideo(c *ui.Context) {
	// A frame with nothing of the picture, after the screen turned: the
	// framework takes the view down, and places it again at the window's
	// new size on the next one.
	if a.videoRemount {
		a.videoRemount = false
		c.Invalidate()
		return
	}
	ui.NativeView(c, a.player).Fill().Children(func() {
		a.dmView(c)
		// A phone plays by the picture's shape: 图一, 图二 or 图三.
		if onPhone {
			a.phoneVideo(c)
			return
		}
		// The tap layer takes a click anywhere on the picture; the bar and
		// the overlay sit over it, and keep their own clicks.
		if ui.ButtonBase(c.Key("tap")).Fill().Label("播放画面").Clicked() {
			a.playerChrome = !a.playerChrome
			if a.playerChrome {
				a.playerChromeComeback()
			}
		}
		if a.playerChrome {
			a.playerBar(c)
		}
	})
}

// playerChromeComeback gives the buttons five seconds without a touch
// before they put themselves away, as the page's bar does. Typing in the
// 弹幕 box, or a menu or the 选集 being open, keeps them.
func (a *app) playerChromeComeback() {
	a.chromeGen++
	gen := a.chromeGen
	win := a.win
	if win == nil {
		return
	}
	time.AfterFunc(5*time.Second, func() {
		win.Update(func() {
			if gen != a.chromeGen || !a.onPlayer || !a.playerChrome || a.cur == nil {
				return
			}
			// 图一 and 图二 keep their 互动 and words up: only the theater
			// puts its buttons away on its own.
			if onPhone && !a.landscapeFull {
				return
			}
			if a.cur.state != playPlaying || a.cur.paused {
				return // paused: the buttons stay
			}
			if a.dmEditing || a.rateOpen || a.moreOpen || a.panelOpen {
				a.playerChromeComeback() // look again once they close
				return
			}
			a.playerChrome = false
		})
	})
}

// chromeTouch keeps the buttons up after what the finger did.
func (a *app) chromeTouch() {
	if a.playerChrome {
		a.playerChromeComeback()
	}
}

// playerBar is the bar at the picture's bottom edge: the transport, and
// the playhead with the speed and 弹幕.
func (a *app) playerBar(c *ui.Context) {
	var at int64
	var dur float64
	if p := a.cur; p != nil {
		at, dur = p.at, p.dur
	}
	bar := ui.Column(c.Key("bar")).Absolute().Bottom(16).Left(16).Right(16).Gap(2).
		Padding(12, 14, 8, 14).Radius(16).Background(ui.RGBA(24, 24, 26, 0.42)).
		Shadow(0, 8, 28, 0, ui.RGBA(0, 0, 0, 0.26)).Label("播放控制")
	bar.Children(func() {
		// The transport, as the page's: back an episode, ten seconds,
		// play, ten seconds, on an episode.
		ui.Row(c).Height(40).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Gap(18).Children(func() {
			if a.playerIconButton(c, "prev", iconSkipBack, nil, "上一集") {
				a.step(-1)
			}
			if a.playerIconButton(c, "rew", iconRew, nil, "快退 10 秒") {
				a.seekBy(-10)
			}
			if a.playerIconButton(c, "play", iconPlay, iconPause, "播放或暂停") {
				a.togglePlay()
			}
			if a.playerIconButton(c, "fwd", iconFwd, nil, "快进 10 秒") {
				a.seekBy(10)
			}
			if a.playerIconButton(c, "next", iconSkipFwd, nil, "下一集") {
				a.step(1)
			}
		})
		// The playhead, the speed and 弹幕 share the second row.
		ui.Row(c).Height(28).Shrink(0).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, clock(at)).FontSize(12).SingleLine().TextColor(barText)
			a.seekSlider(c)
			ui.Text(c, clock(int64(max(dur, 0)*1000))).FontSize(12).SingleLine().TextColor(barText)
			if a.playerTextButton(c, "rate", rateLabel(a.playerRate), "播放速度", a.rateOpen) {
				a.rateOpen = !a.rateOpen
			}
			if a.playerTextButton(c, "dm", "弹", "弹幕", a.danmakuOn) {
				a.setDanmaku(!a.danmakuOn)
			}
		})
	})
	if a.rateOpen {
		a.rateMenu(c)
	}
}

// rateMenu is the speeds above the bar, as the page's menu is. A tap
// anywhere else puts it away.
func (a *app) rateMenu(c *ui.Context) {
	ui.Row(c.Key("rates-back")).Absolute().Fill().Children(func() {
		if ui.ButtonBase(c).Fill().Label("收起速度").Clicked() {
			a.rateOpen = false
		}
	})
	menu := ui.Column(c.Key("rates")).Absolute().Bottom(96).Left(16).Shrink(0).Width(76).Gap(2).
		Padding(6).Radius(12).Background(ui.RGBA(24, 24, 26, 0.92)).
		Shadow(0, 10, 34, 0, ui.RGBA(0, 0, 0, 0.38)).Role(ui.RoleMenu).Label("播放速度")
	menu.Children(func() {
		for _, r := range playerSpeeds {
			on := r == a.playerRate
			b := ui.ButtonBase(c.Key("rate-" + rateLabel(r))).Height(28).Shrink(0).Radius(8).
				Padding(0, 8).AlignItems(ui.Center).Label("速度 " + rateLabel(r))
			if on {
				b.Background(ui.RGBA(255, 255, 255, 0.14))
			}
			b.Children(func() {
				ui.Text(c, rateLabel(r)).FontSize(12).SingleLine().
					TextColor(ternary(on, barBright, barText))
			})
			if b.Clicked() {
				a.setRate(r)
				a.rateOpen = false
			}
		}
	})
}

// seekSlider is the playhead the finger drags. While it drags, the
// player's playhead leaves it alone; when it lets go, the player goes
// there and the 弹幕 start over. It reports whether the finger moved it.
func (a *app) seekSlider(c *ui.Context) bool {
	p := a.cur
	if p == nil {
		return false
	}
	if !a.seeking {
		a.seekPos = float64(p.at) / 1000
	}
	changed := ui.Slider(c, &a.seekPos, 0, max(p.dur, 1)).Grow(1).Changed()
	if changed {
		a.seeking = true
	} else if a.seeking {
		a.seeking = false
		if p.loaded {
			a.player.Seek(time.Duration(max(0, a.seekPos) * float64(time.Second)))
			a.dm.seeked()
		}
	}
	return changed
}

// playerIconButton is a hit of the bar with an icon in it, bright while
// pressed. alt is the icon that shows when the video plays.
func (a *app) playerIconButton(c *ui.Context, key string, icon, alt *ui.SVG, label string) bool {
	b := ui.ButtonBase(c.Key(key)).Size(32, 32).Shrink(0).Radius(10).Center().
		Label(label).TextColor(barBright)
	if alt != nil && a.cur != nil && a.cur.state == playPlaying {
		icon = alt
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(20, 20)
	})
	return b.Clicked()
}

// playerTextButton is a hit of the bar with a word in it; on marks it
// selected, as the 弹幕 button is while 弹幕 show.
func (a *app) playerTextButton(c *ui.Context, key, text, label string, on bool) bool {
	b := ui.ButtonBase(c.Key(key)).Height(28).Shrink(0).Radius(8).Padding(0, 6).
		Center().Label(label).TextColor(barText)
	if on {
		b.TextColor(barBright)
	}
	b.Children(func() {
		ui.Text(c, text).FontSize(12).SingleLine().TextColor(ternary(on, barBright, barText))
	})
	return b.Clicked()
}

// rateLabel is a speed as the bar says it: 1 as 1.0×.
func rateLabel(r float64) string {
	s := fmt.Sprintf("%.2f", r)
	if len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	return s + "×"
}

// clock is seconds as m:ss, as the page's time is.
func clock(t int64) string {
	s := int(max(t, 0) / 1000)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// ternary picks a color.
func ternary(on bool, a, b ui.Color) ui.Color {
	if on {
		return a
	}
	return b
}
