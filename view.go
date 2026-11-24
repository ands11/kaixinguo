package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

const (
	gridPadX   = 32  // the grid's margin, outside its cells' own
	cellBorder = 2   // the clear border GridView keeps around each cell
	cellPadX   = 12  // room on each side of a poster in its cell
	posterMinW = 176 // the narrowest a poster gets

	// The room in a card above its poster, between the poster and the
	// lines about it, and under those lines, which keeps the rows apart.
	cardPadTop    = 6
	cardTitleGap  = 12
	cardPadBottom = 18
	// coverMute is how much of a poster's color gives way to gray until
	// the pointer comes: the covers are loud, and a wall of them more so.
	// A finger has no pointer to come, so phones show them in color.
	coverMute = 0.4

	// A phone's screen is narrow, and its fingers are not a pointer: under
	// compactWidth the app shows tabs instead of the sidebar, with a
	// tighter grid that fits three posters across.
	compactWidth     = 640
	compactHeaderH   = 48 // a page's title row
	compactRightH    = 40 // the controls a page puts under its title
	compactPadX      = 12
	compactCellPadX  = 6
	compactPosterMin = 96
)

// cardMetrics is how the grid of a page is laid out: roomier on a computer,
// where the sidebar takes a side, and tighter on a phone.
type cardMetrics struct {
	pad        float32 // the page's margin, outside the cells' own
	posterMin  float32 // the narrowest a poster gets
	cellPad    float32 // room on each side of a poster in its cell
	padTop     float32 // the room in a card above its poster
	titleGap   float32 // between the poster and the lines about it
	padBottom  float32 // under those lines, which keeps the rows apart
	lines      float32 // the height of a card's title and the line under it
	titleSize  float32
	subSize    float32
	bodyHeight float32 // the height of a page's title row
}

func (m cardMetrics) cellMin() float32 { return m.posterMin + 2*(cellBorder+m.cellPad) }
func (m cardMetrics) inset() float32   { return m.pad + cellBorder + m.cellPad }
func (m cardMetrics) chrome() float32 {
	return 2*cellBorder + m.padTop + m.titleGap + m.lines + m.padBottom
}

// metrics is the layout the pages are built with now.
func (a *app) metrics() cardMetrics {
	if a.compact {
		return cardMetrics{
			pad: compactPadX, posterMin: compactPosterMin, cellPad: compactCellPadX,
			padTop: 4, titleGap: 10, padBottom: 14, lines: 36,
			titleSize: 13, subSize: 11.5, bodyHeight: compactHeaderH,
		}
	}
	return cardMetrics{
		pad: gridPadX, posterMin: posterMinW, cellPad: cellPadX,
		padTop: cardPadTop, titleGap: cardTitleGap, padBottom: cardPadBottom, lines: cardLines,
		titleSize: 14, subSize: 12, bodyHeight: titleBarHeight,
	}
}

// gridMetrics is how a grid of cards fits the page: how many columns, as
// GridView counts them, and how high each row, so that every poster is 3:4.
func (a *app) gridMetrics(c *ui.Context) (cols int, rowH float32) {
	w, _ := c.Size()
	m := a.metrics()
	inner := w - 2*m.pad
	if !a.compact {
		inner -= sidebarWidth*a.sideShown + 1
	}
	gap := c.Theme().Space(2)
	cols = max(1, int((inner+gap)/(m.cellMin()+gap)))
	cellW := (inner - float32(cols-1)*gap) / float32(cols)
	posterW := cellW - (m.cellMin() - m.posterMin)
	return cols, posterW*4/3 + m.chrome()
}

// cardLines is the height of a card's title and the line under it.
const cardLines = 41

