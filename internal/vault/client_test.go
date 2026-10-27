package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	vauthlogin "github.com/131/vauth-login"

	"github.com/131/vauth-desktop/v2/internal/i18n"
)

const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl user@host"

type fakeVault struct {
	sync.Mutex
	values   map[string]any
	version  int64
	keys     []any
	policies []any
}

func (f *fakeVault) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.Lock()
	defer f.Unlock()
	if r.Header.Get("X-Vault-Token") != "user-token" {
		w.WriteHeader(403)
		return
	}
	body, _ := io.ReadAll(r.Body)
	reply := func(value any) { json.NewEncoder(w).Encode(value) }
	switch r.Method + " " + r.URL.EscapedPath() {
	case "GET /v1/auth/token/lookup-self":
		reply(map[string]any{"data": map[string]any{"ttl": 3600, "renewable": true, "creation_ttl": 3600, "entity_id": "e1", "policies": f.policies}})
	case "GET /v1/identity/entity/id/e1":
		reply(map[string]any{"data": map[string]any{"metadata": map[string]any{"VAUTH_USER_LOGIN": "jdoe", "VAUTH_USER_NAME": "John Doe", "VAUTH_USER_MAIL": "jdoe@example.com"}}})
	case "GET /v1/private/jdoe/data/.vauth_database":
		if f.version == 0 {
			w.WriteHeader(404)
			return
		}
		reply(map[string]any{"data": map[string]any{"data": f.values, "metadata": map[string]any{"version": f.version}}})
	case "POST /v1/private/jdoe/data/.vauth_database":
		var request struct {
			Data    map[string]any `json:"data"`
			Options struct {
				CAS int64 `json:"cas"`
			} `json:"options"`
		}
		json.Unmarshal(body, &request)
		if request.Options.CAS != f.version {
			w.WriteHeader(400)
			return
		}
		f.values, f.version = request.Data, f.version+1
		reply(map[string]any{"data": map[string]any{"version": f.version}})
	case "GET /v1/auth/ssh/role/jdoe":
		reply(map[string]any{"data": map[string]any{"public_keys": f.keys}})
	case "POST /v1/auth/ssh/role/jdoe":
		var request struct {
			PublicKeys []any `json:"public_keys"`
		}
		json.Unmarshal(body, &request)
		f.keys = request.PublicKeys
		w.WriteHeader(204)
	case "POST /v1/sys/capabilities-self":
		reply(map[string]any{"capabilities": []any{"read", "create", "update", "delete", "list"}})
	default:
		w.WriteHeader(404)
	}
}

func setup(t *testing.T) (*fakeVault, *Client) {
	vault := &fakeVault{}
	server := httptest.NewServer(vault)
	t.Cleanup(server.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("VAUTHRC", "")
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "user-token")
	cwd, _ := os.Getwd()
	os.Chdir(t.TempDir())
	t.Cleanup(func() { os.Chdir(cwd) })
	return vault, &Client{New: func() (*vauthlogin.Vauth, error) {
		v, err := vauthlogin.New()
		if err == nil {
			v.HTTPClient = server.Client()
		}
		return v, err
	}}
}

func TestConnectReadWrite(t *testing.T) {
	vault, client := setup(t)
	session, err := client.Connect(false)
	if err != nil || session.Login != "jdoe" || session.Profile["VAUTH_USER_NAME"] != "John Doe" {
		t.Fatalf("connect: %v %+v", err, session)
	}
	database, err := client.Read()
	if err != nil || database.Version != 0 || len(database.Values) != 0 {
		t.Fatalf("empty read: %v %+v", err, database)
	}
	if err := client.Write(map[string]any{"API_TOKEN": "x"}, 0); err != nil {
		t.Fatal(err)
	}
	if err := client.Write(map[string]any{"API_TOKEN": "y"}, 0); err == nil || !strings.Contains(err.Error(), "une autre session") {
		t.Fatalf("stale write accepted: %v", err)
	}
	if err := client.Write(map[string]any{"VAUTH_USER_LOGIN": "x"}, 1); err == nil {
		t.Fatal("identity key accepted")
	}
	database, _ = client.Read()
	if database.Version != 1 || database.Values["API_TOKEN"] != "x" || vault.values["API_TOKEN"] != "x" {
		t.Fatalf("read back: %+v", database)
	}
}

func TestRootRefused(t *testing.T) {
	vault, client := setup(t)
	vault.policies = []any{"root"}
	if _, err := client.Connect(false); err == nil {
		t.Fatal("root token accepted")
	}
}

func TestSSH(t *testing.T) {
	_, client := setup(t)
	if _, err := client.ReadSSH(); err == nil {
		t.Fatal("read before connect")
	}
	client.Connect(false)
	if err := client.PublishSSH("-----BEGIN OPENSSH PRIVATE KEY-----"); err == nil {
		t.Fatal("private key accepted")
	}
	if err := client.PublishSSH(" " + testKey + "\n"); err != nil {
		t.Fatal(err)
	}
	keys, err := client.ReadSSH()
	if err != nil || len(keys) != 1 || keys[0] != testKey {
		t.Fatalf("keys: %v %v", err, keys)
	}
}

func TestVariableName(t *testing.T) {
	if key, err := VariableName(" my_token "); err != nil || key != "MY_TOKEN" {
		t.Fatal(key, err)
	}
	for _, bad := range []string{"1A", "A-B", "vauth_user_mail", ""} {
		if _, err := VariableName(bad); err == nil {
			t.Fatal("accepted", bad)
		}
	}
}

func TestAgentError(t *testing.T) {
	cause := errors.New(`open \\.\pipe\openssh-ssh-agent: The system cannot find the file specified.`)
	raw := fmt.Errorf(`Could not reach ssh-agent at \\.\pipe\openssh-ssh-agent: %w`, cause)
	err := agentError(raw)
	if err.Error() != i18n.T("err.agent.unreachable") || !errors.Is(err, cause) {
		t.Fatalf("agent error = %q", err)
	}
	if other := errors.New("boom"); agentError(other) != other {
		t.Fatal("unrelated errors must pass through")
	}
}
