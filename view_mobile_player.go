package main

import (
	"fmt"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var (
	railText = ui.RGBA(255, 255, 255, 0.92)
	phoneRed = ui.RGB(254, 45, 74)
	favGold  = ui.RGB(255, 193, 62)
)

// swipeStep is how far a finger must travel up or down, in DIP, before the
// swipe steps an episode instead of a tap; swipeSeek is the same sideways,
// where the drag moves the playhead through the episode.
const (
	swipeStep = 70
	swipeSeek = 40
)

// The phone plays by the picture's shape, as 红果's player does: a portrait
// drama fills the screen and puts the 互动 at its right edge (图一), a
// landscape one sits in a letterbox with 全屏观看 under it (图二), and the
// full-screen player turns the screen on its side, with the same 互动 (图三).
// All three draw the same buttons; only the frames differ.

// phoneVideo is what a phone's player draws over the picture. In the
// theater a tap brings the buttons, another puts them away; upright it
// plays or pauses the episode. Either way a swipe up or down steps an
// episode.
func (a *app) phoneVideo(c *ui.Context) {
	// The picture's tap layer holds the buttons as its children: a hit goes
	// to the button under the finger first, and to the layer itself only
	// where there is none, which shows or puts away the buttons.
	tap := ui.ButtonBase(c.Key("tap")).Fill().Label("播放画面")
	tap.Children(func() {
		switch {
		case a.landscapeFull:
			a.phoneFullscreenVideo(c)
		case a.videoLandscape:
			a.phoneLandscapeVideo(c)
		default:
			a.phonePortraitVideo(c)
		}
	})
	if a.phoneTap(c, tap) {
		a.phoneHint(c)
		return // the swipe stepped an episode or moved the playhead
	}
	a.phoneMenus(c)
	a.phoneSwipeHint(c)
	a.phoneHint(c)
}

// phoneTap hears the picture's press: in the theater a tap shows and puts
// away the buttons, upright it plays or pauses; a swipe up or down steps
// an episode, and a swipe sideways moves the playhead that far through the
// episode. It reports whether it did one of the two swipes, so the frame
// ends there.
func (a *app) phoneTap(c *ui.Context, b ui.Element) bool {
	pressed := b.Pressed()
	if pressed {
		if dx, dy, ok := b.Dragged(); ok {
			a.swipeDX += dx
			a.swipeDY += dy
			// The hint follows the finger, so the screen redraws as it moves.
			c.Invalidate()
		}
		a.swipeHeld = true
	}
	if a.swipeHeld && !pressed {
		a.swipeHeld = false
		dx, dy := a.swipeDX, a.swipeDY
		a.swipeDX, a.swipeDY = 0, 0
		if absF(dx) > absF(dy) {
			if absF(dx) >= swipeSeek {
				a.swipeSeekBy(dx, c)
				a.chromeTouch()
				c.Invalidate()
				return true
			}
		} else if dy <= -swipeStep || dy >= swipeStep {
			if dy < 0 {
				a.step(1) // up for the next episode
			} else {
				a.step(-1) // down for the one before
			}
			a.chromeTouch()
			c.Invalidate()
			return true
		}
	}
	if b.Clicked() {
		a.rateOpen, a.moreOpen = false, false
		if !a.landscapeFull {
			// 图一 and 图二 keep their rows up, so a tap on the picture has
			// nothing to show: it plays or pauses, as the drama player does.
			a.togglePlay()
			c.Invalidate()
			return false
		}
		a.playerChrome = !a.playerChrome
		if a.playerChrome {
			a.playerChromeComeback()
		} else {
			a.dmEditing = false
		}
		// Nothing else redraws while a quiet episode plays, so the tap
		// has to ask for its own frame.
		c.Invalidate()
	}
	return false
}

// absF is how far a distance is, whichever way the finger went.
func absF(v float32) float32 {
	return max(v, -v)
}

// swipeSeekTo is where a sideways drag of dx, over a picture w wide, puts
// the playhead: the whole width goes through the whole episode.
func (a *app) swipeSeekTo(dx, w float32) float64 {
	p := a.cur
	if p == nil || p.dur <= 0 || w <= 0 {
		return 0
	}
	to := float64(p.at)/1000 + float64(dx/w)*p.dur
	return min(max(to, 0), p.dur)
}

// swipeSeekBy moves the playhead where the finger dragged it, and starts
// the 弹幕 over from there.
func (a *app) swipeSeekBy(dx float32, c *ui.Context) {
	w, _ := c.Size()
	p := a.cur
	if p == nil || !p.loaded || a.player == nil {
		return
	}
	to := a.swipeSeekTo(dx, w)
	a.player.Seek(time.Duration(to * float64(time.Second)))
	p.at = int64(to * 1000)
	a.dm.seeked()
}

// phoneSwipeHint shows what the finger's swipe will do: the episode it
// steps to, or the playhead it moves, either following the finger.
func (a *app) phoneSwipeHint(c *ui.Context) {
	if !a.swipeHeld {
		return
	}
	dx, dy := a.swipeDX, a.swipeDY
	if absF(dx) < 12 && absF(dy) < 12 {
		return
	}
	w, h := c.Size()
	if absF(dx) > absF(dy) {
		p := a.cur
		if p == nil || p.dur <= 0 {
			return
		}
		to := a.swipeSeekTo(dx, w)
		back := to < float64(p.at)/1000
		ui.Box(c.Key("sw-seek")).Absolute().Left(w/2-90).Top(h/2-22).Width(180).Height(44).
			Center().Children(func() {
			ui.Row(c).Height(38).Shrink(0).Radius(12).Padding(0, 14).Gap(7).
				Background(ui.RGBA(16, 16, 18, 0.8)).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, ternIcon(back, iconRew, iconFwd)).Size(16, 16).TextColor(barBright)
				ui.Text(c, clock(int64(to*1000))).FontSize(15).Bold().SingleLine().TextColor(barBright)
				ui.Text(c, "/ "+clock(int64(p.dur*1000))).FontSize(12).SingleLine().TextColor(barText)
			})
		})
		return
	}
	label := "下一集"
	if dy > 0 {
		label = "上一集"
	}
	ui.Box(c.Key("sw-step")).Absolute().Left(w/2-70).Top(h/2-24+clampF(dy*0.18, -26, 26)).
		Width(140).Height(48).Center().Children(func() {
		ui.Column(c).Shrink(0).Radius(12).Padding(10, 16).Gap(2).
			Background(ui.RGBA(16, 16, 18, 0.8)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(14).Bold().SingleLine().TextColor(barBright)
			ui.Text(c, "松手切换").FontSize(11).SingleLine().TextColor(barText)
		})
	})
}

