package main

import (
	"io"
	"testing"
	"time"

	playermod "github.com/kadaliao/kaixinguo/modules/player"
)

// fakePlayer is the native player a test answers for: it records what the
// app asks of it, and hands its events back when the test says.
type fakePlayer struct {
	calls []string
	event func(playermod.State, string)
	tick  func(time.Duration, time.Duration)
}

func (f *fakePlayer) NativeView() uintptr { return 0 }
func (f *fakePlayer) Load(url string, start time.Duration) {
	f.calls = append(f.calls, "load")
}
func (f *fakePlayer) Play()  { f.calls = append(f.calls, "play") }
func (f *fakePlayer) Pause() { f.calls = append(f.calls, "pause") }
func (f *fakePlayer) Seek(to time.Duration) {
	f.calls = append(f.calls, "seek")
}
func (f *fakePlayer) SetRate(r float64) { f.calls = append(f.calls, "rate") }
func (f *fakePlayer) VideoSize() playermod.VideoSize {
	return playermod.VideoSize{Width: 720, Height: 1280}
}
func (f *fakePlayer) OnEvent(fn func(playermod.State, string)) io.Closer {
	f.event = fn
	return closeFn(func() { f.event = nil })
}
func (f *fakePlayer) OnTime(fn func(time.Duration, time.Duration)) io.Closer {
	f.tick = fn
	return closeFn(func() { f.tick = nil })
}
func (f *fakePlayer) Close() error { f.calls = append(f.calls, "close"); return nil }

// closeFn is a Closer that runs fn once.
type closeFn func()

func (c closeFn) Close() error { c(); return nil }

// did reports whether the player was asked for each of want, in order.
func (f *fakePlayer) did(want ...string) bool {
	if len(f.calls) != len(want) {
		return false
	}
	for i := range want {
		if f.calls[i] != want[i] {
			return false
		}
	}
	return true
}

// The hold keeps the episode waiting until the player's tour is over,
// then the native player starts it.
func TestNativeHold(t *testing.T) {
	a := testApp(t)
	p := &fakePlayer{}
	a.player = p
	a.cur = &playback{seriesID: "1", number: 1, gen: 1, epID: "e1", loaded: true}
	a.holdPlayback(true)
	if !p.did() {
		t.Fatalf("a hold was asked something: %q", p.calls)
	}
	a.holdPlayback(false)
	if !p.did("play") {
		t.Fatalf("the hold's end asked %q, want play", p.calls)
	}
}

// Play and pause follow the play button, and a seek goes from the
// playhead and restarts the 弹幕.
func TestNativeToggleAndSeek(t *testing.T) {
	a := testApp(t)
	p := &fakePlayer{}
	a.player = p
	a.cur = &playback{seriesID: "1", number: 1, gen: 1, epID: "e1", loaded: true, state: playPlaying, at: 95_000}
	a.togglePlay()
	if !p.did("pause") {
		t.Fatalf("playing: toggle asked %q, want pause", p.calls)
	}
	a.cur.state = playBuffering
	a.togglePlay()
	if !p.did("pause", "play") {
		t.Fatalf("buffering: toggle asked %q, want play", p.calls)
	}
	a.seekBy(10)
	if !p.did("pause", "play", "seek") {
		t.Fatalf("seek asked %q", p.calls)
	}
	if got := len(a.dm.flying) + len(a.dm.queue); got != 0 {
		t.Fatalf("the seek left %d 弹幕 flying", got)
	}
}

// The native player's states go where the page's events went.
func TestNativeStates(t *testing.T) {
	a := testApp(t)
	p := &fakePlayer{}
	a.player = p
	a.cur = &playback{seriesID: "1", number: 1, gen: 1, epID: "e1"}
	a.futures["e1"] = &future{done: make(chan struct{})}
	a.playerState(playermod.StateBuffering, "")
	if a.cur.state != playResolving {
		t.Fatalf("a buffering before playing left %d", a.cur.state)
	}
	a.playerState(playermod.StatePlaying, "")
	if a.cur.state != playPlaying {
		t.Fatalf("playing left %d", a.cur.state)
	}
	if _, ok := a.futures["e2"]; !ok {
		t.Fatal("the next episode was not resolved while this one plays")
	}
	a.playerState(playermod.StateBuffering, "")
	if a.cur.state != playBuffering {
		t.Fatalf("a stall left %d", a.cur.state)
	}
	a.playerState(playermod.StateError, "io")
	if a.cur.state != playFailed || a.cur.msg == "" {
		t.Fatalf("an error left %d, %q", a.cur.state, a.cur.msg)
	}
	if _, ok := a.futures["e1"]; ok {
		t.Fatal("the episode's resolve stayed for a retry")
	}
}

// The playhead the native player reports drives the 弹幕 clock, and the
// history records it at the page's three-second cadence.
func TestNativeTime(t *testing.T) {
	a := testApp(t)
	p := &fakePlayer{}
	a.player = p
	a.cur = &playback{seriesID: "1", number: 1, gen: 1, epID: "e1", state: playPlaying}
	a.playerTime(500*time.Millisecond, 60*time.Second)
	a.playerTime(520*time.Millisecond, 60*time.Second)
	if a.cur.dur != 60 {
		t.Fatalf("the episode is %v s long", a.cur.dur)
	}
	w, ok := a.hist.get("1")
	if !ok || w.Episode != 1 || w.Position < 0.5 {
		t.Fatalf("history has %+v", w)
	}
	first := a.lastProgress
	a.playerTime(530*time.Millisecond, 60*time.Second)
	if !a.lastProgress.Equal(first) {
		t.Fatal("the history recorded twice within three seconds")
	}
}

// The flight: lines fly over lanes at the playhead's pace, wait for a
// lane, and come down at the picture's left edge.
func TestDanmakuFlight(t *testing.T) {
	a := testApp(t)
	d := a.dm
	d.on = true
	d.feed([]bullet{{at: 0, text: "第一条"}, {at: 0, text: "第二条"}, {at: 100, text: "第三条"}})
	if !d.advance(0, 300, 4) {
		t.Fatal("nothing flies at the episode's start")
	}
	if len(d.flying) != 2 || len(d.queue) != 0 {
		t.Fatalf("at t=0 %d fly and %d wait, want 2 and 0", len(d.flying), len(d.queue))
	}
	// The third line comes due a hundredth in, and its lane takes it when
	// the line ahead of it is all in.
	if !d.advance(100, 300, 4) {
		t.Fatal("nothing flies when the third line comes due")
	}
	if len(d.flying) != 3 {
		t.Fatalf("%d fly at t=100, want 3", len(d.flying))
	}
	// The lane's last line is all in before the next one of it flies.
	if !d.advance(float64(d.flyDur(300))+10, 300, 4) {
		t.Fatal("nothing flies once the first lines are gone")
	}
	if len(d.flying) != 1 {
		t.Fatalf("%d fly at the first lines' end, want 1", len(d.flying))
	}
	// A pause freezes the flight, which stops drawing for it.
	d.wall = time.Now().Add(-time.Second)
	if d.advance(0, 300, 4) {
		t.Fatal("a frozen clock keeps the flight going")
	}
}

// The phone's player draws its own bar over the platform's view, and the
// playhead and the speed show in it.
func TestNativeBar(t *testing.T) {
	a, tt := playerApp(t)
	a.player = &fakePlayer{}
	a.cur.loaded = true
	tt.Frame()
	for _, s := range []string{"0:00", "1.0×", "弹"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q in %q", s, tt.Texts())
		}
	}
}
