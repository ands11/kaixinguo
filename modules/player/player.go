// Package player is a native module: its interfaces are implemented in
// Swift, Kotlin, Rust or Zig, in the directory of the language (see AGENTS.md).
package player

import (
	"io"
	"time"

	"github.com/egoist/mygo/native"
)

// State is what a player is doing.
type State string

// VideoSize is a video's size in pixels.
type VideoSize struct {
	Width, Height int64
}

const (
	StatePlaying   State = "playing"   // the video shows and sounds
	StateBuffering State = "buffering" // the video loads or stalls
	StateEnded     State = "ended"     // the video played to its end
	StateError     State = "error"     // the video failed; code says roughly why
)

// Player plays a video in a platform view, shown with ui.NativeView. It
// plays an HTTP URL of the app's local proxy, which serves the file with
// its Range.
//
//mygo:native
type Player interface {
	native.View
	io.Closer

	// Load plays the video at url from start in. It replaces whatever the
	// player had, so episode changes reuse the same player.
	Load(url string, start time.Duration)
	// Play starts or resumes the video.
	Play()
	// Pause stops at the playhead.
	Pause()
	// Seek goes to to in the video.
	Seek(to time.Duration)
	// SetRate plays at rate times the normal speed.
	SetRate(rate float64)
	// VideoSize is the video's width and height in pixels, 0, 0 until the
	// video is known.
	VideoSize() VideoSize

	// OnEvent calls f when the state changes, in order, on Go's main
	// thread. StateError comes with a code of "io", "decode" or "drm".
	OnEvent(f func(s State, code string)) io.Closer
	// OnTime calls f with the playhead and the duration while the video
	// plays, in order, on Go's main thread.
	OnTime(f func(pos, dur time.Duration)) io.Closer
}
