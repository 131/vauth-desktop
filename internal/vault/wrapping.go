package vault

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/131/vauth-desktop/v2/internal/i18n"
)

// wrapTransport adds the wrapping header without bypassing vauth-login's
// destination trust, HTTP client or redirect policy.
type wrapTransport struct {
	base http.RoundTripper
	ttl  string
}

func (t wrapTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Vault-Wrap-TTL", t.ttl)
	return t.base.RoundTrip(req)
}

func (c *Client) requestWithTTL(method, path string, payload any, ttl int) (int, []byte, error) {
	v := *c.vauth
	client := http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if v.HTTPClient != nil {
		client = *v.HTTPClient
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = wrapTransport{base, strconv.Itoa(ttl)}
	v.HTTPClient = &client
	return v.Request(method, path, c.token, payload, 0)
}

// WrapInfo is a single-use bearer token, not the desktop's session token.
type WrapInfo struct {
	Token        string `json:"token"`
	TTL          int    `json:"ttl"`
	CreationTime string `json:"creation_time"`
	CreationPath string `json:"creation_path"`
}

// Wrap stores an ad-hoc JSON object in Vault's response wrapping, never KV.
func (c *Client) Wrap(values map[string]any, ttl int) (*WrapInfo, error) {
	if values == nil {
		return nil, i18n.Errorf("err.wrap.object")
	}
	if ttl <= 0 {
		return nil, i18n.Errorf("err.wrap.ttl")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	response, err := c.callWithTTL("POST", "sys/wrapping/wrap", values, false, ttl)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(response["wrap_info"])
	if err != nil {
		return nil, i18n.Errorf("err.response")
	}
	var info WrapInfo
	if json.Unmarshal(encoded, &info) != nil || info.Token == "" || info.TTL <= 0 {
		return nil, i18n.Errorf("err.response")
	}
	return &info, nil
}

// Unwrap uses the wrapping token ONLY as the client token. No login token,
// token body parameter, session renewal or retry may consume it a second time.
func (c *Client) Unwrap(token string) (map[string]any, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, i18n.Errorf("err.wrap.token")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.vauth == nil {
		if err := c.connect(false); err != nil {
			return nil, err
		}
	}
	status, body, err := c.vauth.Request("POST", "sys/wrapping/unwrap", token, nil, 0)
	if err != nil {
		return nil, err
	}
	if status == 400 || status == 403 {
		return nil, i18n.Errorf("err.wrap.invalid")
	}
	if status < 200 || status >= 300 {
		return nil, &VaultError{status}
	}
	var response map[string]any
	if json.Unmarshal(body, &response) != nil || response == nil {
		return nil, i18n.Errorf("err.response")
	}
	return response, nil
}