// clampF keeps a value between lo and hi.
func clampF(v, lo, hi float32) float32 {
	return min(max(v, lo), hi)
}

// phoneCenterPlay is the big play in the middle of the picture. 图一 and 图二
// show it while the video isn't playing; 图三 shows it with its buttons,
// playing or not, so a tap on the theater offers the pause.
func (a *app) phoneCenterPlay(c *ui.Context) bool {
	p := a.cur
	if p == nil || p.state == playFailed {
		return false
	}
	playing := p.state == playPlaying && !p.paused
	if playing && !a.landscapeFull {
		return false
	}
	w, h := c.Size()
	b := ui.ButtonBase(c.Key("center-play")).Absolute().Left(w/2 - 28).Top(h/2 - 28).
		Size(56, 56).Radius(28).
		Background(ui.RGBA(252, 252, 252, 0.92)).Center().Label("播放或暂停")
	b.Children(func() {
		ui.Icon(c, ternIcon(playing, iconPause, iconPlay)).Size(24, 24).TextColor(ui.RGBA(64, 64, 64, 0.92))
	})
	if b.Clicked() {
		a.togglePlay()
		a.chromeTouch()
		c.Invalidate()
		return true
	}
	return false
}

// phoneTopButton is a hit of the phone player's top row: an icon with a
// word beside it, or either alone.
func (a *app) phoneTopButton(c *ui.Context, key string, ic *ui.SVG, text, label string) bool {
	b := ui.ButtonBase(c.Key(key)).Height(32).Shrink(0).Radius(9).Padding(0, 8).
		AlignItems(ui.Center).Label(label)
	b.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
			if ic != nil {
				ui.Icon(c, ic).Size(15, 15).TextColor(barBright)
			}
			if text != "" {
				ui.Text(c, text).FontSize(13).SingleLine().TextColor(barBright)
			}
		})
	})
	return b.Clicked()
}

