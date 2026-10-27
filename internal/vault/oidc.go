package vault

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/131/vauth-desktop/v2/internal/i18n"
)

const oidcCallback = "http://localhost:8250/oidc/callback"

type expiredError struct{}

func (*expiredError) Error() string { return i18n.T("err.session.expired") }

var ErrSessionExpired error = &expiredError{}

type oidcAttempt struct{ cancel context.CancelFunc }
type callbackResult struct {
	code string
	err  error
}
type contextTransport struct {
	ctx  context.Context
	base http.RoundTripper
}

func (t contextTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.base.RoundTrip(r.WithContext(t.ctx))
}

func (c *Client) cancelSSO() {
	if c.pending != nil {
		c.pending.cancel()
		c.pending = nil
	}
}
func (c *Client) CancelSSO() { c.mu.Lock(); defer c.mu.Unlock(); c.cancelSSO() }
func (c *Client) Close()     { c.mu.Lock(); defer c.mu.Unlock(); c.cancelSSO(); c.clear() }
func (c *Client) clear() {
	c.vauth = nil
	c.token = ""
	c.login = ""
	c.profile = nil
	c.mounts = nil
	c.mountTypes = nil
	c.sso = false
	c.renewable = false
	c.expiresAt = time.Time{}
	c.refreshAt = time.Time{}
}
func (c *Client) adopt(next *Client) {
	c.vauth = next.vauth
	c.token = next.token
	c.login = next.login
	c.profile = next.profile
	c.sso = next.sso
	c.renewable = next.renewable
	c.expiresAt = next.expiresAt
	c.refreshAt = next.refreshAt
	c.mounts = nil
	c.mountTypes = nil
}
func (c *Client) scheduleSSO(ttl int64, renewable bool) {
	c.refreshAt = time.Time{}
	c.expiresAt = time.Time{}
	c.renewable = renewable
	if ttl > 0 {
		c.expiresAt = c.now().Add(time.Duration(ttl) * time.Second)
		c.refreshAt = c.now().Add(time.Duration(ttl) * time.Second * 2 / 3)
	}
}
func (c *Client) renewSSO() error {
	status, body, err := c.vauth.Request("POST", "auth/token/renew-self", c.token, nil, 0)
	if err != nil {
		return i18n.Errorf("err.sso.network")
	}
	if status == 403 {
		c.clear()
		return ErrSessionExpired
	}
	if status < 200 || status >= 300 {
		return &VaultError{status}
	}
	var result struct {
		Auth struct {
			LeaseDuration int64 `json:"lease_duration"`
			Renewable     bool  `json:"renewable"`
		} `json:"auth"`
	}
	if json.Unmarshal(body, &result) != nil || result.Auth.LeaseDuration <= 0 {
		return i18n.Errorf("err.response")
	}
	c.scheduleSSO(result.Auth.LeaseDuration, result.Auth.Renewable)
	return nil
}

func (c *Client) CheckSession() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connected(); err != nil {
		return err
	}
	_, err := c.call("GET", "auth/token/lookup-self", nil, false)
	return err
}

// ExistingSession restores a page after a language reload without logging in.
func (c *Client) ExistingSession() (*Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.login == "" {
		return nil, nil
	}
	return c.loadProfile()
}

