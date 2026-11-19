package main

import "github.com/egoist/mygo/ui"

func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconHome     = icon(`<path d="M2.5 7.3 8 2.8l5.5 4.5v5.7a.8.8 0 0 1-.8.8H10v-3.6H6v3.6H3.3a.8.8 0 0 1-.8-.8z"/>`)
	iconChart    = icon(`<path d="M3.5 13.5v-5M8 13.5v-11M12.5 13.5v-7.5"/>`)
	iconCalendar = icon(`<rect x="2" y="3.2" width="12" height="10.8" rx="2.2"/><path d="M2 6.8h12M5.3 1.8v2.6M10.7 1.8v2.6"/>`)
	iconClock    = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 4.9V8l2.1 1.5"/>`)
	iconCheck    = icon(`<path d="m3.4 8.3 2.8 2.8 6.4-6.4" stroke-width="1.8"/>`)
	iconBack     = icon(`<path d="M9.8 3.2 5 8l4.8 4.8" stroke-width="1.7"/>`)
	iconPlay     = icon(`<path d="M5 3.1v9.8a.6.6 0 0 0 .92.5l7.7-4.9a.6.6 0 0 0 0-1L5.92 2.6A.6.6 0 0 0 5 3.1z" fill="currentColor" stroke="none"/>`)
	iconLock     = icon(`<rect x="3.5" y="7" width="9" height="7" rx="1.6"/><path d="M5.5 7V5.2a2.5 2.5 0 0 1 5 0V7"/>`)
	iconClose    = icon(`<path d="M4.1 4.1l7.8 7.8M11.9 4.1l-7.8 7.8" stroke-width="1.6"/>`)
	iconList     = icon(`<path d="M6.2 3.5h7.3M6.2 8h7.3M6.2 12.5h7.3"/><circle cx="3.1" cy="3.5" r=".7" fill="currentColor" stroke="none"/><circle cx="3.1" cy="8" r=".7" fill="currentColor" stroke="none"/><circle cx="3.1" cy="12.5" r=".7" fill="currentColor" stroke="none"/>`)
	iconTheater  = icon(`<rect x="1.9" y="3.1" width="12.2" height="9.8" rx="1.8"/><path d="M1.9 10.1h12.2" fill="currentColor"/>`)
	iconStar     = icon(`<path d="M8 2.1 9.53 5.9l4.08.28-3.14 2.62 1 3.97L8 10.6l-3.47 2.17 1-3.97L2.39 6.18l4.08-.28z"/>`)
	iconStarOn   = icon(`<path d="M8 2.1 9.53 5.9l4.08.28-3.14 2.62 1 3.97L8 10.6l-3.47 2.17 1-3.97L2.39 6.18l4.08-.28z" fill="currentColor"/>`)
	iconUpdate   = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 11V5.2M5.6 7.4 8 5l2.4 2.4"/>`)
	iconFlame    = icon(`<path d="M8 14c2.6 0 4.5-1.8 4.5-4.4 0-2.6-1.9-3.9-2.6-6.1-.3 1.4-1 2.2-1.7 2.6C8.2 4.3 7.3 2.8 5.8 2c.3 2.3-2.3 3.9-2.3 7.6C3.5 12.2 5.4 14 8 14z"/>`)
	iconSearch   = icon(`<circle cx="7.2" cy="7.2" r="4.4"/><path d="m10.6 10.6 3.1 3.1"/>`)
	iconPerson   = icon(`<circle cx="8" cy="5.3" r="2.7"/><path d="M2.9 13.7c0-2.5 2.3-4.2 5.1-4.2s5.1 1.7 5.1 4.2"/>`)
	iconChevron  = icon(`<path d="m6.2 3.6 4.4 4.4-4.4 4.4"/>`)
	iconPause    = icon(`<rect x="4.2" y="3.2" width="2.8" height="9.6" rx="0.8" fill="currentColor" stroke="none"/><rect x="9" y="3.2" width="2.8" height="9.6" rx="0.8" fill="currentColor" stroke="none"/>`)
	iconRew      = icon(`<path d="M7.6 4 3.2 8l4.4 4z" fill="currentColor" stroke="none"/><path d="M12.8 4 8.4 8l4.4 4z" fill="currentColor" stroke="none"/>`)
	iconFwd      = icon(`<path d="M3.2 4 7.6 8l-4.4 4z" fill="currentColor" stroke="none"/><path d="M8.4 4 12.8 8l-4.4 4z" fill="currentColor" stroke="none"/>`)
	iconSkipBack = icon(`<path d="M4.6 3.4v9.2"/><path d="M12.2 4.4 7 8l5.2 3.6z" fill="currentColor" stroke="none"/>`)
	iconSkipFwd  = icon(`<path d="M11.4 3.4v9.2"/><path d="M3.8 4.4 9 8l-5.2 3.6z" fill="currentColor" stroke="none"/>`)
	iconHeart    = icon(`<path d="M8 13.4C4.6 11.1 2.4 8.9 2.4 6.3c0-1.9 1.4-3.3 3.2-3.3 1 0 1.9.5 2.4 1.3.5-.8 1.4-1.3 2.4-1.3 1.8 0 3.2 1.4 3.2 3.3 0 2.6-2.2 4.8-5.6 7.1z"/>`)
	iconHeartOn  = icon(`<path d="M8 13.4C4.6 11.1 2.4 8.9 2.4 6.3c0-1.9 1.4-3.3 3.2-3.3 1 0 1.9.5 2.4 1.3.5-.8 1.4-1.3 2.4-1.3 1.8 0 3.2 1.4 3.2 3.3 0 2.6-2.2 4.8-5.6 7.1z" fill="currentColor" stroke="none"/>`)
	iconComment  = icon(`<path d="M13.6 8a5.6 5.6 0 0 1-8.1 5L3 13.7l.8-2.4A5.6 5.6 0 1 1 13.6 8z"/>`)
	iconShare    = icon(`<path d="M13.5 2.5 7 9M13.5 2.5 9.8 13.4 7 9 2.6 6.2z"/>`)
	iconMore     = icon(`<circle cx="8" cy="3.2" r="1.1" fill="currentColor" stroke="none"/><circle cx="8" cy="8" r="1.1" fill="currentColor" stroke="none"/><circle cx="8" cy="12.8" r="1.1" fill="currentColor" stroke="none"/>`)
	iconFullscr  = icon(`<path d="M2.2 6V2.2H6M10 2.2h3.8V6M13.8 10v3.8H10M6 13.8H2.2V10" stroke-width="1.6"/>`)
	iconFullscrS = icon(`<rect x="1.8" y="3.4" width="12.4" height="9.2" rx="1.6"/><path d="M9.6 9.2h3.4M13 7.6v1.6M6.4 6.8H3M3 8.4V6.8" stroke-width="1.2"/>`)
	iconChevUp   = icon(`<path d="m3.6 10.2 4.4-4.4 4.4 4.4" stroke-width="1.6"/>`)
	iconEye      = icon(`<path d="M1.9 8s2.3-4 6.1-4 6.1 4 6.1 4-2.3 4-6.1 4S1.9 8 1.9 8z"/><circle cx="8" cy="8" r="1.9"/>`)
	iconEyeOff   = icon(`<path d="M3.6 4.7C2.4 5.9 1.9 8 1.9 8s2.3 4 6.1 4c1 0 1.9-.3 2.7-.7M13.1 10.3c.6-.9 1-2.3 1-2.3s-2.3-4-6.1-4c-.5 0-1 .1-1.5.2"/><path d="m2.6 2.6 10.8 10.8" stroke-width="1.4"/>`)
	iconSend     = icon(`<path d="M13.8 2.2 2.4 7.4l4.2 1.9 1.9 4.2z" fill="currentColor" stroke="none"/><path d="M13.8 2.2 6.6 9.3" stroke-width="1.1"/>`)
)
