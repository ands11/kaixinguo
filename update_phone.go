//go:build android || ios

package main

import "github.com/egoist/mygo/ui"

// On a phone the app is installed and updated by its store, which the
// system handles: there is no release to download and no menu bar to check
// from, so the updater is left out of the build and these do nothing.

func useUpdater(a *app) {}

func (a *app) watchUpdates() {}

func (a *app) updateBanner(c *ui.Context) {}
