package vault

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	vauthlogin "github.com/131/vauth-login"
)

type oidcFixture struct {
	mu                 sync.Mutex
	base               kvFixture
	nonce              string
	root               bool
	badURL             bool
	unauthorizedHeader bool
	exchanges          int
	renews             int
	revoked            bool
}

func (f *oidcFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	reply := func(v any) { json.NewEncoder(w).Encode(v) }
	path := r.URL.Path
	if path == "/v1/auth/oidc/oidc/auth_url" {
		f.unauthorizedHeader = f.unauthorizedHeader || r.Header.Get("X-Vault-Token") != ""
		var data map[string]string
		json.NewDecoder(r.Body).Decode(&data)
		if data["redirect_uri"] != oidcCallback || data["role"] != "oidc" || len(data["client_nonce"]) != 64 {
			w.WriteHeader(400)
			return
		}
		f.nonce = data["client_nonce"]
		query := url.Values{"state": {"test-state"}, "redirect_uri": {oidcCallback}}
		host := "login.microsoftonline.com"
		if f.badURL {
			host = "attacker.invalid"
		}
		reply(map[string]any{"data": map[string]any{"auth_url": "https://" + host + "/common/oauth2/v2.0/authorize?" + query.Encode()}})
		return
	}
	if path == "/v1/auth/oidc/oidc/callback" {
		f.unauthorizedHeader = f.unauthorizedHeader || r.Header.Get("X-Vault-Token") != ""
		if r.URL.Query().Get("state") != "test-state" || r.URL.Query().Get("code") != "test-code" || r.URL.Query().Get("client_nonce") != f.nonce {
			w.WriteHeader(400)
			return
		}
		f.exchanges++
		reply(map[string]any{"auth": map[string]any{"client_token": "sso-token", "lease_duration": 60, "renewable": true}})
		return
	}
	if r.Header.Get("X-Vault-Token") == "sso-token" {
		if f.revoked {
			w.WriteHeader(403)
			return
		}
		if path == "/v1/auth/token/lookup-self" {
			policies := []string{"default"}
			if f.root {
				policies = []string{"root"}
			}
			reply(map[string]any{"data": map[string]any{"ttl": 60, "creation_ttl": 60, "entity_id": "e1", "policies": policies}})
			return
		}
		if path == "/v1/auth/token/renew-self" {
			f.renews++
			reply(map[string]any{"auth": map[string]any{"lease_duration": 60, "renewable": true}})
			return
		}
		if path == "/v1/auth/ssh/role/jdoe" {
			if r.Method == "POST" {
				var body struct {
					PublicKeys []string `json:"public_keys"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				if len(body.PublicKeys) != 1 || body.PublicKeys[0] != testKey {
					w.WriteHeader(400)
					return
				}
				w.WriteHeader(204)
			} else {
				reply(map[string]any{"data": map[string]any{"public_keys": []string{testKey}}})
			}
			return
		}
		clone := r.Clone(r.Context())
		clone.Header = r.Header.Clone()
		clone.Header.Set("X-Vault-Token", "user-token")
		f.base.ServeHTTP(w, clone)
		return
	}
	f.base.ServeHTTP(w, r)
}

func oidcClient(t *testing.T) (*oidcFixture, *Client, *string) {
	t.Helper()
	f := &oidcFixture{}
	c := testClient(t, f)
	address := ""
	c.listenOIDC = func() (net.Listener, error) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err == nil {
			address = listener.Addr().String()
		}
		return listener, err
	}
	c.ssoTimeout = 2 * time.Second
	return f, c, &address
}

func callback(t *testing.T, address, state string) int {
	t.Helper()
	query := url.Values{"state": {state}, "code": {"test-code"}}
	r, err := http.NewRequest("GET", "http://"+address+"/oidc/callback?"+query.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Host = "localhost:8250"
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal("local callback failed")
	}
	defer res.Body.Close()
	return res.StatusCode
}

func TestSSONoCacheAndFirstEnrollment(t *testing.T) {
	f, c, address := oidcClient(t)
	canary := filepath.Join(os.Getenv("HOME"), ".vauth", "tokens", "canary.creds")
	os.MkdirAll(filepath.Dir(canary), 0700)
	os.WriteFile(canary, []byte("existing-cache-canary"), 0600)
	session, err := c.LoginSSO(func(link string) error {
		u, _ := url.Parse(link)
		if u.Query().Get("state") != "test-state" {
			t.Error("missing challenge state")
		}
		if callback(t, *address, "wrong-state") != 400 {
			t.Error("mismatched state accepted")
		}
		if callback(t, *address, "test-state") != 200 {
			t.Error("valid callback denied")
		}
		if callback(t, *address, "test-state") != 410 {
			t.Error("callback replay accepted")
		}
		return nil
	})
	if err != nil || session.AuthMode != "sso" || session.Login != "jdoe" {
		t.Fatalf("SSO %+v %v", session, err)
	}
	if c.vauth.TokenCache || f.unauthorizedHeader || f.exchanges != 1 {
		t.Fatal("SSO touched cache/inherited token or exchanged twice")
	}
	files, _ := os.ReadDir(filepath.Dir(canary))
	bytes, _ := os.ReadFile(canary)
	if len(files) != 1 || string(bytes) != "existing-cache-canary" {
		t.Fatal("shared token cache changed")
	}
	if err := c.PublishSSH(testKey); err != nil {
		t.Fatal("first public-key enrollment failed", err)
	}
	if _, err := c.Read(); err != nil {
		t.Fatal("missing environment should be an empty document", err)
	}
	if len(f.base.requests) == 0 {
		t.Fatal("no profile/role operation used SSO session")
	}
	c.Close()
	if c.token != "" || c.vauth != nil {
		t.Fatal("session survived close")
	}
}

func TestSSOCancelTimeoutAndSingleFlight(t *testing.T) {
	_, c, _ := oidcClient(t)
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() { _, err := c.LoginSSO(func(string) error { close(started); return nil }); finished <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("SSO did not start")
	}
	if _, err := c.LoginSSO(func(string) error { return nil }); err == nil {
		t.Fatal("parallel SSO accepted")
	}
	c.CancelSSO()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not stop listener")
	}
	c.ssoTimeout = 40 * time.Millisecond
	if _, err := c.LoginSSO(func(string) error { return nil }); err == nil {
		t.Fatal("timeout succeeded")
	}
	if c.pending != nil || c.token != "" {
		t.Fatal("cancelled state survived")
	}
}

func TestSSORejectsUntrustedURLAndRoot(t *testing.T) {
	f, c, address := oidcClient(t)
	if _, err := c.Connect(false); err != nil {
		t.Fatal(err)
	}
	f.badURL = true
	opened := false
	if _, err := c.LoginSSO(func(string) error { opened = true; return nil }); err == nil || opened {
		t.Fatal("untrusted issuer opened")
	}
	f.badURL = false
	f.root = true
	if _, err := c.LoginSSO(func(string) error { callback(t, *address, "test-state"); return nil }); err == nil {
		t.Fatal("root SSO accepted")
	}
	if c.token != "user-token" || c.sso {
		t.Fatal("failed SSO replaced active session")
	}
}

func TestSSORenewalAndRevocationNeverUseSSHCache(t *testing.T) {
	f, c, address := oidcClient(t)
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	c.Now = func() time.Time { return now }
	if _, err := c.LoginSSO(func(string) error { callback(t, *address, "test-state"); return nil }); err != nil {
		t.Fatal(err)
	}
	c.New = func() (*vauthlogin.Vauth, error) {
		t.Error("SSO fell back to Connect/new SSH client")
		return nil, errors.New("must not be called")
	}
	now = now.Add(45 * time.Second)
	if err := c.CheckSession(); err != nil || f.renews != 1 {
		t.Fatalf("renewal %v count=%d", err, f.renews)
	}
	f.revoked = true
	if err := c.CheckSession(); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("revocation %v", err)
	}
	if c.token != "" || c.profile != nil {
		t.Fatal("revoked session values survived")
	}
}

func TestSSOPortErrorAndExpiry(t *testing.T) {
	_, c, address := oidcClient(t)
	listen := c.listenOIDC
	c.listenOIDC = func() (net.Listener, error) { return nil, errors.New("address occupied") }
	if _, err := c.LoginSSO(func(string) error { return nil }); err == nil || strings.Contains(err.Error(), "address occupied") {
		t.Fatal("raw listener error exposed")
	}
	c.listenOIDC = listen
	if _, err := c.LoginSSO(func(string) error { callback(t, *address, "test-state"); return nil }); err != nil {
		t.Fatal(err)
	}
	c.Now = func() time.Time { return c.expiresAt.Add(time.Second) }
	if err := c.CheckSession(); !errors.Is(err, ErrSessionExpired) {
		t.Fatal("expired SSO did not return to login", err)
	}
}
