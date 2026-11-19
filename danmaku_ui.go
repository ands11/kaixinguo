package main

import (
	"math"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
)

// The 弹幕 of the native player fly in Go's own layer, over the picture:
// each frame's lines come from this state, which the player view's build
// reads. It is the player page's flight, ported: the lines cross the
// picture's width in a few lanes, whose clock is the video's, so they
// pause, speed up and jump with it.

const (
	dmLateMs = 1500 // ms a line may come late and still fly
	dmWaitMs = 2500 // ms a line waits for a free lane
	dmGapPx  = 28   // px between two lines of a lane
)

// dmLane is a lane's room. clear is the playhead, in ms, from which the
// lane takes a new line, and exit when its last line leaves the picture.
type dmLane struct{ clear, exit float64 }

// dmFly is a line in the air.
type dmFly struct {
	b     bullet
	lane  int
	w     float32 // the line's width, in DIP
	start float64 // the playhead, in ms, it launched at
}

// dmPending is a line due that waits for a lane.
type dmPending struct {
	b  bullet
	at float64 // the playhead, in ms, it came due at
}

// danmakuUI is the flight's state.
type danmakuUI struct {
	on     bool
	lines  []bullet // fed, in the order they came
	dmNext int      // the next line that may come due
	lane   []dmLane
	flying []*dmFly
	queue  []dmPending
	widths map[string]float32

	// The clock: the player's playhead and speed, and when it said the
	// playhead. The flight carries it on between ticks, and freezes 250ms
	// after a tick stops coming, which is a pause or a stall.
	pos  time.Duration
	rate float64
	wall time.Time

	// The video's width over its height, for the picture's rect, 0 until
	// the video is known.
	aspect float32
}

func newDanmakuUI() *danmakuUI {
	// rate starts at the player's own: with it at zero the flight would
	// stand still between the player's quarter-second ticks and step.
	return &danmakuUI{widths: map[string]float32{}, rate: 1}
}

// setOn shows or hides the flight.
func (d *danmakuUI) setOn(on bool) {
	d.on = on
	if !on {
		d.clear()
	}
}

// clear takes every flying line down.
func (d *danmakuUI) clear() {
	d.flying = d.flying[:0]
	d.queue = d.queue[:0]
	for i := range d.lane {
		d.lane[i] = dmLane{clear: -1e18, exit: -1e18}
	}
}

// reset forgets an episode's lines: a new one plays.
func (d *danmakuUI) reset() {
	d.clear()
	d.lines = d.lines[:0]
	d.dmNext = 0
}

// seeked starts the flight over from the playhead: what flies goes, and
// the lines from there on fly again.
func (d *danmakuUI) seeked() {
	d.clear()
	d.dmNext = dmIndex(d.lines, int64(d.nowMs()))
}

// feed takes the lines of the episode playing that it doesn't have yet,
// in the order they were read. They may come while the episode plays, and
// what flies goes on flying.
func (d *danmakuUI) feed(lines []bullet) {
	d.lines = append(d.lines, lines...)
	sort.Slice(d.lines, func(i, j int) bool { return d.lines[i].at < d.lines[j].at })
	// Lines that just came due still fly.
	d.dmNext = dmIndex(d.lines, int64(d.nowMs())-dmLateMs)
}

// clock is the player's playhead, from its latest tick.
func (d *danmakuUI) clock(pos time.Duration) {
	d.pos, d.wall = pos, time.Now()
}

// nowMs estimates the playhead, in ms, at the wall clock's now: the latest
// tick, carried on at the playback's speed while ticks keep coming.
func (d *danmakuUI) nowAt(now time.Time) float64 {
	if d.wall.IsZero() {
		return float64(d.pos.Milliseconds())
	}
	since := now.Sub(d.wall)
	if since > 750*time.Millisecond {
		return float64(d.pos.Milliseconds()) // frozen: paused or stalled
	}
	return float64(d.pos.Milliseconds()) + float64(since)*d.rate/float64(time.Millisecond)
}

// nowMs is the playhead now, outside a frame's paint.
func (d *danmakuUI) nowMs() float64 { return d.nowAt(time.Now()) }