// phoneRail is one of the right edge's buttons: the icon with its count
// under it, when it has one. Its box is a finger's size, larger than the
// icon, so it takes the tap where the eye says it is.
func (a *app) phoneRail(c *ui.Context, key string, ic *ui.SVG, label, count string, color ui.Color) bool {
	b := ui.ButtonBase(c.Key(key)).Size(64, 50).Shrink(0).Center().Label(label)
	b.Children(func() {
		ui.Column(c).Gap(3).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, ic).Size(29, 29).TextColor(color)
			if count != "" {
				ui.Text(c, count).FontSize(11).SingleLine().TextColor(railText)
			}
		})
	})
	return b.Clicked()
}

// phoneTopPad is where a top row goes: under the system's top gesture
// area, which reaches down to the status bar's height even while the bar
// is hidden. The player takes that height back as content, so the row
// steps down by it to stay clear of the cutout.
func phoneTopPad(c *ui.Context) float32 {
	top, _, _, _ := c.SafeArea()
	return max(46, top+10)
}

// phoneTopRow is the row at the picture's top: back to the series, and the
// speed and ⋮ at the right. It reports a touch.
func (a *app) phoneTopRow(c *ui.Context, sp *seriesPage) bool {
	pad := float32(14)
	touched := false
	ui.Row(c.Key("p-top")).Absolute().Top(phoneTopPad(c)).Left(pad).Right(pad).Height(34).
		AlignItems(ui.Center).Children(func() {
		back := ui.ButtonBase(c.Key("p-back")).Shrink(0).Height(34).Radius(10).Padding(0, 4).
			AlignItems(ui.Center).Label("退出播放")
		back.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(2).Children(func() {
				ui.Icon(c, iconBack).Size(18, 18).TextColor(barBright)
				ui.Text(c, a.phoneEpLabel(sp)).FontSize(15).SingleLine().TextColor(barBright)
			})
		})
		if back.Clicked() {
			a.leavePlayer()
		}
		ui.Spacer(c)
		if a.phoneTopButton(c, "p-rate", iconClock, rateLabel(a.playerRate), "播放速度") {
			a.rateOpen, a.moreOpen = !a.rateOpen, false
			c.Invalidate()
			touched = true
		}
		// The ⋮ lines up with the rail below it, which sits half a body
		// nearer the edge.
		more := ui.ButtonBase(c.Key("p-more")).Size(32, 32).Shrink(0).Radius(10).Center().
			Label("更多").TextColor(barBright).Translate(15, 0)
		more.Children(func() {
			ui.Icon(c, iconMore).Size(20, 20)
		})
		if more.Clicked() {
			a.moreOpen, a.rateOpen = !a.moreOpen, false
			c.Invalidate()
			touched = true
		}
	})
	return touched
}

// phoneRailBlock is the row of 收藏, 评论, 点赞 and 分享 down the right
// edge, with their counts. right ends the box 64 points wide, with the
// icons centred in it: 图一 and 图二 pass a negative inset, which puts
// their icons half a body from the screen's edge. It reports a touch.
func (a *app) phoneRailBlock(c *ui.Context, sp *seriesPage, right, bottom float32) bool {
	touched := false
	rail := ui.Column(c.Key("p-rail")).Absolute().Right(right).Bottom(bottom).Width(64).Gap(6).
		AlignItems(ui.Center).Shrink(0)
	rail.Children(func() {
		if sp != nil && sp.series != nil {
			saved := a.favs.has(sp.id)
			k := card{ID: sp.id, Title: sp.series.Title, Cover: sp.series.CoverURL, Episodes: len(sp.episodes)}
			if a.phoneRail(c, "p-fav", ternIcon(saved, iconStarOn, iconStar), "收藏",
				heatN(sp.series.FollowCount), ternary(saved, favGold, railText)) {
				a.favs.toggle(favOf(k))
				c.Invalidate()
				touched = true
			}
		}
		ep := a.phoneEpisode(sp)
		if a.phoneRail(c, "p-cmt", iconComment, "评论", heatN(ep.CommentCount), railText) {
			a.flashHint("评论功能暂未开放")
			c.Invalidate()
			touched = true
		}
		if sp != nil && sp.series != nil {
			if a.phoneRail(c, "p-like", ternIcon(a.liked, iconHeartOn, iconHeart), "点赞",
				heatN(a.phoneLikes(sp)), ternary(a.liked, phoneRed, railText)) {
				a.liked = !a.liked
				c.Invalidate()
				touched = true
			}
		}
		if a.phoneRail(c, "p-share", iconShare, "分享", "", railText) {
			a.shareSeries(sp)
			touched = true
		}
	})
	return touched
}

