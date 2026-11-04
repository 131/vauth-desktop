package vault

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	vauthlogin "github.com/131/vauth-login"
)

func wrappingClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("VAUTHRC", "")
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "session-token")
	v, err := vauthlogin.New()
	if err != nil {
		t.Fatal(err)
	}
	v.HTTPClient = server.Client()
	return &Client{vauth: v, token: "session-token"}
}

func TestWrapAndUnwrap(t *testing.T) {
	text := "ad-hoc\ntext {\"value\":false} 😈"
	var stored map[string]any
	calls := 0
	client := wrappingClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/v1/sys/wrapping/wrap":
			if r.Method != "POST" || r.Header.Get("X-Vault-Token") != "session-token" || r.Header.Get("X-Vault-Wrap-TTL") != "300" {
				t.Error("wrong wrap request")
			}
			if err := json.NewDecoder(r.Body).Decode(&stored); err != nil {
				t.Error(err)
			}
			if len(stored) != 1 || stored["value"] != text {
				t.Error("payload types changed")
			}
			json.NewEncoder(w).Encode(map[string]any{"wrap_info": map[string]any{"token": "single-use", "ttl": 300, "creation_time": "2026-11-04T10:00:00Z", "creation_path": "sys/wrapping/wrap"}})
		case "/v1/sys/wrapping/unwrap":
			body, _ := io.ReadAll(r.Body)
			if r.Method != "POST" || r.Header.Get("X-Vault-Token") != "single-use" || len(body) != 0 || r.Header.Get("X-Vault-Wrap-TTL") != "" {
				t.Error("unwrap must use ONLY the wrapping token header")
			}
			if stored == nil {
				w.WriteHeader(400)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": stored})
			stored = nil
		default:
			t.Error("unexpected route", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	originalHTTP := client.vauth.HTTPClient
	token, err := client.Wrap(text)
	if err != nil || token != "single-use" {
		t.Fatalf("wrap: %+v %v", token, err)
	}
	if client.vauth.HTTPClient != originalHTTP {
		t.Fatal("SDK client mutated")
	}
	// Unwrap must not refresh an expired desktop session or retry the one-use token.
	client.sso = true
	client.expiresAt = time.Now().Add(-time.Hour)
	result, err := client.Unwrap("  single-use\n")
	if err != nil || result != text {
		t.Fatalf("unwrap: %+v %v", result, err)
	}
	if _, err = client.Unwrap("single-use"); err == nil {
		t.Fatal("reuse accepted")
	}
	if calls != 3 || client.token != "session-token" {
		t.Fatal("retry or changed session token", calls)
	}
}

func TestWrappingValidationAndFailures(t *testing.T) {
	client := &Client{}
	for _, text := range []string{"", " \n\t"} {
		if _, err := client.Wrap(text); err == nil {
			t.Fatal("blank message accepted")
		}
	}
	if _, err := client.Unwrap(" \n"); err == nil {
		t.Fatal("blank token accepted")
	}
	for _, status := range []int{400, 403, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			client := wrappingClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				io.WriteString(w, `{"errors":["sensitive-server-message"]}`)
			})
			if _, err := client.Unwrap("expired"); err == nil || strings.Contains(err.Error(), "sensitive-server-message") {
				t.Fatal("unsafe error", err)
			}
			if calls != 1 {
				t.Fatal("unwrap retried")
			}
			// A valid session receiving 403 must not trigger a privileged login.
			if status == 403 {
				return
			}
			if _, err := client.Wrap("message"); err == nil {
				t.Fatal("wrap status ignored")
			}
		})
	}
	for _, body := range []string{`not-json`, `{}`, `{"wrap_info":null}`, `{"wrap_info":{"token":"x","ttl":0}}`} {
		t.Run(body, func(t *testing.T) {
			client := wrappingClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
			if _, err := client.Wrap("message"); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
	for _, body := range []string{`not-json`, `null`, `[]`, `{}`, `{"data":{}}`, `{"data":{"value":42}}`, `{"data":{"value":null}}`} {
		t.Run("unwrap-"+body, func(t *testing.T) {
			client := wrappingClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
			if _, err := client.Unwrap("x"); err == nil {
				t.Fatal("invalid unwrap response accepted")
			}
		})
	}
}

type failingWrapTransport struct{ calls int }

func (t *failingWrapTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return nil, errors.New("network unavailable")
}
func TestUnwrapDoesNotRetryNetworkFailure(t *testing.T) {
	client := wrappingClient(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected network request") })
	transport := &failingWrapTransport{}
	client.vauth.HTTPClient.Transport = transport
	if _, err := client.Unwrap("one-use"); err == nil {
		t.Fatal("network error ignored")
	}
	if transport.calls != 1 {
		t.Fatal("retried ambiguous unwrap")
	}
}

func TestWrapDeniedWithValidSession(t *testing.T) {
	calls := 0
	client := wrappingClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/v1/auth/token/lookup-self" {
			if r.Header.Get("X-Vault-Wrap-TTL") != "" {
				t.Error("TTL leaked to session lookup")
			}
			io.WriteString(w, `{}`)
			return
		}
		w.WriteHeader(403)
	})
	if _, err := client.Wrap("message"); err == nil {
		t.Fatal("denied wrap accepted")
	}
	if calls != 2 {
		t.Fatal("valid session retried", calls)
	}
}

func TestWrapPreservesRedirectPolicy(t *testing.T) {
	client := wrappingClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sys/wrapping/wrap" {
			t.Error("redirect followed")
		}
		http.Redirect(w, r, "/forbidden", http.StatusTemporaryRedirect)
	})
	client.vauth.HTTPClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if _, err := client.Wrap("message"); err == nil {
		t.Fatal("redirect accepted")
	}
}
