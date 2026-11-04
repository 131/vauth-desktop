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

// Wrap accepts text and returns its single-use token. The Vault JSON envelope
// and five-minute lifetime are implementation details, not public options.
func (c *Client) Wrap(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", i18n.Errorf("err.wrap.text")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	response, err := c.callWithTTL("POST", "sys/wrapping/wrap", map[string]string{"value": text}, false, 300)
	if err != nil {
		return "", err
	}
	info, ok := response["wrap_info"].(map[string]any)
	if !ok {
		return "", i18n.Errorf("err.response")
	}
	token, ok := info["token"].(string)
	ttl, validTTL := info["ttl"].(float64)
	if !ok || token == "" || !validTTL || ttl <= 0 {
		return "", i18n.Errorf("err.response")
	}
	return token, nil
}

// Unwrap uses the wrapping token ONLY as the client token. No login token,
// token body parameter, session renewal or retry may consume it a second time.
func (c *Client) Unwrap(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", i18n.Errorf("err.wrap.token")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.vauth == nil {
		if err := c.connect(false); err != nil {
			return "", err
		}
	}
	status, body, err := c.vauth.Request("POST", "sys/wrapping/unwrap", token, nil, 0)
	if err != nil {
		return "", err
	}
	if status == 400 || status == 403 {
		return "", i18n.Errorf("err.wrap.invalid")
	}
	if status < 200 || status >= 300 {
		return "", &VaultError{status}
	}
	var response map[string]any
	if json.Unmarshal(body, &response) != nil || response == nil {
		return "", i18n.Errorf("err.response")
	}
	data, ok := response["data"].(map[string]any)
	if !ok {
		return "", i18n.Errorf("err.wrap.message")
	}
	text, ok := data["value"].(string)
	if !ok {
		return "", i18n.Errorf("err.wrap.message")
	}
	return text, nil
}
