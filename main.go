// 开心果 is a small, fast desktop player for Hongguo short dramas, built on
// the openhongguo SDK and MyGo's native UI.
package main

import (
	"context"
	"io"
	"log"
	"net/url"
	"path/filepath"
	"time"

	playermod "github.com/kadaliao/kaixinguo/modules/player"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	titleBarHeight = 40
	sidebarWidth   = 220
)

// app is the state the window shows. Everything in it belongs to the main
// thread: work on other goroutines hands its results over with update.
type app struct {
	win    *mygo.Window
	ready  chan struct{} // closed once win is set
	router *ui.Router
	client *client
	covers *covers
	hist   *history
	favs   *favorites
	proxy  *proxy

	settings settingsForm // the settings dialog

	cacheDir string
	dataDir  string

	// The first launch's tour, until it is finished or skipped.
	tutorialOn   bool
	tutorialStep int

	// compact is set while the window is a phone's—every mobile build, and
	// a desktop window narrower than compactWidth—and the app shows tabs
	// under its pages instead of the sidebar.
	compact bool
	// barsHidden is whether the full-screen player hid the system's bars,
	// so that they are asked for only when they change; barsBG and
	// barsStyle are the colours last given them.
	barsHidden bool
	barsBG     string
	barsStyle  mygo.BarStyle
	// batt is the battery the full screen's row shows, read again every
	// now and then rather than on every frame.
	batt   mygo.BatteryStatus
	battAt time.Time
	// keepAwake releases the phone's screen from locking while an episode
	// plays, and is nil while nothing holds it.
	keepAwake func()

	// The sidebar.
	query        string
	searchField  ui.Handle
	section      string // the sidebar's page the page shown belongs to
	recentOpen   bool
	recentFilter string // recent list filter: "", want, watching or done
	sideShown    float32

	// The catalog.
	home         *feed
	feeds        map[string]*feed
	rankContent  string
	rankBoard    string
	rankIdx      int
	rankContents []model.RankContent
	newDate      string
	newDates     []string
	hot          *model.HotSearch
	hotLoading   bool
	hotErr       error
	favFeed      *feed
	favRev       int // the favorites' rev favFeed was built from

	// Series and playback.
	series  map[string]*seriesPage
	futures map[string]*future
	cur     *playback
	gen     int

	// The theater: the player filling the screen, its episodes a drawer
	// that follows the pointer to the right edge.
	fullscreen        bool
	theaterFullscreen bool // the theater entered full screen, so leaving may leave it
	theaterShown      bool
	panelOpen         bool
	panelSuppressed   bool // closed by hand, until the pointer leaves
	panelJustClosed   bool // the overlay is a frame behind the close
	panelReveal       bool // the next drawer scrolls to the episode playing
	panelPinned       bool // opened from the toolbar; stays until entered or closed

	// The player's first entry shows its own small tour.
	playerTutorialOn   bool
	playerTutorialStep int
	playerTutorialHeld bool

	// The appearance to go back to when the player closes.
	playerTheme mygo.ThemeSource

	// A newer version the updater found, and how to offer it again.
	newVersion   string
	checkUpdates func()

	// 弹幕: whether they show, the episodes' lines read so far, and how
	// to read more (the client's ListDanmaku; tests swap it).
	danmakuOn   bool
	tracks      map[string]*danmakuTrack
	trackOrder  []string
	listDanmaku func(ctx context.Context, seriesID, episodeID string, opts sdk.DanmakuOptions) (*model.DanmakuPage, error)

	// The player's web view.
	web       *mygo.WebView
	webReady  bool
	pendingJS []string    // what the page runs once it has loaded
	js        chan string // what the page runs, in order
	onPlayer  bool
	playerSet bool // the player page has its system settings
	// The native player, where the platform has one: Android plays with
	// ExoPlayer in its own view, and nil here keeps the web view playing.
	player       playermod.Player
	eventSub     io.Closer
	timeSub      io.Closer
	playerRate   float64   // the speed the native player plays at
	holdOn       bool      // the player's tutorial holds playback
	lastProgress time.Time // the last playhead the history recorded
	dm           *danmakuUI
	// The bar the phone's player draws itself: where its slider sits, that
	// it is being dragged, and its speeds open.
	seekPos  float64
	seeking  bool
	rateOpen bool
	// playerChrome is whether the page's bottom bar is showing. In the
	// theater the top buttons hide and come back with it.
	playerChrome bool
	// playerTopHovered is whether the pointer was over the top buttons,
	// which keeps them as the bar hides.
	playerTopHovered bool

	// The phone's three player layouts: whether the video's picture is
	// wider than it is tall (图二), and whether the screen is turned on
	// its side for the full-screen player (图三). The rest of the time a
	// phone plays the way 图一 does.
	videoLandscape bool
	landscapeFull  bool
	// liked is the player's own 点赞, kept until the player closes: the
	// SDK only reads the series' count.
	liked bool
	// dmInput is what the 全屏 player's 弹幕 box holds.
	dmInput string
	// moreOpen is whether the phone player's ⋮ menu shows.
	moreOpen bool
	// hint is a word of feedback the phone player says for a moment.
	hint string
	// chromeGen numbers the auto-hide waits: only the latest one hides.
	chromeGen int
	// dmEditing is whether the 弹幕 box is being typed in, which keeps the
	// buttons up.
	dmEditing bool
	// swipeDX is how far the finger has moved sideways on the picture,
	// and swipeDY up or down: sideways moves the playhead, up or down
	// steps an episode.
	swipeDX   float32
	swipeDY   float32
	swipeHeld bool
	// stallTick numbers the waits for a picture that stopped coming: only
	// the latest one may reload the episode.
	stallTick int
	// videoRemount is set for one frame after a turn of the screen, which
	// is drawn without the picture so the framework places its view again.
	videoRemount bool
}