func (a *app) view(c *ui.Context) {
	w, _ := c.Size()
	a.compact = onPhone || w < compactWidth
	t := c.Theme()
	vibrant := c.Vibrancy() && !a.compact
	if vibrant {
		c.Root().Background(ui.Transparent)
	}
	path := a.router.Path()
	onPlayer := strings.HasPrefix(path, "/play/")
	// The framework keeps the system bars' insets as the root's padding, and
	// fills that band with the root's background rather than the page's. The
	// player takes the band back as content — the picture runs into it on
	// every edge but the bottom, which keeps the gesture bar — and what shows
	// around it is the player's own ground, never a strip of the window.
	// The bars the layouts draw keep clear of the cutout on their own, through
	// SafeArea.
	if onPlayer && a.compact {
		c.Root().Background(playerBg)
		top, right, _, left := c.SafeArea()
		c.Root().Padding(-top, -right, 0, -left)
	}
	if a.onPlayer && !onPlayer {
		a.pausePlayback() // left the player
		a.panelOpen = false
		a.panelSuppressed = false
		a.panelJustClosed = false
		a.panelReveal = false
		a.panelPinned = false
		a.playerSet = false
		a.playerChrome = true
		a.playerTopHovered = false
		a.theaterShown = false
		if a.fullscreen && a.theaterFullscreen {
			a.setFullScreen(false) // only the full screen the theater entered
		}
		a.theaterFullscreen = false
		a.playerAppearance(false)
	}
	if !a.onPlayer && onPlayer {
		a.playerAppearance(true)
	}
	a.onPlayer = onPlayer
	// The bars show on every page but the full screen, where the picture
	// fills the screen and the player's own layout draws the row.
	a.playerBars(onPlayer)
	switch path {
	case "/discover", "/rank", "/new", "/search", "/favorites", "/mine":
		a.section = path
	}

	if a.compact {
		a.compactView(c)
	} else {
		row := ui.Row(c).Fill().AlignItems(ui.Stretch)
		a.sideShown = row.Animate("sidebar", b2f(!onPlayer), 280*time.Millisecond)
		row.Children(func() {
			// The sidebar slides away while the player shows.
			w := sidebarWidth * a.sideShown
			ui.Row(c).Width(w).Shrink(0).AlignItems(ui.Stretch).Clip().Children(func() {
				side := ui.Column(c).Width(sidebarWidth).Shrink(0).Margin(0, 0, 0, w-sidebarWidth)
				if !vibrant {
					side.Background(t.Surface)
				}
				side.Children(func() { a.sidebar(c, vibrant) })
			})
			if w > 0.5 {
				ui.Box(c).Width(1).Background(hairline(t))
			}
			a.router.View(c, func(r *ui.Route) {
				switch {
				case r.Match("/discover"):
					r.Title("推荐")
					a.gridPage(c, "推荐", a.home, nil, nil, 0, "暂时没有推荐")
				case r.Match("/rank"):
					r.Title("排行榜")
					a.rankPage(c)
				case r.Match("/new"):
					r.Title("新剧上线")
					a.newPage(c)
				case r.Match("/search"):
					r.Title("搜索")
					a.searchPage(c, r.Query("q"))
				case r.Match("/favorites"):
					r.Title("收藏")
					a.favoritesPage(c)
				case r.Match("/series/{id}"):
					r.Title("详情")
					a.seriesView(c, r.Param("id"))
				case r.Match("/play/{id}"):
					r.Title("播放")
					a.playerView(c, r.Param("id"))
				}
			}).Grow(1).MinWidth(0).Background(t.Background)
		})
	}

	if !a.tutorialOn && !(onPlayer && a.playerTutorialOn) {
		c.OnShortcut(ui.Cmd, ui.KeyF, func() {
			if onPlayer {
				a.leavePlayer()
			}
			a.searchField.Focus(c)
		})
	}
	if onPlayer && !a.tutorialOn && !a.playerTutorialOn {
		// The player's keys are the window's too, so that they go on
		// working when the focus left its page, as it does when the
		// theater's episodes close.
		a.playerShortcuts(c)
	}
	if a.tutorialOn {
		a.tutorial(c)
	}
	a.settingsDialog(c)
}

// compactView is the app on a phone's screen: a page, with the sections as
// tabs under it. A series and the player open over the page, without tabs.
func (a *app) compactView(c *ui.Context) {
	t := c.Theme()
	path := a.router.Path()
	full := strings.HasPrefix(path, "/play/") || strings.HasPrefix(path, "/series/")
	// The system's bars show everywhere but the full screen, and the
	// framework holds their insets as the root's padding, so each page
	// already keeps clear of the cutout: the player takes the padding back
	// in view, where its picture runs to the edges.
	ui.Column(c).Fill().Children(func() {
		a.router.View(c, func(r *ui.Route) {
			switch {
			case r.Match("/discover"):
				r.Title("推荐")
				a.gridPage(c, "推荐", a.home, nil, nil, 0, "暂时没有推荐")
			case r.Match("/rank"):
				r.Title("排行榜")
				a.rankPage(c)
			case r.Match("/new"):
				r.Title("新剧上线")
				a.newPage(c)
			case r.Match("/search"):
				r.Title("搜索")
				a.searchPage(c, r.Query("q"))
			case r.Match("/favorites"):
				r.Title("收藏")
				a.favoritesPage(c)
			case r.Match("/mine"):
				r.Title("我的")
				a.minePage(c)
			case r.Match("/series/{id}"):
				r.Title("详情")
				a.seriesView(c, r.Param("id"))
			case r.Match("/play/{id}"):
				r.Title("播放")
				a.playerView(c, r.Param("id"))
			}
		}).Grow(1).Basis(0).FillWidth().Background(t.Background)
		if !full {
			a.tabsView(c)
		}
	})
}

// tabs are the sections of the compact view, in order.
var tabs = []struct {
	label string
	icon  *ui.SVG
	path  string
}{
	{"推荐", iconHome, "/discover"},
	{"榜单", iconChart, "/rank"},
	{"新剧", iconCalendar, "/new"},
	{"搜索", iconSearch, "/search"},
	{"我的", iconPerson, "/mine"},
}

// tabsView is the bar of tabs at the bottom of the compact view. Choosing
// one replaces the page shown, so that going back does not walk the tabs.
func (a *app) tabsView(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Shrink(0).Height(58).Padding(4, 4).Gap(2).
		Background(t.Surface).Children(func() {
		for _, tab := range tabs {
			on := a.section == tab.path
			col := t.TextMuted
			if on {
				col = t.Accent
			}
			b := ui.ButtonBase(c.Key("tab:"+tab.path)).Grow(1).Basis(0).MinWidth(0).Height(50).
				Radius(10).Center().Label(tab.label).Role(ui.RoleTab).AccessSelected(on)
			if on {
				b.Background(t.Accent.Alpha(0.1))
			}
			b.Children(func() {
				ui.Column(c).Gap(3).Center().Children(func() {
					ui.Icon(c, tab.icon).Size(20, 20).TextColor(col)
					ui.Text(c, tab.label).FontSize(10.5).FontWeight(600).TextColor(col).SingleLine()
				})
			})
			if b.Clicked() && !on {
				a.router.Replace(tab.path)
				a.section = tab.path
			}
		}
	})
	ui.Box(c).FillWidth().Height(1).Shrink(0).Background(hairline(t))
}

