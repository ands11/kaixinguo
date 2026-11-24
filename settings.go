package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// settingsForm is what the settings dialog edits, until it is saved.
type settingsForm struct {
	open     bool
	url      string
	token    string
	checking bool
	err      string // why the service did not answer as one should
	unsure   bool   // saving again saves it all the same
}

// openSettings shows the settings dialog with what is saved.
func (a *app) openSettings() {
	s := loadSignerSettings(a.dataDir)
	a.settings = settingsForm{open: true, url: s.URL, token: s.Token}
}

// settingsDialog is the settings: the signing service playback falls back
// to when the built-in signing stops working.
func (a *app) settingsDialog(c *ui.Context) {
	f := &a.settings
	if !f.open {
		return
	}
	t := c.Theme()
	ui.Modal(c, &f.open, func() {
		ui.Column(c).Width(440).Gap(14).Children(func() {
			ui.Text(c, "设置").FontSize(17).Bold()
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "签名服务").FontWeight(600)
				ui.Text(c, "红果改了签名校验时，内置签名可能暂时播不了。这时开心果会改用你部署的 openhongguo-signer 签名服务取播放地址，其他请求不经过它。不需要时留空。").
					FontSize(12).TextColor(t.TextMuted).Wrap()
			})
			urlErr := checkSignerURL(strings.TrimSpace(f.url))
			ui.Form(c, func() {
				ui.Field(c, "地址", func() {
					if ui.TextInput(c, &f.url).Placeholder("https://signer.example.com").Changed() {
						f.err, f.unsure = "", false
					}
				}).Error(urlErr)
				ui.Field(c, "Token", func() {
					if ui.TextInput(c, &f.token).Password().Placeholder("签名服务没有设 token 时留空").Changed() {
						f.err, f.unsure = "", false
					}
				})
			})
			if f.err != "" {
				ui.Text(c, f.err).FontSize(12).TextColor(t.Danger).Wrap()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					f.open = false
				}
				label := "保存"
				switch {
				case f.checking:
					label = "正在检查…"
				case f.unsure:
					label = "仍然保存"
				}
				if ui.PrimaryButton(c, label).Disabled(f.checking || urlErr != "").Clicked() {
					a.saveSettings()
				}
			})
		})
	})
}

// saveSettings checks that the service answers as one, then saves it and
// has playback fall back to it. A second save keeps one that did not.
func (a *app) saveSettings() {
	f := &a.settings
	s := signerSettings{URL: strings.TrimSpace(f.url), Token: strings.TrimSpace(f.token)}
	if s.URL == "" {
		s.Token = ""
	}
	if s.URL == "" || f.unsure {
		a.applySettings(s)
		return
	}
	f.checking, f.err = true, ""
	go func() {
		ctx, cancel := contextTimeout(20 * time.Second)
		defer cancel()
		err := probeSigner(ctx, s.URL, s.Token)
		a.update(func() {
			f.checking = false
			if !f.open {
				return
			}
			if err != nil {
				f.err, f.unsure = err.Error(), true
				return
			}
			a.applySettings(s)
		})
	}()
}

func (a *app) applySettings(s signerSettings) {
	f := &a.settings
	if err := saveSignerSettings(a.dataDir, s); err != nil {
		f.err = "保存失败：" + err.Error()
		return
	}
	if err := a.client.setSigner(s.URL, s.Token); err != nil {
		f.err = "签名服务的地址用不了：" + err.Error()
		return
	}
	f.open = false
}