// phoneInfoBlock is the bottom's left: the 弹 switch, the title, and what
// the series says about itself. It reports a touch.
func (a *app) phoneInfoBlock(c *ui.Context, sp *seriesPage) bool {
	touched := false
	pad := float32(14)
	info := ui.Column(c.Key("p-info")).Absolute().Left(pad).Bottom(52).Gap(9).Shrink(0).
		AlignItems(ui.Start)
	info.Children(func() {
		if a.phoneDmButton(c) {
			touched = true
		}
		if sp != nil && sp.series != nil {
			t := ui.ButtonBase(c.Key("p-title")).Shrink(0).AlignItems(ui.Center).Label(sp.series.Title)
			t.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(2).Children(func() {
					ui.Text(c, sp.series.Title).FontSize(16).Bold().SingleLine().MaxWidth(300).TextColor(barBright)
					ui.Icon(c, iconChevron).Size(12, 12).TextColor(ui.RGBA(255, 255, 255, 0.7))
				})
			})
			if t.Clicked() {
				a.leavePlayer()
			}
			if n := len(sp.series.Subtitles); n > 0 {
				ui.Text(c, sp.series.Subtitles[n-1]).FontSize(12).SingleLine().MaxWidth(280).TextColor(barText)
			}
			ui.Text(c, "内容由AI生成").FontSize(11).TextColor(barText.Alpha(0.7))
		}
	})
	return touched
}

// phoneDmButton is the 弹 switch at the picture's left edge: a thumb-sized
// pill that says whether 弹幕 fly. It reports a click.
func (a *app) phoneDmButton(c *ui.Context) bool {
	b := ui.ButtonBase(c.Key("p-dm")).Height(36).Shrink(0).Radius(18).Padding(0, 16).
		AlignItems(ui.Center).Label("弹幕")
	if a.danmakuOn {
		b.Background(ui.RGBA(255, 255, 255, 0.18))
	} else {
		b.Background(ui.RGBA(0, 0, 0, 0.34))
	}
	b.Children(func() {
		ui.Text(c, "弹").FontSize(15).Bold().SingleLine().
			TextColor(ternary(a.danmakuOn, barBright, barText))
	})
	if b.Clicked() {
		a.setDanmaku(!a.danmakuOn)
		c.Invalidate()
		return true
	}
	return false
}

// phoneProgress is the playhead as the thin red line along the bottom.
func (a *app) phoneProgress(c *ui.Context) {
	prog := ui.Box(c.Key("p-prog")).Absolute().Left(0).Right(0).Bottom(44).Height(3).
		Background(ui.RGBA(255, 255, 255, 0.22))
	prog.Children(func() {
		w, _ := c.Size()
		if f := a.phoneFraction(); f > 0 {
			ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(w * f).Background(phoneRed)
		}
	})
}

// phoneHandleRow is the very bottom: the 选集 handle, and the way into the
// full-screen player. It reports a touch.
func (a *app) phoneHandleRow(c *ui.Context, sp *seriesPage) bool {
	touched := false
	pad := float32(14)
	ui.Row(c.Key("p-handle")).Absolute().Left(0).Right(0).Bottom(0).Height(44).
		Padding(0, pad, 0, pad).AlignItems(ui.Center).Children(func() {
		h := ui.ButtonBase(c.Key("p-eps")).Shrink(0).Height(34).Radius(10).Padding(0, 6).
			AlignItems(ui.Center).Label("选集")
		h.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, "选集").FontSize(14).Bold().TextColor(barBright)
				if sp != nil && sp.series != nil {
					ui.Text(c, episodesTag(sp.series)).FontSize(12).TextColor(barText)
				}
				ui.Icon(c, iconChevUp).Size(14, 14).TextColor(barText)
			})
		})
		if h.Clicked() {
			a.panelOpen = true
			a.panelReveal = true
			c.Invalidate()
			touched = true
		}
		ui.Spacer(c)
		if a.playerIconButton(c, "p-info", ternIcon(a.playerChrome, iconEye, iconEyeOff), nil,
			"显示或隐藏信息") {
			a.playerChrome = !a.playerChrome
			if a.playerChrome {
				a.playerChromeComeback()
			}
			c.Invalidate()
			touched = true
		}
	})
	return touched
}