func (a *app) sidebar(c *ui.Context, vibrant bool) {
	t := c.Theme()
	ui.Box(c).Height(titleBarHeight).Shrink(0).DragWindow()
	field := ui.SearchField(c.Key("search"), &a.query).Placeholder("搜索短剧").Label("搜索短剧").
		Bind(&a.searchField).Margin(0, 12, 10, 12)
	if field.Submitted() {
		a.router.Push("/search?q=" + url.QueryEscape(strings.TrimSpace(a.query)))
	}

	selected := a.section
	path := a.router.Path()
	if id, ok := strings.CutPrefix(path, "/series/"); ok {
		if _, watched := a.hist.get(id); watched {
			selected = path
		}
	}
	list := ui.Sidebar(c, &selected, func() {
		ui.SidebarSection(c, "发现", nil, func() {
			a.sectionItem(c, selected, "/discover", iconHome, "推荐")
			a.sectionItem(c, selected, "/rank", iconChart, "排行榜")
			a.sectionItem(c, selected, "/new", iconCalendar, "新剧上线")
			a.sectionItem(c, selected, "/search", iconFlame, "热搜")
		})
		ui.SidebarSection(c, "我的", nil, func() {
			a.sectionItem(c, selected, "/favorites", iconStar, "收藏").Children(func() {
				if n := len(a.favs.list); n > 0 {
					ui.Textf(c, "%d", n).FontSize(11).TextColor(t.TextMuted).FontFeatures("tnum")
				}
			})
		})
		if len(a.hist.list) > 0 {
			ui.SidebarSection(c, "最近播放", &a.recentOpen, func() {
				ui.Row(c).Gap(3).Padding(0, 8, 4, 8).Children(func() {
					for _, f := range []struct{ id, label string }{
						{"", "全部"}, {"want", "想看"}, {"watching", "正在看"}, {"done", "已看完"},
					} {
						if a.recentFilterButton(c, f.id, f.label) {
							a.recentFilter = f.id
						}
					}
				})
				items := a.hist.filtered(a.recentFilter)
				if len(items) == 0 {
					ui.Text(c, "还没有这样的剧").FontSize(11).TextColor(t.TextMuted).
						Padding(3, 12, 8, 12)
					return
				}
				for _, w := range items[:min(len(items), 8)] {
					ui.SidebarItem(c.Key(w.SeriesID), "/series/"+w.SeriesID, iconClock, w.Title).Children(func() {
						switch w.status() {
						case "want":
							ui.Text(c, "想看").FontSize(11).TextColor(t.TextMuted)
						case "done":
							ui.Text(c, "看完").FontSize(11).TextColor(t.TextMuted)
						default:
							ui.Textf(c, "%d", w.Episode).FontSize(11).TextColor(t.TextMuted).FontFeatures("tnum")
						}
					})
				}
			})
		}
	}).Grow(1).Label("导航")
	if vibrant {
		list.Background(ui.Transparent)
	}
	if list.Changed() {
		a.router.Push(selected)
	}
	a.updateBanner(c)
}

// recentFilterButton is one of the recent list's status filters.
func (a *app) recentFilterButton(c *ui.Context, id, label string) bool {
	t := c.Theme()
	on := a.recentFilter == id
	b := ui.ButtonBase(c.Key("recent:"+id)).Padding(3, 7).Radius(999).Label(label).
		Cursor(ui.CursorPointer)
	switch {
	case on:
		b.Background(t.Accent).TextColor(t.AccentText)
	case b.Hovered():
		b.Background(t.Text.Alpha(0.14))
	default:
		b.Background(t.Text.Alpha(0.07))
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(10.5).FontWeight(500)
	})
	return b.Clicked()
}

// sectionItem is the sidebar's item of a section's page. The item stays
// chosen while a series opened from the page shows, and while search
// results show under 热搜, so a click on it changes no choice: it goes to
// the page itself, as the item promises.
func (a *app) sectionItem(c *ui.Context, selected, id string, icon *ui.SVG, label string) ui.Element {
	item := ui.SidebarItem(c, id, icon, label)
	if item.Clicked() && id == selected && a.router.Location() != id {
		a.router.Push(id)
	}
	return item
}

// header is a page's title, level with the window controls, and what goes
// beside it. On a phone there are no window controls to make room for, and
// the system's bars are already outside the page, so it is closer to the
// edges and opaque, since the grid scrolls under it.
func (a *app) header(c *ui.Context, title string, right func()) {
	m := a.metrics()
	row := ui.Row(c).Height(m.bodyHeight).Shrink(0).AlignItems(ui.Center).Gap(10).
		Padding(0, m.inset())
	if a.compact {
		row.Background(c.Theme().Background)
	} else {
		row.DragWindow()
	}
	row.Children(func() {
		if a.compact && a.router.CanGoBack() && !a.headerOwnsBack() {
			a.headerBack(c)
		}
		size := float32(22)
		if a.compact {
			size = 20
		}
		ui.Text(c, title).FontSize(size).Bold().SingleLine()
		ui.Spacer(c)
		if right != nil {
			right()
		}
	})
}

