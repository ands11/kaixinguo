package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/beilunyang/openhongguo/adapters/httptransport"
	"github.com/beilunyang/openhongguo/adapters/memstore"
	"github.com/beilunyang/openhongguo/sdk"
)

func sdkClient(t *testing.T) *sdk.Client {
	t.Helper()
	c, err := sdk.NewClient(sdk.Options{Transport: httptransport.New(), ProfileStore: memstore.New()})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// fallbackClient is a client whose signing answers as local and remote say:
// nil for an answer.
type fallbackClient struct {
	*client
	local, remote error
	asked         []string
}

func newFallbackClient(t *testing.T) *fallbackClient {
	return &fallbackClient{client: withFallback(sdkClient(t), sdkClient(t))}
}

func (f *fallbackClient) play() error {
	_, err := call(f.client, func(s *sdk.Client) (string, error) {
		if s == f.client.remote {
			f.asked = append(f.asked, "remote")
			return "remote", f.remote
		}
		f.asked = append(f.asked, "local")
		return "local", f.local
	})
	return err
}

func (f *fallbackClient) took() []string {
	out := f.asked
	f.asked = nil
	return out
}

// While the built-in signing works, the signing service is not asked.
func TestBuiltinSigningFirst(t *testing.T) {
	f := newFallbackClient(t)
	if err := f.play(); err != nil {
		t.Fatal(err)
	}
	if got := f.took(); !slices.Equal(got, []string{"local"}) {
		t.Fatalf("asked %v", got)
	}
}

// Errors that are not the signing's are not retried with the service.
func TestOtherErrorsStay(t *testing.T) {
	f := newFallbackClient(t)
	f.local = sdk.ErrContentUnavailable
	if err := f.play(); !errors.Is(err, sdk.ErrContentUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if got := f.took(); !slices.Equal(got, []string{"local"}) {
		t.Fatalf("asked %v", got)
	}
}

// When the server stops taking the built-in signing, playback is signed by
// the service, and goes to the service first after that, until the
// service fails and the built-in signing works again.
func TestFallBackToSigner(t *testing.T) {
	f := newFallbackClient(t)
	f.local = sdk.ErrAnonymousLimited
	if err := f.play(); err != nil {
		t.Fatal(err)
	}
	if got := f.took(); !slices.Equal(got, []string{"local", "remote"}) {
		t.Fatalf("asked %v", got)
	}
	if err := f.play(); err != nil {
		t.Fatal(err)
	}
	if got := f.took(); !slices.Equal(got, []string{"remote"}) {
		t.Fatalf("then asked %v", got)
	}

	f.local, f.remote = nil, sdk.ErrSignerUnavailable
	if err := f.play(); err != nil {
		t.Fatal(err)
	}
	if got := f.took(); !slices.Equal(got, []string{"remote", "local"}) {
		t.Fatalf("with the service down, asked %v", got)
	}
	if f.remoteFirst() {
		t.Fatal("still signing with the service first")
	}
}

// With both failing, the error is the built-in signing's, not the service's.
func TestSignerDownKeepsBuiltinError(t *testing.T) {
	f := newFallbackClient(t)
	f.local, f.remote = sdk.ErrAnonymousLimited, sdk.ErrSignerUnavailable
	if err := f.play(); !errors.Is(err, sdk.ErrAnonymousLimited) {
		t.Fatalf("err = %v", err)
	}
	if f.remoteFirst() {
		t.Fatal("signing with the service first after it failed")
	}
}

// Without a service, nothing is retried.
func TestNoSigner(t *testing.T) {
	c := withFallback(sdkClient(t), nil)
	n := 0
	_, err := call(c, func(*sdk.Client) (int, error) { n++; return 0, sdk.ErrAnonymousLimited })
	if !errors.Is(err, sdk.ErrAnonymousLimited) || n != 1 {
		t.Fatalf("err = %v after %d tries", err, n)
	}
}

func TestSignerSettingsFile(t *testing.T) {
	dir := t.TempDir()
	s := signerSettings{URL: "https://signer.example.com", Token: "t"}
	if err := saveSignerSettings(dir, s); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "signer.json")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file %v, %v; want mode 0600", fi.Mode(), err)
	}
	if got := loadSignerSettings(dir); got != s {
		t.Fatalf("read %+v", got)
	}
	if err := saveSignerSettings(dir, signerSettings{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "signer.json")); !os.IsNotExist(err) {
		t.Fatal("cleared settings left their file")
	}
}

// The settings' check tells a service that is not ready, a wrong token and
// a right one apart.
func TestProbeSigner(t *testing.T) {
	ready := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			if ready {
				_, _ = w.Write([]byte(`{"service":"openhongguo-signer","ready":true}`))
			} else {
				_, _ = w.Write([]byte(`{"service":"openhongguo-signer","ready":false}`))
			}
		case "/feature-hash":
			if r.Header.Get("X-Signer-Token") != "right" {
				http.Error(w, `{"error":"unauthorized"}`, 401)
				return
			}
			http.Error(w, `{"error":"url is required"}`, 400)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	if err := probeSigner(ctx, srv.URL+"/", "right"); err != nil {
		t.Fatalf("right token: %v", err)
	}
	if err := probeSigner(ctx, srv.URL, "wrong"); err == nil || err.Error() != "token 不对" {
		t.Fatalf("wrong token: %v", err)
	}
	if err := probeSigner(ctx, srv.URL, ""); err == nil || err.Error() != "签名服务要求填写 token" {
		t.Fatalf("no token: %v", err)
	}
	ready = false
	if err := probeSigner(ctx, srv.URL, "right"); err == nil {
		t.Fatal("not ready: no error")
	}
	if err := probeSigner(ctx, srv.URL+"/nothing", "right"); err == nil {
		t.Fatal("not a signer: no error")
	}
}

func TestCheckSignerURL(t *testing.T) {
	for raw, ok := range map[string]bool{
		"": true, "https://pp.example.com": true, "http://127.0.0.1:19830": true,
		"pp.example.com": false, "ftp://x": false, "https://": false,
	} {
		if got := checkSignerURL(raw) == ""; got != ok {
			t.Errorf("checkSignerURL(%q) ok = %v", raw, got)
		}
	}
}