// phoneMenus draws the speed and ⋮ menus over whatever layout is showing.
func (a *app) phoneMenus(c *ui.Context) {
	if a.rateOpen {
		a.phoneRateMenu(c)
	}
	if a.moreOpen {
		a.phoneMoreMenu(c)
	}
}

// 图一 is the portrait drama: the screen is the picture, the 互动 sit at
// its right edge, and the 选集 pulls up from the bottom. The top row and
// the playhead stay up; the 互动 and the series' words come and go, with a
// tap on the picture or the eye in the corner.
func (a *app) phonePortraitVideo(c *ui.Context) {
	sp := a.phoneSeries()
	touched := a.phoneTopRow(c, sp)
	if a.phoneCenterPlay(c) {
		touched = true
	}
	if a.playerChrome {
		if a.phoneRailBlock(c, sp, -9, 118) {
			touched = true
		}
		if a.phoneInfoBlock(c, sp) {
			touched = true
		}
	}
	a.phoneProgress(c)
	a.phoneClock(c)
	if a.phoneHandleRow(c, sp) {
		touched = true
	}
	if touched {
		a.chromeTouch()
	}
}

// 图二 is a landscape drama held upright: the same buttons as 图一, with
// 全屏观看 under the picture, level in the middle.
func (a *app) phoneLandscapeVideo(c *ui.Context) {
	sp := a.phoneSeries()
	touched := a.phoneTopRow(c, sp)
	if a.phoneCenterPlay(c) {
		touched = true
	}
	if a.playerChrome {
		if a.phoneRailBlock(c, sp, -9, 118) {
			touched = true
		}
		if a.phoneInfoBlock(c, sp) {
			touched = true
		}
	}
	a.phoneProgress(c)
	a.phoneClock(c)

	// 全屏观看, just under the picture and level in the middle.
	w, h := c.Size()
	top, vh := a.phoneVideoRect(w, h)
	ui.Row(c.Key("l-fullwrap")).Absolute().Left(0).Right(0).Top(top+vh+14).Height(34).
		Justify(ui.Center).Children(func() {
		fs := ui.ButtonBase(c.Key("l-full")).Shrink(0).Height(32).Radius(16).Padding(0, 14).
			AlignItems(ui.Center).Background(ui.RGBA(0, 0, 0, 0.52)).Label("全屏观看")
		fs.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
				ui.Icon(c, iconFullscr).Size(14, 14).TextColor(barBright)
				ui.Text(c, "全屏观看").FontSize(12).SingleLine().TextColor(barBright)
			})
		})
		if fs.Clicked() {
			a.enterLandscapeFull()
			c.Invalidate()
			touched = true
		}
	})

	if a.phoneHandleRow(c, sp) {
		touched = true
	}
	if touched {
		a.chromeTouch()
	}
}