// headerBack is the compact header's way out of a page reached from a tab,
// as search results are.
func (a *app) headerBack(c *ui.Context) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("header-back")).Size(32, 32).Radius(16).Shrink(0).
		Center().Label("返回").Tooltip("返回")
	if b.Pressed() || b.Hovered() {
		b.Background(t.Text.Alpha(0.1))
	}
	b.Children(func() { ui.Icon(c, iconBack).Size(16, 16) })
	if b.Clicked() {
		a.router.Back()
	}
}

// headerOwnsBack reports whether the page draws its own way back, as the
// series and the player do, so the header need not add one.
func (a *app) headerOwnsBack() bool {
	path := a.router.Path()
	return strings.HasPrefix(path, "/series/") || strings.HasPrefix(path, "/play/")
}

// gridPage is a page of posters under a header that the grid scrolls
// under, with a row of controls under the title when it has them.
func (a *app) gridPage(c *ui.Context, title string, f *feed, right, controls func(), controlsH float32, empty string) {
	m := a.metrics()
	top := m.bodyHeight + controlsH
	ui.Box(c).Fill().Children(func() {
		a.cardGrid(c, f, top+12, empty)
		edge := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(top + 12).PassThrough()
		if a.compact {
			edge.Background(c.Theme().Background)
		} else {
			edge.Material(glass.ScrollEdge{})
		}
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Children(func() {
			a.header(c, title, right)
			if controls != nil {
				ui.Row(c).Height(controlsH).AlignItems(ui.Start).Padding(0, m.inset()).Children(controls)
			}
		})
	})
}

func (a *app) cardGrid(c *ui.Context, f *feed, top float32, empty string) {
	if len(f.cards) == 0 {
		a.feedState(c, f, top, empty)
		return
	}
	m := a.metrics()
	cols, rowH := a.gridMetrics(c)
	n := len(f.cards)
	ui.GridView(c, &f.grid, n, m.cellMin(), rowH, func(i int) {
		a.card(c, f.cards[i], f.showWatch)
		if i >= n-cols*2 && f.more {
			a.loadMore(f)
		}
	}).Fill().Padding(top, m.pad, 32, m.pad).ScrollbarInsets(top, 0, 0, 0)
}

// feedState is what a feed with no series yet shows.
func (a *app) feedState(c *ui.Context, f *feed, top float32, empty string) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(top, 24, 24, 24).Center().Gap(10).Children(func() {
		switch {
		case f.err != nil && f.tries < 4:
			// Try again by itself, after 2, 4, 8 and 16 seconds.
			wait := time.Duration(2<<f.tries) * time.Second
			if since := time.Since(f.errAt); since >= wait {
				f.tries++
				a.retry(f)
			} else {
				c.After(wait - since)
			}
			ui.Spinner(c).FontSize(20).Label("正在重试")
			ui.Text(c, "网络不稳定，正在重试…").FontSize(13).TextColor(t.TextMuted)
		case f.err != nil:
			ui.Text(c, "加载失败").FontSize(15).FontWeight(600)
			ui.Text(c, friendly(f.err)).FontSize(13).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(360)
			ui.Button(c.Key("retry"), "重试").Margin(6, 0, 0, 0).OnClick(func() { f.tries = 0; a.retry(f) })
		case f.loading || !f.started:
			ui.Spinner(c).FontSize(20).Label("正在加载")
			if !f.started {
				a.loadMore(f)
			}
		default:
			ui.Text(c, empty).FontSize(14).TextColor(t.TextMuted)
		}
	})
}

// card is a series in a grid: its poster, title and a line about it.
func (a *app) card(c *ui.Context, k card, showWatch bool) {
	t := c.Theme()
	m := a.metrics()
	b := ui.ButtonBase(c.Key(k.ID)).Fill().Label(k.Title).Cursor(ui.CursorPointer)
	hover := b.Hovered()
	saved := a.favs.has(k.ID)
	b.ContextMenu(func(m *ui.Menu) {
		if m.Item(favLabel(saved)).Chosen() {
			a.favs.toggle(favOf(k))
		}
	})
	b.Children(func() {
		ui.Column(c).Fill().Padding(m.padTop, m.cellPad, m.padBottom, m.cellPad).Gap(m.titleGap).Children(func() {
			poster := a.poster(c, k.Cover, 12, hover).Grow(1).MinHeight(0)
			if showWatch {
				if label, fraction, bar, ok := a.favoriteProgress(k); ok {
					poster.Children(func() { a.watchOverlay(c, label, fraction, bar) })
				}
			}
			ui.Column(c).Height(m.lines).Shrink(0).Gap(4).Padding(0, 2).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if k.Rank > 0 {
						ui.Textf(c, "%d", k.Rank).FontSize(m.titleSize).FontWeight(600).TextColor(t.TextMuted).FontFeatures("tnum")
					}
					ui.Text(c, k.Title).FontSize(m.titleSize).FontWeight(500).SingleLine().Grow(1).MinWidth(0)
					if saved {
						ui.Icon(c, iconStarOn).Size(11, 11).Shrink(0).TextColor(t.TextMuted.Alpha(0.7)).Tooltip("已收藏")
					}
				})
				ui.Text(c, k.Sub).FontSize(m.subSize).TextColor(t.TextMuted.Alpha(0.85)).SingleLine().FontFeatures("tnum")
			})
		})
	})
	if b.Clicked() {
		a.router.Push("/series/" + k.ID)
	}
}

