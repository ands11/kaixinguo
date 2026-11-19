//go:build android

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// setPlatformPaths gives mygo the app's own dirs. The host sets HOME, the
// XDG dirs and its resources dir in the process's env before it starts
// Go, but on a release build Go's runtime has read its environ by then,
// and os.UserConfigDir finds none of it; the well known places of an
// Android app take over instead.
func setPlatformPaths() error {
	b, err := os.ReadFile("/proc/self/cmdline")
	if err != nil {
		return err
	}
	pkg := strings.TrimRight(string(b), "\x00")
	if pkg == "" {
		return errors.New("no process name in /proc/self/cmdline")
	}
	base := "/data/user/0"
	for _, u := range []string{"0", "10", "11", "99"} {
		if _, err := os.Stat("/data/user/" + u + "/" + pkg); err == nil {
			base = "/data/user/" + u
			break
		}
	}
	files := filepath.Join(base, pkg, "files")
	if _, err := os.Stat(files); err != nil {
		return err
	}
	cache := filepath.Join(base, pkg, "cache")
	_ = os.MkdirAll(cache, 0o755)
	data := filepath.Join(files, mygo.App.Name())
	_ = os.MkdirAll(data, 0o755)
	mygo.App.SetPath(mygo.PathUserData, data)
	mygo.App.SetPath(mygo.PathCache, cache)
	mygo.App.SetPath(mygo.PathTemp, filepath.Join(cache, "tmp"))
	if _, err := os.Stat(filepath.Join(files, "mygo-resources")); err == nil {
		mygo.App.SetPath(mygo.PathResources, filepath.Join(files, "mygo-resources"))
	}
	return nil
}