// 图三 is the full-screen player: the screen lies on its side, the picture
// fills it, and the same 互动 float at its right edge while the 弹幕 box and
// playhead sit along the bottom.
func (a *app) phoneFullscreenVideo(c *ui.Context) {
	if !a.playerChrome {
		return
	}
	pad := float32(16)
	// The picture fills the screen on every side but the bottom, so the
	// rows step in past the cutout themselves.
	_, safeR, _, safeL := c.SafeArea()
	padL, padR := pad+safeL, pad+safeR
	sp := a.phoneSeries()
	touched := false
	// The system's bar goes with the picture, so the layout draws its row
	// at the screen's top edge: signal, hour, battery.
	a.phoneStatusBar(c, padL, padR)
	topPad := float32(26)

	// The top: back to the upright screen, and ⋮ at the right.
	ui.Row(c.Key("f-top")).Absolute().Top(topPad).Left(padL).Right(padR).Height(34).
		AlignItems(ui.Center).Children(func() {
		back := ui.ButtonBase(c.Key("f-back")).Shrink(0).Height(34).Radius(10).Padding(0, 4).
			AlignItems(ui.Center).Label("退出全屏")
		back.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
				ui.Icon(c, iconBack).Size(18, 18).TextColor(barBright)
				ui.Text(c, a.phoneTitle(sp)).FontSize(14).SingleLine().TextColor(barBright)
			})
		})
		if back.Clicked() {
			a.exitLandscapeFull()
			c.Invalidate()
		}
		ui.Spacer(c)
		if a.playerIconButton(c, "p-more", iconMore, nil, "更多") {
			a.moreOpen, a.rateOpen = !a.moreOpen, false
			c.Invalidate()
			touched = true
		}
	})

	if a.phoneCenterPlay(c) {
		touched = true
	}
	if a.phoneRailBlock(c, sp, 8+safeR, 76) {
		touched = true
	}

	// The bottom: the 弹 switch, the timecode and the speed.
	ui.Row(c.Key("f-bar")).Absolute().Left(padL).Right(padR).Bottom(34).Height(36).
		AlignItems(ui.Center).Gap(10).Children(func() {
		if a.phoneDmButton(c) {
			touched = true
		}
		ui.Spacer(c)
		ui.Text(c, a.phoneClockText()).FontSize(11).SingleLine().
			TextColor(barText).FontFeatures("tnum")
		if a.playerTextButton(c, "f-rate", rateLabel(a.playerRate), "播放速度", false) {
			a.rateOpen = !a.rateOpen
			c.Invalidate()
			touched = true
		}
	})

	// The playhead, as the finger's line.
	ui.Box(c.Key("f-seek")).Absolute().Left(padL).Right(padR).Bottom(4).Height(24).
		Children(func() {
			if a.seekSlider(c) {
				touched = true
			}
		})

	if touched {
		a.chromeTouch()
	}
}

// phoneRateMenu is the speeds under the top row's 🕐, as the bar's menu is.
// A tap anywhere else puts it away.
func (a *app) phoneRateMenu(c *ui.Context) {
	ui.Row(c.Key("rates-back")).Absolute().Fill().Children(func() {
		if ui.ButtonBase(c).Fill().Label("收起速度").Clicked() {
			a.rateOpen = false
			c.Invalidate()
		}
	})
	menu := ui.Column(c.Key("rates")).Absolute().Top(48).Right(14).Shrink(0).Width(84).Gap(2).
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
				c.Invalidate()
			}
		}
	})
}

// phoneMoreMenu is what the ⋮ holds: the 弹幕's way in. A tap anywhere
// else puts it away.
func (a *app) phoneMoreMenu(c *ui.Context) {
	ui.Row(c.Key("more-back")).Absolute().Fill().Children(func() {
		if ui.ButtonBase(c).Fill().Label("收起菜单").Clicked() {
			a.moreOpen = false
			c.Invalidate()
		}
	})
	menu := ui.Column(c.Key("more")).Absolute().Top(48).Right(14).Shrink(0).Width(112).Gap(2).
		Padding(6).Radius(12).Background(ui.RGBA(24, 24, 26, 0.92)).
		Shadow(0, 10, 34, 0, ui.RGBA(0, 0, 0, 0.38)).Role(ui.RoleMenu).Label("更多")
	menu.Children(func() {
		if a.moreRow(c, "more-full", "全屏播放") {
			a.moreOpen = false
			a.enterLandscapeFull()
			c.Invalidate()
		}
		if a.moreRow(c, "more-eps", "选集") {
			a.moreOpen = false
			a.panelOpen = true
			a.panelReveal = true
			c.Invalidate()
		}
		dm := ui.ButtonBase(c.Key("more-dm")).Height(34).Shrink(0).Radius(8).Padding(0, 10).
			AlignItems(ui.Center).Label("弹幕")
		dm.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Text(c, "弹幕").FontSize(13).TextColor(barBright)
				dmState := "关"
				if a.danmakuOn {
					dmState = "开"
				}
				ui.Text(c, dmState).FontSize(13).TextColor(barText)
			})
		})
		if dm.Clicked() {
			a.setDanmaku(!a.danmakuOn)
			a.moreOpen = false
			c.Invalidate()
		}
	})
}

// moreRow is a word in the ⋮ menu.
func (a *app) moreRow(c *ui.Context, key, text string) bool {
	b := ui.ButtonBase(c.Key(key)).Height(34).Shrink(0).Radius(8).Padding(0, 10).
		AlignItems(ui.Center).Label(text)
	b.Children(func() {
		ui.Text(c, text).FontSize(13).TextColor(barBright)
	})
	return b.Clicked()
}

