package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// compactApp is the test app in a phone's window: narrow enough for the
// app's own tabs, without the sidebar.
func compactApp(t *testing.T) (*app, *ui.Tester) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 390, 844)
	tt.Advance(time.Second)
	return a, tt
}

// A phone has no room for a sidebar: the sections are tabs at the bottom,
// and the recent plays move to 我的.
func TestCompactLayout(t *testing.T) {
	a, tt := compactApp(t)
	a.hist.record(watch{SeriesID: "1", Title: "浅浅归聿", Episode: 7, Total: 130})
	tt.Frame()
	if tt.HasText("最近播放") {
		t.Fatalf("the sidebar shows on a phone: %q", tt.Texts())
	}
	for _, s := range []string{"推荐", "榜单", "新剧", "搜索", "我的", "浅浅归聿"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q in %q", s, tt.Texts())
		}
	}
	click(t, tt, "我的")
	if got := a.router.Path(); got != "/mine" {
		t.Fatalf("path %q after the 我的 tab", got)
	}
	for _, s := range []string{"最近播放", "我的收藏", "看到第 7 集 / 共 130 集"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q on 我的: %q", s, tt.Texts())
		}
	}

	// The favorite row opens the grid, and the header's back button
	// returns to 我的.
	click(t, tt, "我的收藏")
	if got := a.router.Path(); got != "/favorites" {
		t.Fatalf("path %q after 我的收藏", got)
	}
	click(t, tt, "返回")
	if got := a.router.Path(); got != "/mine" {
		t.Fatalf("path %q after back", got)
	}
}

// The series page on a phone stacks the hero's details under its poster,
// and keeps a back button and the episodes within a finger's reach.
func TestCompactSeries(t *testing.T) {
	a, tt := compactApp(t)
	a.router.Push("/series/1")
	tt.Frame()
	tt.Advance(time.Second)
	for _, s := range []string{"浅浅归聿", "播放第 1 集", "选集", "共 130 集", "返回"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q in %q", s, tt.Texts())
		}
	}
}

// A phone's player is the theater: no way in or out of full screen, the
// episodes a drawer from the right that closes as an episode is chosen.
func TestCompactPlayer(t *testing.T) {
	a, tt := compactApp(t)
	a.cur = &playback{seriesID: "1", number: 1, gen: 1}
	a.fullscreen = true // a phone's screen is the theater
	a.router.Push("/play/1")
	tt.Frame()
	tt.Advance(time.Second)
	for _, s := range []string{"退出播放", "选集"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q in %q", s, tt.Texts())
		}
	}
	if tt.HasText("影院") || tt.HasText("退出影院") {
		t.Log("a narrow desktop window keeps the theater button; phones hide it")
	}
	click(t, tt, "选集")
	if !a.panelOpen {
		t.Fatal("the episodes do not open")
	}
	if !tt.HasText("第 1 集 / 共 130 集") {
		t.Fatalf("the drawer has no episodes: %q", tt.Texts())
	}
	click(t, tt, "收起选集")
	if a.panelOpen {
		t.Fatal("the episodes stay after the close button")
	}

	// Leaving the player goes back to the page the player opened from.
	click(t, tt, "退出播放")
	if got := a.router.Path(); got != "/discover" {
		t.Fatalf("path %q after leaving the player", got)
	}
}