// favoriteProgress is what a saved series' card says about watching it:
// the badge, how full its bar is, and whether it has a bar at all.
func (a *app) favoriteProgress(k card) (label string, fraction float64, bar, ok bool) {
	w, ok := a.hist.get(k.ID)
	if !ok {
		return "", 0, false, false
	}
	season := w.Season
	if season == 0 {
		if sp := a.series[k.ID]; sp != nil && sp.series != nil {
			season = sp.seasonIndex()
		}
	}
	if season == 0 {
		title := w.Title
		if title == "" {
			title = k.Title
		}
		season = seasonNumber(title)
	}
	total := k.Episodes
	if w.Total > 0 {
		total = w.Total
	}
	switch w.status() {
	case "done":
		if season > 0 {
			return fmt.Sprintf("第 %d 季 · 已看完", season), 1, true, true
		}
		return "已看完", 1, true, true
	case "watching":
		if season > 0 {
			return fmt.Sprintf("第 %d 季 · 第 %d 集", season, w.Episode), watchFraction(w, total), true, true
		}
		return fmt.Sprintf("看到第 %d 集", w.Episode), watchFraction(w, total), true, true
	case "want":
		if season > 0 {
			return fmt.Sprintf("第 %d 季 · 想看", season), 0, false, true
		}
		return "想看", 0, false, true
	}
	return "", 0, false, false
}

// watchFraction is how much of a series' episodes were watched, counting
// the part of the one under way.
func watchFraction(w watch, total int) float64 {
	if w.Done {
		return 1
	}
	if total <= 0 {
		return 0
	}
	finished := 0
	for _, e := range w.Episodes {
		if e.Finished {
			finished++
		}
	}
	part := 0.0
	if !w.Finished && w.Episode > 0 {
		if e, ok := w.episode(w.Episode); !ok || !e.Finished {
			_, part = w.episodeState(w.Episode)
		}
	}
	fraction := (float64(finished) + part) / float64(total)
	if fraction < 0 {
		return 0
	}
	if fraction > 1 {
		return 1
	}
	return fraction
}

// watchOverlay draws the progress badge and bar over a poster's bottom.
func (a *app) watchOverlay(c *ui.Context, label string, fraction float64, bar bool) {
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(48).
		LinearGradient(ui.LinearGradient{
			From: ui.RGBA(0, 0, 0, 0), To: ui.RGBA(0, 0, 0, 0.7), Angle: 180,
		})
	ui.Row(c).Absolute().Left(8).Right(8).Bottom(10).Children(func() {
		ui.Row(c).Padding(3, 7).Radius(6).Background(ui.RGBA(0, 0, 0, 0.55)).Children(func() {
			ui.Text(c, label).FontSize(11).FontWeight(600).SingleLine().
				TextColor(ui.RGBA(255, 255, 255, 0.95))
		})
	})
	if bar {
		ui.Progress(c, fraction).Absolute().Left(8).Right(8).Bottom(5).Height(3).Label("观看进度")
	}
}

// poster is a cover in a rounded frame, gray until it loads. Its colors are
// muted until the pointer comes, when it lifts a little in full color.
func (a *app) poster(c *ui.Context, cover string, radius float32, hover bool) ui.Element {
	t := c.Theme()
	p := ui.Box(c).Radius(radius).Clip().Background(placeholder(t))
	lift := p.Animate("lift", b2f(hover), 220*time.Millisecond)
	s := 1 + 0.02*lift
	p.Scale(s, s).Shadow(0, 1+9*lift, 4+20*lift, 0, ui.RGBA(0, 0, 0, 0.08+0.14*lift))
	p.Children(func() {
		if bmp, alpha := a.covers.get(c, cover); bmp != nil {
			ui.Image(c, bmp).Fill().Fit(ui.Cover).Opacity(alpha)
			// A gray copy over the cover, partly see-through, takes some
			// of its color out: MyGo draws images in color or in gray. A
			// phone's finger never comes to unmute them, so they stay in
			// color there.
			mute := coverMute * (1 - lift)
			if onPhone {
				mute = 0
			}
			if mute > 0.01 {
				ui.Image(c, bmp).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover).
					Grayscale().Opacity(alpha * mute)
			}
		}
		// A hairline inside the edge, which keeps dark covers from
		// bleeding into a dark page and light ones into a light page.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).
			Border(1, t.Text.Alpha(0.08)).PassThrough()
	})
	return p
}

// feed returns the feed of key, making it with mk the first time.
func (a *app) feed(key string, mk func() *feed) *feed {
	f := a.feeds[key]
	if f == nil {
		f = mk()
		a.feeds[key] = f
	}
	return f
}

