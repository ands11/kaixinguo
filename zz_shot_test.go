package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestShotPlayer draws the phone player with its episodes open, for a
// look at the drawer while adapting the app to Android.
func TestShotPlayer(t *testing.T) {
	a, tt := compactApp(t)
	a.cur = &playback{seriesID: "1", number: 1, gen: 1}
	a.fullscreen = true
	a.router.Push("/play/1")
	tt.Frame()
	tt.Advance(time.Second)
	click(t, tt, "选集")
	tt.Advance(time.Second)
	savePNG(t, tt.Image(), "player-drawer")

	// The tour over the player, too.
	a.playerTutorialOn = true
	a.playerTutorialStep = 0
	tt.Frame()
	savePNG(t, tt.Image(), "player-tutorial")
}

// TestShotSeries draws the phone series page.
func TestShotSeries(t *testing.T) {
	a, tt := compactApp(t)
	a.router.Push("/series/1")
	tt.Frame()
	tt.Advance(time.Second)
	savePNG(t, tt.Image(), "series")
}

// TestShotTutorial draws the app tour over the grid.
func TestShotTutorial(t *testing.T) {
	a, tt := compactApp(t)
	a.tutorialOn = true
	a.tutorialStep = 0
	tt.Frame()
	tt.Advance(time.Second)
	savePNG(t, tt.Image(), "tutorial")
}

func savePNG(t *testing.T, img *image.RGBA, name string) {
	t.Helper()
	if img == nil {
		t.Fatal("no image")
	}
	dir := filepath.Join(".shots", "ui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
