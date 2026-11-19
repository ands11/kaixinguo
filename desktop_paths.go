//go:build !android

package main

// setPlatformPaths is nothing on the desktop, where mygo finds the app's
// dirs in the environment on its own.
func setPlatformPaths() error { return nil }