func (a *app) rankPage(c *ui.Context) {
	t := c.Theme()
	content, board := a.rankContent, a.rankBoard
	f := a.feed("rank:"+content+":"+board, func() *feed {
		f := newFeed(a.rankFetcher(content, board))
		f.onPage = func(p page) {
			if len(p.contents) == 0 {
				return
			}
			a.rankContents = p.contents
			if a.rankContent == "" { // the segment of the server's choice
				n := 0
				for _, rc := range p.contents {
					if rc.ID == "" || len(rc.Boards) == 0 || strings.Contains(rc.Name, "演员") {
						continue
					}
					if rc.Selected {
						a.rankIdx = n
					}
					n++
				}
			}
		}
		return f
	})
	// Contents with charts of series; the actors' chart has none.
	var contents []model.RankContent
	for _, rc := range a.rankContents {
		if rc.ID != "" && len(rc.Boards) > 0 && !strings.Contains(rc.Name, "演员") {
			contents = append(contents, rc)
		}
	}
	right := func() {
		if len(contents) < 2 {
			return
		}
		labels := make([]string, len(contents))
		for i, rc := range contents {
			labels[i] = rc.Name
		}
		a.rankIdx = min(a.rankIdx, len(contents)-1)
		ui.Segmented(c.Key("contents"), &a.rankIdx, labels...)
		if id := contents[a.rankIdx].ID; id != a.currentRankContent(contents) {
			a.rankContent, a.rankBoard = id, ""
		}
	}
	controls := func() {
		var boards []model.RankBoard
		cur := a.currentRankContent(contents)
		for _, rc := range contents {
			if rc.ID == cur {
				boards = rc.Boards
			}
		}
		ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
			ui.Row(c).Gap(4).Children(func() {
				for _, b := range boards {
					on := b.ID == a.rankBoard || (a.rankBoard == "" && b.Selected)
					if a.chip(c, "board:"+b.ID, b.Name, on, t) {
						a.rankBoard = b.ID
					}
				}
			})
		})
	}
	a.gridPage(c, "排行榜", f, right, controls, 40, "这个榜单暂时是空的")
}

// currentRankContent is the content whose charts show: the one chosen,
// else the one the server chose.
func (a *app) currentRankContent(contents []model.RankContent) string {
	if a.rankContent != "" {
		return a.rankContent
	}
	for _, rc := range contents {
		if rc.Selected {
			return rc.ID
		}
	}
	if len(contents) > 0 {
		return contents[0].ID
	}
	return ""
}

// chip is a capsule that chooses one of a few, as a chart or a day.
func (a *app) chip(c *ui.Context, key, label string, on bool, t *ui.Theme) bool {
	b := ui.ButtonBase(c.Key(key)).Padding(5, 12).Radius(999).Label(label).Cursor(ui.CursorPointer)
	switch {
	case on:
		b.Background(t.Text).TextColor(t.Background)
	case b.Hovered():
		b.Background(t.Text.Alpha(0.08))
	}
	b.Children(func() { ui.Text(c, label).FontSize(13).FontWeight(500).SingleLine() })
	return b.Clicked() && !on
}

func (a *app) newPage(c *ui.Context) {
	t := c.Theme()
	date := a.newDate
	f := a.feed("new:"+date, func() *feed {
		f := newFeed(a.newFetcher(date))
		f.onPage = func(p page) {
			if len(p.dates) > 0 {
				a.newDates = p.dates
			}
		}
		return f
	})
	today := time.Now().Format("20060102")
	controls := func() {
		ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
			ui.Row(c).Gap(4).Children(func() {
				for _, d := range a.newDates {
					on := d == a.newDate || (a.newDate == "" && d == today)
					if a.chip(c, "day:"+d, dayLabel(d, today), on, t) {
						a.newDate = d
						if d == today {
							a.newDate = ""
						}
					}
				}
			})
		})
	}
	a.gridPage(c, "新剧上线", f, nil, controls, 40, "这一天没有新剧")
}

// dayLabel names a day of the calendar (YYYYMMDD) as people say it.
func dayLabel(d, today string) string {
	day, err := time.ParseInLocation("20060102", d, time.Local)
	now, err2 := time.ParseInLocation("20060102", today, time.Local)
	if err != nil || err2 != nil {
		return d
	}
	switch int(day.Sub(now).Hours() / 24) {
	case 0:
		return "今天"
	case 1:
		return "明天"
	case -1:
		return "昨天"
	}
	return fmt.Sprintf("%d月%d日", day.Month(), day.Day())
}

func (a *app) searchPage(c *ui.Context, q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		a.hotPage(c)
		return
	}
	f := a.feed("search:"+q, func() *feed {
		f := newFeed(a.searchFetcher(q))
		f.order = orderSeasons
		return f
	})
	a.gridPage(c, "“"+q+"”", f, nil, nil, 0, "没有找到“"+q+"”相关的短剧，换个词试试")
}

