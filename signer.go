package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// remoteFor is how long playback keeps signing with the service after the
// built-in signing failed where the service succeeded, before trying the
// built-in signing again.
const remoteFor = 30 * time.Minute

// client is the SDK's client, which signs with the SDK's built-in signing.
// Playback, which the server's changes to signing broke each time, can fall
// back to an openhongguo-signer service the user set up: when the built-in
// signing gets nothing back and the service gets an answer, playback uses
// the service for a while. The two keep separate device profiles, so a
// device reset on one does not overwrite the other's.
type client struct {
	*sdk.Client
	dir  string
	opts sdk.Options

	mu     sync.Mutex
	remote *sdk.Client
	until  time.Time // remote first until then
}

func newClient(dir string) (*client, error) {
	opts := sdk.Options{RetryAttempts: 3, DeviceResets: sdk.Int(3)}
	if os.Getenv("KX_DEBUG") != "" {
		opts.Logger = log.New(os.Stderr, "sdk: ", log.Ltime|log.Lmicroseconds)
	}
	local, err := adapters.NewClient(filepath.Join(dir, "device.json"), opts)
	if err != nil {
		return nil, err
	}
	c := withFallback(local, nil)
	c.dir, c.opts = dir, opts
	return c, nil
}

// withFallback is a client that signs with local's built-in signing, and
// whose playback falls back to remote's signing service; remote may be nil.
func withFallback(local, remote *sdk.Client) *client {
	return &client{Client: local, remote: remote}
}

// setSigner has playback fall back to the signing service at rawURL, or to
// none when rawURL is empty.
func (c *client) setSigner(rawURL, token string) error {
	var remote *sdk.Client
	if rawURL != "" {
		opts := c.opts
		opts.SignerURL, opts.SignerToken = rawURL, token
		var err error
		if remote, err = adapters.NewClient(filepath.Join(c.dir, "device-signer.json"), opts); err != nil {
			return err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remote, c.until = remote, time.Time{}
	return nil
}

// signingFailed reports an error the built-in signing may be behind: when the
// server stops accepting it, requests come back empty or rejected.
func signingFailed(err error) bool {
	return errors.Is(err, sdk.ErrAnonymousLimited) || errors.Is(err, sdk.ErrSignatureRejected)
}

// order is the clients to ask, in turn: the one whose signing worked last
// first, and nil for no second.
func (c *client) order() (first, second *sdk.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.remote != nil && time.Now().Before(c.until) {
		return c.remote, c.Client
	}
	return c.Client, c.remote
}

func (c *client) remoteFirst() bool {
	first, _ := c.order()
	return first != c.Client
}

// worked notes that s's signing got the answer where the other's did not.
func (c *client) worked(s *sdk.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case s != c.remote:
		c.until = time.Time{}
	case s != nil:
		c.until = time.Now().Add(remoteFor)
	}
}

// call runs f with the client whose signing works now.
func call[T any](c *client, f func(*sdk.Client) (T, error)) (T, error) {
	first, second := c.order()
	v, err := f(first)
	if second == nil || !signingFailed(err) && !errors.Is(err, sdk.ErrSignerUnavailable) {
		return v, err
	}
	logSigner(err)
	sv, serr := f(second)
	if serr != nil && !errors.Is(serr, sdk.ErrDRMProtected) {
		logSigner(serr)
		if errors.Is(err, sdk.ErrSignerUnavailable) {
			return sv, serr // what the built-in signing got
		}
		return v, err
	}
	c.worked(second)
	return sv, serr
}

func logSigner(err error) {
	if errors.Is(err, sdk.ErrSignerUnavailable) && os.Getenv("KX_DEBUG") != "" {
		log.Printf("signer: %v", err)
	}
}

// ResolvePlayback reads an episode's playback model, with the signing that
// works now.
func (c *client) ResolvePlayback(ctx context.Context, episodeID string, opts sdk.PlaybackOptions) (*model.Playback, error) {
	return call(c, func(s *sdk.Client) (*model.Playback, error) { return s.ResolvePlayback(ctx, episodeID, opts) })
}

// signerSettings is the signing service playback falls back to, as the
// settings dialog keeps it. It lives in its own file, readable by the user
// alone, as it holds the service's token.
type signerSettings struct {
	URL   string `json:"url,omitempty"`
	Token string `json:"token,omitempty"`
}

func signerSettingsPath(dir string) string { return filepath.Join(dir, "signer.json") }

// loadSignerSettings reads the settings; missing or unreadable ones are none.
func loadSignerSettings(dir string) signerSettings {
	var s signerSettings
	if data, err := os.ReadFile(signerSettingsPath(dir)); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func saveSignerSettings(dir string, s signerSettings) error {
	path := signerSettingsPath(dir)
	if s == (signerSettings{}) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// checkSignerURL is what is wrong with a service's address, or "".
func checkSignerURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "请填写以 http:// 或 https:// 开头的地址"
	}
	return ""
}

var signerHTTP = &http.Client{Timeout: 15 * time.Second}

// probeSigner asks the service at rawURL whether it is ready to sign and
// takes token, and says what went wrong if not.
func probeSigner(ctx context.Context, rawURL, token string) error {
	base := strings.TrimRight(rawURL, "/")
	var ready struct {
		Ready bool `json:"ready"`
	}
	code, err := signerDo(ctx, "GET", base+"/ready", "", nil, &ready)
	switch {
	case err != nil:
		return errors.New("连不上签名服务")
	case code != 200:
		return fmt.Errorf("这个地址不是签名服务（HTTP %d）", code)
	case !ready.Ready:
		return errors.New("签名服务还在启动，稍后再试")
	}
	// Signing nothing: the service checks the token before the request,
	// which it then refuses for having no URL.
	code, err = signerDo(ctx, "POST", base+"/feature-hash", token, []byte("{}"), nil)
	switch {
	case err != nil:
		return errors.New("连不上签名服务")
	case code == 401:
		if token == "" {
			return errors.New("签名服务要求填写 token")
		}
		return errors.New("token 不对")
	case code != 400:
		return fmt.Errorf("签名服务返回 HTTP %d", code)
	}
	return nil
}

func signerDo(ctx context.Context, method, rawURL, token string, body []byte, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("X-Signer-Token", token)
	}
	resp, err := signerHTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(out)
	}
	return resp.StatusCode, nil
}