// phoneHint is a word of feedback at the picture's top, for a tap that
// can't do anything yet.
func (a *app) phoneHint(c *ui.Context) {
	if a.hint == "" {
		return
	}
	ui.Box(c).Absolute().Top(52).Left(0).Right(0).Height(32).Center().Children(func() {
		ui.Box(c).Height(30).Radius(15).Padding(0, 14).AlignItems(ui.Center).
			Background(ui.RGBA(20, 20, 22, 0.85)).Children(func() {
			ui.Text(c, a.hint).FontSize(12).SingleLine().TextColor(barBright)
		})
	})
}

// phoneEpisodesSheet is the phone's 选集: the episodes pull up from the
// bottom edge, as 图一's handle asks them. It reports whether the sheet
// keeps its place open.
func (a *app) phoneEpisodesSheet(c *ui.Context, sp *seriesPage) bool {
	ui.Overlay(c, func() {
		back := ui.Row(c.Key("sheet-back")).Absolute().Fill().Children(func() {
			if ui.ButtonBase(c).Fill().Label("收起选集").Clicked() {
				a.closePanel()
				c.Invalidate()
			}
		})
		_, h := c.Size()
		sheet := ui.Column(c.Key("sheet")).Absolute().Left(0).Right(0).Bottom(0).
			Height(min(h*0.62, 480)).Background(playerPanelColor).Radius(16).
			Shadow(0, -12, 40, 0, ui.RGBA(0, 0, 0, 0.5)).Role(ui.RoleDialog).Label("选集")
		sheet.Children(func() {
			ui.Row(c).Height(46).Shrink(0).Padding(0, 16).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "选集").FontSize(16).Bold()
				ui.Spacer(c)
				if a.closePanelButton(c) {
					a.closePanel()
					c.Invalidate()
				}
			})
			ui.Divider(c).Background(ui.RGBA(255, 255, 255, 0.06))
			reveal := a.panelReveal
			a.panelReveal = false
			a.playerPanelBody(c, sp, true, reveal)
		})
		if sheet.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
			a.closePanel()
			c.Invalidate()
		}
	})
	return true
}

// phoneSeries is the series playing, without asking the server for it.
func (a *app) phoneSeries() *seriesPage {
	if a.cur == nil {
		return nil
	}
	return a.series[a.cur.seriesID]
}

// phoneEpisode is the episode playing, empty before its series has loaded.
func (a *app) phoneEpisode(sp *seriesPage) model.Episode {
	var zero model.Episode
	if sp == nil || a.cur == nil {
		return zero
	}
	ep, _ := sp.episode(a.cur.number)
	return ep
}

// phoneEpLabel is the episode playing as the top's back button says it:
// 第 N 集.
func (a *app) phoneEpLabel(sp *seriesPage) string {
	if a.cur == nil {
		return ""
	}
	return fmt.Sprintf("第 %d 集", a.cur.number)
}

// phoneTitle is the episode as 图三's back button says it: the series'
// title, and the episode with it.
func (a *app) phoneTitle(sp *seriesPage) string {
	if a.cur == nil {
		return ""
	}
	title := ""
	if sp != nil && sp.series != nil {
		title = sp.series.Title
	}
	if title != "" {
		return title + " " + a.phoneEpLabel(sp)
	}
	return a.phoneEpLabel(sp)
}

// phoneLikes is the series' 点赞 as the rail shows it: the count, plus the
// viewer's own while it lasts.
func (a *app) phoneLikes(sp *seriesPage) int64 {
	var n int64
	if sp != nil && sp.series != nil {
		n = sp.series.DiggCount
	}
	if a.liked {
		n++
	}
	return n
}

// phoneAt is the playhead, in ms, as the timecode says it.
func (a *app) phoneAt() int64 {
	if a.cur == nil {
		return 0
	}
	return a.cur.at
}

// phoneClockText is where the playhead is, and how long the episode runs:
// "0:36 / 2:10", as the players say it.
func (a *app) phoneClockText() string {
	p := a.cur
	if p == nil {
		return ""
	}
	return clock(p.at) + " / " + clock(int64(max(p.dur, 0)*1000))
}