// dmIndex is the first of lines that shows at or after t, in ms.
func dmIndex(lines []bullet, t int64) int {
	lo, hi := 0, len(lines)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if lines[m].at < t {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo
}

// advance lets the flight on from the playhead t, in ms, across a picture
// of width W with room for lanes lines: what came due flies, waits, or
// gives up, and what left the picture comes down. It reports whether
// anything is in the air or still due.
func (d *danmakuUI) advance(t float64, W float32, lanes int) bool {
	for len(d.lane) < lanes {
		d.lane = append(d.lane, dmLane{clear: -1e18, exit: -1e18})
	}
	// A line comes due at its time; one that comes later than dmLateMs
	// past it never flies.
	for d.dmNext < len(d.lines) && float64(d.lines[d.dmNext].at) <= t {
		l := d.lines[d.dmNext]
		d.dmNext++
		if t-float64(l.at) <= dmLateMs {
			d.queue = append(d.queue, dmPending{b: l, at: float64(l.at)})
		}
	}
	// A line flies when a lane has room, and waits at most dmWaitMs for
	// one.
	kept := d.queue[:0]
	for _, q := range d.queue {
		if t-q.at <= dmWaitMs && !d.launch(q.b, t, W) {
			kept = append(kept, q)
		}
	}
	d.queue = kept
	// Gone lines come down.
	out := d.flying[:0]
	for _, f := range d.flying {
		if t-f.start < d.flyDur(W) {
			out = append(out, f)
		}
	}
	d.flying = out
	return (len(d.flying) > 0 || len(d.queue) > 0 ||
		d.dmNext < len(d.lines) && float64(d.lines[d.dmNext].at) <= t+dmWaitMs) && !d.frozen()
}

// frozen reports whether the clock stands still: the player's ticks have
// stopped coming, which is a pause or a stall, and nothing glides.
func (d *danmakuUI) frozen() bool {
	return !d.wall.IsZero() && time.Since(d.wall) > 750*time.Millisecond
}

// launch flies b from the playhead t, if a lane has room, and reports
// whether it did. A longer line is faster, that it reaches the left edge
// only after the line ahead of it in its lane is all in.
func (d *danmakuUI) launch(b bullet, t float64, W float32) bool {
	w := d.width(b.text)
	D := d.flyDur(W)
	speed := (float64(W) + float64(w)) / D // px per ms of play
	for i := range d.lane {
		n := &d.lane[i]
		if t >= n.clear && t+float64(W)/speed >= n.exit {
			n.clear = t + (float64(w)+dmGapPx)/speed
			n.exit = t + D
			d.flying = append(d.flying, &dmFly{b: b, lane: i, w: w, start: t})
			return true
		}
	}
	return false
}

// flyDur is how long a line takes across a picture of W, in ms. A slower
// flight is a smaller step from frame to frame, which is what the eye sees
// as smooth.
func (d *danmakuUI) flyDur(W float32) float64 {
	return max(7000, min(12000, float64(W)*18))
}

// width is a line's width at the flight's size, measured once.
func (d *danmakuUI) width(s string) float32 {
	if w, ok := d.widths[s]; ok {
		return w
	}
	w, _ := ui.ShapeText(s, dmFont, 0).Size()
	d.widths[s] = w
	return w
}

// dmFont is a 弹幕 line's font. Its size follows the picture's, but only
// in the view: measurements take it at the middle of the range, which is
// what the width of an average line is off by no more than a few points.
var dmFont = ui.Font{Size: 17, LineHeight: 1.3}

// dmGeom is where the flight goes: the picture's rect in the view, and the
// room the lines have.
type dmGeom struct {
	left, top      float32 // the picture's corner in the view
	vw, vh         float32 // the picture's size
	fs, lh, boxTop float32
	lanes          int
}

// dmGeometry lays the flight out for a view of w by h and a picture of
// aspect ar. Upright, the picture is letterboxed in the middle of the
// screen, and the flight starts at the screen's top instead, under the
// episode's row.
func dmGeometry(w, h, ar float32, upright, full bool) dmGeom {
	if ar <= 0 {
		ar = 9.0 / 16.0
	}
	vw, vh := w, h
	if vw/vh > ar {
		vw = vh * ar
	} else {
		vh = vw / ar
	}
	g := dmGeom{left: (w - vw) / 2, top: (h - vh) / 2, vw: vw, vh: vh}
	g.fs = max(13, min(22, float32(math.Round(float64(vh)*0.024))))
	g.lh = float32(math.Round(float64(g.fs) * 1.6))
	g.lanes = int(max(3, min(6, math.Floor(float64(vh)*0.3/float64(g.lh)))))
	g.boxTop = max(48-g.top, float32(math.Round(float64(vh)*0.06)))
	if upright && !full {
		g.top = 0
		g.boxTop = 96
	}
	return g
}

// dmView paints the flying 弹幕 over the native player's picture. It is a
// Draw box rather than elements: while it asks for frames, MyGo paints the
// drawing alone on the last frame's scene, without building the view, so
// the lines glide at the display's pace instead of stepping from build to
// build.
func (a *app) dmView(c *ui.Context) {
	d := a.dm
	if d == nil || !d.on || a.player == nil {
		return
	}
	w, h := c.Size()
	if w < 1 || h < 1 {
		return
	}
	g := dmGeometry(w, h, d.aspect, onPhone, a.landscapeFull)
	ui.Box(c.Key("dm-ink")).Absolute().Fill().PassThrough().Draw(func(p *ui.Painter, _ ui.Rect) {
		t := d.nowAt(p.Now())
		d.advance(t, g.vw, g.lanes)
		for _, f := range d.flying {
			x := g.vw - float32((t-f.start)/d.flyDur(g.vw))*(g.vw+f.w)
			y := g.top + g.boxTop + float32(f.lane)*g.lh
			p.Text(g.left+x, y, f.b.text, g.fs, ui.RGBA(255, 255, 255, 0.86))
		}
		if len(d.flying)+len(d.queue) == 0 {
			return // nothing to move
		}
		// 60 frames a second is all the eye needs of a line crossing the
		// picture, and the display shows them evenly.
		p.AnimationFrameRate(60)
	})
}