// hotPage shows what people search for, before a search. On a phone it
// starts with the field to type in, since the sidebar's field is gone.
func (a *app) hotPage(c *ui.Context) {
	t := c.Theme()
	m := a.metrics()
	if a.hot == nil && !a.hotLoading && a.hotErr == nil {
		a.loadHot()
	}
	searchH := float32(0)
	if a.compact {
		searchH = 52
	}
	ui.Box(c).Fill().Children(func() {
		top := m.bodyHeight + searchH
		switch {
		case a.hot == nil && a.hotErr != nil:
			ui.Column(c).Fill().Center().Gap(10).Children(func() {
				ui.Text(c, "加载失败").FontSize(15).FontWeight(600)
				ui.Text(c, friendly(a.hotErr)).FontSize(13).TextColor(t.TextMuted)
				ui.Button(c.Key("retry"), "重试").OnClick(func() { a.hotErr = nil; a.loadHot() })
			})
		case a.hot == nil:
			ui.Column(c).Fill().Center().Children(func() { ui.Spinner(c).FontSize(20).Label("正在加载") })
		default:
			// The boards' cards are as large as the other pages' grids.
			cols, rowH := a.gridMetrics(c)
			gap := t.Space(2)
			ui.Scroll(c).Fill().Padding(top, m.pad, 32, m.pad).ScrollbarInsets(top, 0, 0, 0).Children(func() {
				for _, b := range a.hot.Boards {
					if len(b.Items) == 0 {
						continue
					}
					ui.Text(c, b.Name).FontSize(17).Bold().Margin(20, cellBorder+m.cellPad, 6, cellBorder+m.cellPad)
					for r := 0; r < len(b.Items); r += cols {
						ui.Row(c).Gap(gap).PaddingY(gap / 2).Children(func() {
							for i := r; i < r+cols; i++ {
								if i >= len(b.Items) {
									// Empty places keep the last row's cards as wide as the others.
									ui.Box(c).Grow(1).Basis(0).MinWidth(0)
									continue
								}
								it := b.Items[i]
								k := card{ID: it.SeriesID, Title: it.Title, Cover: it.CoverURL, Sub: it.SearchScore,
									Rank: it.Position, Episodes: it.EpisodeCount}
								if k.Sub == "" {
									k.Sub = it.Heat
								}
								ui.Box(c.Key(b.Name + it.SeriesID)).Grow(1).Basis(0).MinWidth(0).Height(rowH).
									Padding(cellBorder).Children(func() { a.card(c, k, false) })
							}
						})
					}
				}
			})
		}
		edge := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(top + 12).PassThrough()
		if a.compact {
			edge.Background(t.Background)
		} else {
			edge.Material(glass.ScrollEdge{})
		}
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Children(func() {
			a.header(c, "热搜", nil)
			if a.compact {
				ui.Row(c).Height(searchH).Padding(0, m.inset()).AlignItems(ui.Center).Children(func() {
					a.searchBox(c)
				})
			}
		})
	})
}

// searchBox is the field to search with, for the pages a phone shows
// instead of the sidebar that holds it on a computer.
func (a *app) searchBox(c *ui.Context) {
	field := ui.SearchField(c.Key("search"), &a.query).Placeholder("搜索短剧").Label("搜索短剧").
		Bind(&a.searchField).Grow(1)
	if field.Submitted() {
		a.router.Push("/search?q=" + url.QueryEscape(strings.TrimSpace(a.query)))
	}
}

func (a *app) loadHot() {
	a.hotLoading = true
	go func() {
		ctx, cancel := contextTimeout(60 * time.Second)
		defer cancel()
		h, err := a.client.HotSearch(ctx)
		a.update(func() { a.hot, a.hotErr, a.hotLoading = h, err, false })
	}()
}

// favoritesPage is the grid of saved series. It has no fetcher: the grid
// is rebuilt from the list whenever that changes.
func (a *app) favoritesPage(c *ui.Context) {
	if a.favFeed == nil || a.favRev != a.favs.rev {
		f := newFeed(nil)
		f.started, f.showWatch = true, true
		if a.favFeed != nil {
			f.grid = a.favFeed.grid // stay where the viewer scrolled to
		}
		f.add(page{cards: a.favs.cards()})
		a.favFeed, a.favRev = f, a.favs.rev
	}
	a.favFeed.showWatch = true
	a.gridPage(c, "收藏", a.favFeed, nil, nil, 0, "还没有收藏。在剧集页点「收藏」，想看的就存下来。")
}

// minePage is the compact view's 我的: the series watched lately, which the
// sidebar lists on a computer, and the way to the saved ones.
func (a *app) minePage(c *ui.Context) {
	t := c.Theme()
	m := a.metrics()
	ui.Box(c).Fill().Children(func() {
		ui.Scroll(c).Fill().Padding(m.bodyHeight+10, m.pad, 28, m.pad).Gap(20).Children(func() {
			a.savedRow(c)
			ui.Column(c).Gap(10).FillWidth().Children(func() {
				ui.Text(c, "最近播放").FontSize(15).Bold().Padding(0, 2)
				if len(a.hist.list) == 0 {
					ui.Text(c, "看过的剧会出现在这里，点一下接着看。").FontSize(12).TextColor(t.TextMuted).Padding(2, 4)
					return
				}
				ui.ScrollHorizontal(c).Scrollbars(false).Shrink(0).Children(func() {
					ui.Row(c).Gap(6).Children(func() {
						for _, f := range []struct{ id, label string }{
							{"", "全部"}, {"want", "想看"}, {"watching", "正在看"}, {"done", "已看完"},
						} {
							if a.recentFilterButton(c, f.id, f.label) {
								a.recentFilter = f.id
							}
						}
					})
				})
				items := a.hist.filtered(a.recentFilter)
				if len(items) == 0 {
					ui.Text(c, "还没有这样的剧").FontSize(12).TextColor(t.TextMuted).Padding(2, 4)
					return
				}
				ui.Column(c).Gap(2).FillWidth().Children(func() {
					for _, w := range items[:min(len(items), 40)] {
						a.recentRow(c, w)
					}
				})
			})
		})
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Children(func() { a.header(c, "我的", nil) })
	})
}