// phoneStatusBar draws the row the system's bar would be, while the
// picture fills the screen: the signal at the left, the hour in the middle,
// and the battery's level and icon at the right.
func (a *app) phoneStatusBar(c *ui.Context, padL, padR float32) {
	b := a.battery()
	ui.Row(c.Key("f-status")).Absolute().Top(2).Left(padL).Right(padR).Height(20).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		// The signal: a phone's bars, all standing.
		ui.Row(c).Gap(2).Height(10).AlignItems(ui.End).Children(func() {
			for _, h := range []float32{3, 5, 7, 9} {
				ui.Box(c).Width(3).Height(h).Radius(1).Background(barBright)
			}
		})
		ui.Text(c, time.Now().Format("15:04")).FontSize(12).SingleLine().TextColor(barBright)
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("%d%%", int(min(max(b.Level, 0), 1)*100))).
				FontSize(11).SingleLine().TextColor(barBright)
			ui.Row(c).Gap(1).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(22).Height(11).Radius(3).Border(1, barBright).Padding(1).
					Children(func() {
						ui.Box(c).Width(20 * float32(min(max(b.Level, 0), 1))).
							Height(9).Radius(2).Background(barBright)
					})
				ui.Box(c).Width(2).Height(4).Radius(1).Background(barBright)
			})
		})
	})
}

// battery is the phone's battery, read again now and then rather than on
// every frame.
func (a *app) battery() mygo.BatteryStatus {
	if !onPhone || a.win == nil {
		return mygo.BatteryStatus{Level: 100}
	}
	if time.Since(a.battAt) >= 15*time.Second {
		a.battAt = time.Now()
		a.batt = mygo.Power.Battery()
	}
	return a.batt
}

// phoneClock draws the timecode at the bottom's right, over the progress
// line: 图一 and 图二 have no other place for it. A dark pill keeps it
// legible over a bright picture.
func (a *app) phoneClock(c *ui.Context) {
	if a.cur == nil {
		return
	}
	ui.Box(c.Key("p-clock")).Absolute().Right(14).Bottom(48).Children(func() {
		ui.Box(c).Height(20).Radius(10).Padding(0, 8).Center().
			Background(ui.RGBA(16, 16, 18, 0.55)).Children(func() {
			ui.Text(c, a.phoneClockText()).FontSize(11).SingleLine().
				TextColor(barBright).FontFeatures("tnum")
		})
	})
}

// phoneFraction is how far the playhead is into the episode, 0 to 1.
func (a *app) phoneFraction() float32 {
	p := a.cur
	if p == nil || p.dur <= 0 {
		return 0
	}
	f := float32(max(0, float64(p.at)/1000)) / float32(p.dur)
	return min(f, 1)
}

// phoneVideoRect is the picture's rect in the view, as object-fit: contain
// lays it out centered in the screen: 图二's letterbox.
func (a *app) phoneVideoRect(w, h float32) (top, vh float32) {
	ar := float32(16) / float32(9)
	if a.player != nil {
		if vs := a.player.VideoSize(); vs.Width > 0 && vs.Height > 0 && vs.Width >= vs.Height {
			ar = float32(vs.Width) / float32(vs.Height)
		}
	}
	vh = min(w/ar, h)
	return (h - vh) / 2, vh
}

// heatN is a count as the rail says it, blank while it is unknown.
func heatN(n int64) string {
	if n <= 0 {
		return ""
	}
	return heat(n)
}

// ternIcon picks an icon.
func ternIcon(on bool, a, b *ui.SVG) *ui.SVG {
	if on {
		return a
	}
	return b
}

// episodesTag is the 选集 handle's word for the series' length.
func episodesTag(s *model.Series) string {
	tag := "已完结"
	if s != nil && s.Status == model.SeriesUpdating {
		tag = "更新中"
	}
	if s != nil && s.EpisodeCount > 0 {
		return tag + " · 全" + fmt.Sprint(s.EpisodeCount) + "集"
	}
	return tag
}

// shareSeries hands the series to the system's share sheet, from its own
// goroutine: Share waits for the sheet to show.
func (a *app) shareSeries(sp *seriesPage) {
	if sp == nil || sp.series == nil {
		return
	}
	text := "我在开心果看《" + sp.series.Title + "》"
	go func() { _ = mygo.Share(mygo.ShareOptions{Text: text}) }()
}

// flashHint says something for a moment at the picture's top.
func (a *app) flashHint(msg string) {
	a.hint = msg
	if win := a.win; win != nil {
		time.AfterFunc(1800*time.Millisecond, func() {
			win.Update(func() { a.hint = "" })
		})
	}
}
