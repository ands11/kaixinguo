//go:build android || ios

package main

// onPhone is true in builds for phones and tablets: MyGo draws the app on
// the device's screen, the system owns the window, and there is no pointer
// to hover, window to drag or title bar to draw.
const onPhone = true