// savedRow is the way from 我的 to the saved series.
func (a *app) savedRow(c *ui.Context) {
	t := c.Theme()
	n := len(a.favs.list)
	b := ui.ButtonBase(c.Key("saved")).FillWidth().Height(52).Padding(0, 14).Radius(12).
		Label("我的收藏").Cursor(ui.CursorPointer)
	if b.Hovered() || b.Pressed() {
		b.Background(t.Text.Alpha(0.07))
	} else {
		b.Background(t.Text.Alpha(0.05))
	}
	b.Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconStar).Size(18, 18).Shrink(0).TextColor(t.Accent)
			ui.Text(c, "我的收藏").FontSize(14).FontWeight(500).Grow(1).MinWidth(0)
			if n > 0 {
				ui.Textf(c, "%d", n).FontSize(12).TextColor(t.TextMuted).FontFeatures("tnum")
			}
			ui.Icon(c, iconChevron).Size(13, 13).Shrink(0).TextColor(t.TextMuted.Alpha(0.6))
		})
	})
	if b.Clicked() {
		a.router.Push("/favorites")
	}
}

// recentRow is a series watched lately: its cover, its title and where the
// viewer is in it.
func (a *app) recentRow(c *ui.Context, w watch) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("recent:"+w.SeriesID)).FillWidth().Padding(6, 4).Radius(10).
		Label(w.Title).Cursor(ui.CursorPointer)
	if b.Hovered() || b.Pressed() {
		b.Background(t.Text.Alpha(0.06))
	}
	b.Children(func() {
		// The row fills the button: a row that only takes its content's
		// width is centred, which pushes each poster to a different left.
		ui.Row(c).Gap(12).AlignItems(ui.Center).FillWidth().Children(func() {
			a.poster(c, w.Cover, 6, false).Size(44, 59).Shrink(0)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, w.Title).FontSize(13.5).FontWeight(500).SingleLine()
				ui.Text(c, recentLine(w)).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
			})
			ui.Icon(c, iconChevron).Size(13, 13).Shrink(0).TextColor(t.TextMuted.Alpha(0.6))
		})
	})
	if b.Clicked() {
		a.router.Push("/series/" + w.SeriesID)
	}
}

// recentLine says where the viewer is in a series, in a few words.
func recentLine(w watch) string {
	switch w.status() {
	case "want":
		return "想看"
	case "done":
		return "已看完"
	}
	if w.Total > 0 {
		return fmt.Sprintf("看到第 %d 集 / 共 %d 集", w.Episode, w.Total)
	}
	return fmt.Sprintf("看到第 %d 集", w.Episode)
}

// favLabel names the action on a series that is, or is not, saved.
func favLabel(saved bool) string {
	if saved {
		return "取消收藏"
	}
	return "收藏"
}

func hairline(t *ui.Theme) ui.Color    { return t.Text.Alpha(0.1) }
func placeholder(t *ui.Theme) ui.Color { return t.Text.Alpha(0.07) }

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// playerAppearance draws the window's chrome dark while the player shows,
// so that the title bar that comes down with the menu bar in full screen is
// black rather than the system's white. The player's page is dark already;
// the rest of the app takes its appearance back afterwards.
func (a *app) playerAppearance(on bool) {
	if a.win == nil {
		return // no window to draw, as in the tester
	}
	if on {
		a.playerTheme = mygo.Theme.Source()
		mygo.Theme.SetSource(mygo.ThemeDark)
		return
	}
	if a.playerTheme != "" {
		mygo.Theme.SetSource(a.playerTheme)
		a.playerTheme = ""
	}
}

// playerBars keeps the system's bars in their place: they show on every
// page, and go only while the picture fills the screen, where the layout
// draws a row of its own. The window's own background fills what the bars'
// insets leave — the band at the screen's edge — so it is blacked for the
// player and left light elsewhere.
func (a *app) playerBars(on bool) {
	if !onPhone {
		return
	}
	if a.win != nil {
		bg := "#FFFFFF"
		if on {
			bg = "#000000"
		}
		if bg != a.barsBG {
			a.barsBG = bg
			_ = a.win.SetBackgroundColor(bg)
		}
	}
	style := mygo.BarStyleAuto
	if on {
		style = mygo.BarStyleLight
	}
	if style != a.barsStyle {
		a.barsStyle = style
		mygo.SystemBars.SetStyle(style)
	}
	hidden := on && a.landscapeFull
	if hidden != a.barsHidden {
		a.barsHidden = hidden
		mygo.SystemBars.SetHidden(hidden)
	}
}

// titleBarLeft is where a title bar control goes, clear of the window's
// traffic lights (and of the screen's edge in full screen).
func titleBarLeft(c *ui.Context, gap float32) float32 {
	return max(c.TitleBar().Left+gap, 16)
}

func itoa(n int) string { return strconv.Itoa(n) }
