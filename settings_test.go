package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func settingsApp(t *testing.T) (*app, *ui.Tester) {
	t.Helper()
	a := testApp(t)
	a.client = withFallback(sdkClient(t), nil)
	a.client.dir = t.TempDir()
	tt := ui.NewTester(a.view, 1180, 760)
	tt.Frame()
	tt.Advance(time.Second)
	a.openSettings()
	tt.Frame()
	tt.Advance(time.Second)
	return a, tt
}

// The dialog shows what is saved, and refuses an address that is not one.
func TestSettingsDialog(t *testing.T) {
	a, tt := settingsApp(t)
	for _, s := range []string{"设置", "签名服务", "地址", "Token", "保存"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q in the dialog: %q", s, tt.Texts())
		}
	}
	a.settings.url = "pp.example.com"
	tt.Frame()
	tt.Advance(time.Second)
	if !tt.HasText("请填写以 http:// 或 https:// 开头的地址") {
		t.Fatalf("no error for a bare host: %q", tt.Texts())
	}
	if err := tt.Click("保存"); err == nil && !a.settings.open {
		t.Fatal("saved an address that is not one")
	}
}

// A service that did not answer the check is saved by a second click, and
// playback then falls back to it; clearing the address turns it off.
func TestSettingsSave(t *testing.T) {
	a, tt := settingsApp(t)
	a.settings.url, a.settings.token = " https://signer.example.com ", "tok"
	a.settings.err, a.settings.unsure = "连不上签名服务", true
	tt.Frame()
	tt.Advance(time.Second)
	if !tt.HasText("连不上签名服务") || !tt.HasText("仍然保存") {
		t.Fatalf("the failed check does not show: %q", tt.Texts())
	}
	if err := tt.Click("仍然保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(time.Second)
	if a.settings.open {
		t.Fatalf("the dialog stays open: %q", a.settings.err)
	}
	if got := loadSignerSettings(a.dataDir); got != (signerSettings{URL: "https://signer.example.com", Token: "tok"}) {
		t.Fatalf("saved %+v", got)
	}
	if a.client.remote == nil {
		t.Fatal("playback does not fall back to the service")
	}

	a.openSettings()
	tt.Frame()
	tt.Advance(time.Second)
	if a.settings.url != "https://signer.example.com" || a.settings.token != "tok" {
		t.Fatalf("the dialog opens with %q, %q", a.settings.url, a.settings.token)
	}
	a.settings.url = ""
	tt.Frame()
	tt.Advance(time.Second)
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(time.Second)
	if a.settings.open || a.client.remote != nil || loadSignerSettings(a.dataDir) != (signerSettings{}) {
		t.Fatal("clearing the address did not turn the service off")
	}
}