func newApp(dataDir, cacheDir string) *app {
	a := &app{
		ready:        make(chan struct{}),
		router:       ui.NewRouter("/discover"),
		cacheDir:     cacheDir,
		dataDir:      dataDir,
		section:      "/discover",
		recentOpen:   true,
		sideShown:    1,
		feeds:        map[string]*feed{},
		series:       map[string]*seriesPage{},
		futures:      map[string]*future{},
		tracks:       map[string]*danmakuTrack{},
		dm:           newDanmakuUI(),
		playerRate:   1,
		danmakuOn:    danmakuOn(dataDir),
		hist:         loadHistory(dataDir),
		favs:         loadFavorites(dataDir),
		playerChrome: true,
	}
	a.covers = newCovers(filepath.Join(cacheDir, "covers"), a.update)
	// 弹幕 show from the first frame when the switch says so: the flight's
	// on-state starts as the setting, not off until someone toggles it.
	a.dm.setOn(a.danmakuOn)
	a.home = newFeed(a.discover)
	// Last launch's recommendations show at once; fresh ones replace them.
	if cards := a.loadHomeCache(); len(cards) > 0 {
		a.home.add(page{cards: cards, more: true})
		a.home.stale = true
	}
	a.home.onPage = func(page) { a.saveHomeCache(a.home.cards) }
	return a
}

// update runs fn on the main thread and redraws, once the window exists.
func (a *app) update(fn func()) {
	<-a.ready
	a.win.Update(fn)
}

func main() {
	log.SetFlags(0)
	if err := setPlatformPaths(); err != nil {
		log.Fatal(err)
	}
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	cacheDir, err := mygo.App.Path(mygo.PathCache)
	if err != nil {
		log.Fatal(err)
	}
	a := newApp(dataDir, cacheDir)
	a.tutorialOn = !tutorialSeen(dataDir)
	a.playerTutorialOn = !playerTutorialSeen(dataDir)
	if a.client, err = newClient(dataDir); err != nil {
		log.Fatal(err)
	}
	if s := loadSignerSettings(dataDir); s.URL != "" {
		if err := a.client.setSigner(s.URL, s.Token); err != nil {
			log.Printf("signer: %v", err)
		}
	}
	a.listDanmaku = a.client.ListDanmaku
	if a.proxy, err = newProxy(func(q url.Values) { a.update(func() { a.onPlayerEvent(q) }) }); err != nil {
		log.Fatal(err)
	}
	// What was saved last, as the app ended, reaches the disk.
	save := func() {
		a.hist.store.flush()
		a.favs.store.flush()
	}
	mygo.App.OnQuit(func() {
		save()
		if a.player != nil {
			a.player.Close()
		}
	})
	// A phone never quits: it goes to the background, where the system may
	// end it. Save there, and stop the picture — but leave the player
	// running. Closed there, it would be released, and the app would come
	// back to a player that never plays another frame.
	mygo.Power.OnSuspend(func() {
		save()
		select {
		case <-a.ready:
			a.win.Update(func() {
				if a.onPlayer {
					a.pausePlayback()
				}
			})
		default:
		}
	})
	// Back in the foreground, the episode goes on from where it stopped.
	mygo.Power.OnResume(func() {
		select {
		case <-a.ready:
			a.win.Update(func() {
				if a.onPlayer && a.player != nil {
					a.resumePlayback()
				}
			})
		default:
		}
	})
	useUpdater(a)
	mygo.App.WhenReady(func() {
		opts := mygo.WindowOptions{
			Title:   "开心果",
			Content: ui.View(a.view),
		}
		if !onPhone {
			// A phone's screen is the window: it fills it, and the system
			// draws its bars and its edges.
			opts.Width, opts.Height = 1180, 760
			opts.MinWidth, opts.MinHeight = 780, 520
			opts.StateKey = "main"
			// Hidden, not HiddenInset: the inset style adds a toolbar
			// taller than the title bar the app draws.
			opts.TitleBarStyle = mygo.TitleBarHidden
			opts.TrafficLightPosition = &mygo.Point{X: 20, Y: 13}
			opts.TitleBarHeight = titleBarHeight
			opts.Vibrancy = mygo.VibrancySidebar
		}
		a.win = mygo.NewWindow(opts)
		a.fullscreen = a.win.IsFullScreen()
		log.Println("kaixinguo: window ready")
		close(a.ready)
		// Where the platform has a native player, it plays; elsewhere the
		// player's web view does.
		if p, err := playermod.NewPlayer(); err == nil {
			log.Println("kaixinguo: native player ready")
			a.player = p
			a.eventSub = p.OnEvent(func(s playermod.State, code string) {
				a.update(func() { a.playerState(s, code) })
			})
			a.timeSub = p.OnTime(func(pos, dur time.Duration) {
				a.update(func() { a.playerTime(pos, dur) })
			})
		} else {
			log.Printf("kaixinguo: no native player: %v", err)
		}
		a.watchUpdates()
		// The theater follows the window: entering full screen with the
		// green button does what the theater button does.
		a.win.OnEnterFullScreen(func() {
			a.win.Update(func() { a.fullscreen = true })
		})
		a.win.OnLeaveFullScreen(func() {
			a.win.Update(func() {
				a.fullscreen = false
				a.theaterFullscreen = false
				a.panelOpen = false
				a.panelReveal = false
				a.panelPinned = false
				a.theaterShown = false
			})
		})
		a.loadMore(a.home)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