// LoginSSO exchanges the callback entirely in Go and never calls Connect or
// reads/writes the shared CLI cache. Only a validated candidate replaces state.
func (c *Client) LoginSSO(open func(string) error) (*Session, error) {
	timeout := 3 * time.Minute
	if c.ssoTimeout > 0 {
		timeout = c.ssoTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	attempt := &oidcAttempt{cancel: cancel}
	c.mu.Lock()
	if c.pending != nil {
		c.mu.Unlock()
		cancel()
		return nil, i18n.Errorf("err.sso.pending")
	}
	c.pending = attempt
	c.mu.Unlock()
	defer func() {
		cancel()
		c.mu.Lock()
		if c.pending == attempt {
			c.pending = nil
		}
		c.mu.Unlock()
	}()
	listen := c.listenOIDC
	if listen == nil {
		listen = func() (net.Listener, error) { return net.Listen("tcp4", "127.0.0.1:8250") }
	}
	listener, err := listen()
	if err != nil {
		return nil, i18n.Errorf("err.sso.port")
	}
	defer listener.Close()
	v, err := c.New()
	if err != nil {
		return nil, i18n.Errorf("err.sso.failed")
	}
	v.TokenCache = false
	v.Stderr = io.Discard
	original := v.HTTPClient
	base := http.DefaultTransport
	if original != nil && original.Transport != nil {
		base = original.Transport
	}
	v.HTTPClient = &http.Client{Transport: contextTransport{ctx, base}, Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(method, path string, payload any) (map[string]any, error) {
		status, body, err := v.Request(method, path, "", payload, 0)
		if err != nil {
			if ctx.Err() != nil {
				return nil, i18n.Errorf("err.sso.cancelled")
			}
			return nil, i18n.Errorf("err.sso.network")
		}
		if status < 200 || status >= 300 {
			return nil, &VaultError{status}
		}
		var result map[string]any
		if json.Unmarshal(body, &result) != nil {
			return nil, i18n.Errorf("err.response")
		}
		return result, nil
	}
	var random [32]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, i18n.Errorf("err.sso.failed")
	}
	nonce := hex.EncodeToString(random[:])
	challenge, err := request("POST", "auth/oidc/oidc/auth_url", map[string]any{"role": "oidc", "redirect_uri": oidcCallback, "client_nonce": nonce})
	if err != nil {
		return nil, err
	}
	data, _ := challenge["data"].(map[string]any)
	link, _ := data["auth_url"].(string)
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.Hostname() != "login.microsoftonline.com" || u.User != nil || (u.Port() != "" && u.Port() != "443") || u.Fragment != "" || u.Query().Get("redirect_uri") != oidcCallback || u.Query().Get("state") == "" {
		return nil, i18n.Errorf("err.sso.url")
	}
	state := u.Query().Get("state")
	result := make(chan callbackResult, 1)
	consumed := false
	var handlerMu sync.Mutex
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if r.Method != "GET" || r.URL.Path != "/oidc/callback" || r.Host != "localhost:8250" {
			http.NotFound(w, r)
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(query["state"]) != 1 || query.Get("state") != state {
			http.Error(w, i18n.T("sso.callback.invalid"), 400)
			return
		}
		handlerMu.Lock()
		defer handlerMu.Unlock()
		if consumed || ctx.Err() != nil {
			http.Error(w, i18n.T("sso.callback.invalid"), 410)
			return
		}
		if query.Get("error") != "" {
			consumed = true
			result <- callbackResult{err: i18n.Errorf("err.sso.failed")}
			io.WriteString(w, i18n.T("sso.callback.done"))
			return
		}
		if len(query["code"]) != 1 || query.Get("code") == "" {
			http.Error(w, i18n.T("sso.callback.invalid"), 400)
			return
		}
		consumed = true
		result <- callbackResult{code: query.Get("code")}
		io.WriteString(w, i18n.T("sso.callback.done"))
	})}
	defer server.Close()
	go func() { server.Serve(listener) }()
	if open == nil || open(link) != nil {
		return nil, i18n.Errorf("err.sso.browser")
	}
	var received callbackResult
	select {
	case received = <-result:
		if received.err != nil {
			return nil, received.err
		}
	case <-ctx.Done():
		return nil, i18n.Errorf("err.sso.cancelled")
	}
	params := url.Values{"code": {received.code}, "state": {state}, "client_nonce": {nonce}}
	exchanged, err := request("GET", "auth/oidc/oidc/callback?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	auth, _ := exchanged["auth"].(map[string]any)
	token, _ := auth["client_token"].(string)
	ttl, ttlOK := auth["lease_duration"].(float64)
	renewable, _ := auth["renewable"].(bool)
	if token == "" || !ttlOK || ttl < 0 {
		return nil, i18n.Errorf("err.auth")
	}
	v.HTTPClient = original
	next := &Client{New: c.New, Now: c.Now, vauth: v, token: token, sso: true}
	next.scheduleSSO(int64(ttl), renewable)
	session, err := next.loadProfile()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending != attempt || ctx.Err() != nil {
		return nil, i18n.Errorf("err.sso.cancelled")
	}
	c.adopt(next)
	return session, nil
}
