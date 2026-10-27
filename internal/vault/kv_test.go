package vault

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	vauthlogin "github.com/131/vauth-login"
)

func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("VAUTHRC", "")
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "user-token")
	return &Client{New: func() (*vauthlogin.Vauth, error) {
		v, err := vauthlogin.New()
		if err == nil {
			v.HTTPClient = server.Client()
		}
		return v, err
	}}
}

type kvFixture struct {
	mu       sync.Mutex
	requests []string
	values   map[string]any
	version  int64
	deleted  bool
}

func (f *kvFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	reply := func(value any) { json.NewEncoder(w).Encode(value) }
	if r.Header.Get("X-Vault-Token") != "user-token" {
		w.WriteHeader(403)
		return
	}
	route := strings.TrimPrefix(r.URL.Path, "/v1/")
	switch {
	case route == "auth/token/lookup-self":
		reply(map[string]any{"data": map[string]any{"ttl": 3600, "creation_ttl": 3600, "entity_id": "e1", "policies": []string{"default"}}})
	case route == "identity/entity/id/e1":
		reply(map[string]any{"data": map[string]any{"metadata": map[string]any{"VAUTH_USER_LOGIN": "jdoe", "VAUTH_USER_NAME": "John Doe", "CUSTOM_ID": "identity-value"}}})
	case route == "sys/internal/ui/mounts":
		reply(map[string]any{"data": map[string]any{"secret": map[string]any{
			"private/jdoe/":         map[string]any{"type": "kv", "options": map[string]any{"version": "2"}},
			"secrets/":              map[string]any{"type": "kv", "options": map[string]any{"version": "2"}},
			"legacy/":               map[string]any{"type": "kv"},
			"future/":               map[string]any{"type": "kv", "options": map[string]any{"version": "9"}},
			"transit/":              map[string]any{"type": "transit"},
			"secrets/data/blocked/": map[string]any{"type": "pki"},
		}}})
	case route == "sys/capabilities-self":
		var body struct {
			Paths []string `json:"paths"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		caps := []string{"read", "list", "create", "update", "delete"}
		if len(body.Paths) != 1 {
			w.WriteHeader(400)
			return
		}
		if body.Paths[0] == "secrets/data/denied" {
			caps = []string{"deny"}
		}
		if body.Paths[0] == "secrets/metadata/" {
			caps = []string{"read", "list"}
		}
		reply(map[string]any{body.Paths[0]: caps})
	case r.Method == "LIST" && (strings.HasPrefix(route, "secrets/metadata/") || route == "legacy/"):
		reply(map[string]any{"data": map[string]any{"keys": []string{"group/", "record"}}})
	case r.Method == "GET" && (route == "secrets/data/record" || route == "legacy/record"):
		if f.version == 0 {
			w.WriteHeader(404)
			return
		}
		if route == "legacy/record" {
			reply(map[string]any{"data": f.values})
			return
		}
		metadata := map[string]any{"version": f.version, "deletion_time": ""}
		var values any = f.values
		if f.deleted {
			metadata["deletion_time"] = "deleted"
			values = nil
		}
		reply(map[string]any{"data": map[string]any{"data": values, "metadata": metadata}})
	case r.Method == "POST" && (route == "secrets/data/record" || route == "legacy/record"):
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if route == "legacy/record" {
			f.values = body
			f.version++
			f.deleted = false
			w.WriteHeader(204)
			return
		}
		options, _ := body["options"].(map[string]any)
		cas, ok := options["cas"].(float64)
		if !ok || int64(cas) != f.version {
			w.WriteHeader(400)
			return
		}
		f.values, _ = body["data"].(map[string]any)
		f.version++
		f.deleted = false
		reply(map[string]any{"data": map[string]any{"version": f.version}})
	case r.Method == "DELETE" && (route == "secrets/data/record" || route == "legacy/record"):
		f.deleted = true
		if route == "legacy/record" {
			f.version = 0
		}
		w.WriteHeader(204)
	default:
		w.WriteHeader(404)
	}
}

func readyKV(t *testing.T) (*kvFixture, *Client) {
	t.Helper()
	f := &kvFixture{version: 1, values: map[string]any{"text": "secret", "nested": map[string]any{"flag": true}, "array": []any{nil, float64(2)}, "number": float64(3)}}
	c := testClient(t, f)
	if _, err := c.Connect(false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Mounts(); err != nil {
		t.Fatal(err)
	}
	return f, c
}

func TestKVDiscoveryAndTypes(t *testing.T) {
	_, c := readyKV(t)
	mounts, err := c.Mounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 4 {
		t.Fatalf("mounts %+v", mounts)
	}
	for _, mount := range mounts {
		if strings.Contains(mount.Path, "transit") || strings.Contains(mount.Path, "blocked") {
			t.Fatal("non-KV mount exposed")
		}
	}
	folder, err := c.ListKV("secrets/", "")
	if err != nil || len(folder.Keys) != 2 {
		t.Fatalf("folder %+v %v", folder, err)
	}
	doc, err := c.ReadKV("secrets/", "record")
	if err != nil || !doc.Exists || doc.Version != 1 {
		t.Fatalf("document %+v %v", doc, err)
	}
	if doc.Values["number"] != float64(3) || doc.Values["nested"].(map[string]any)["flag"] != true || doc.Values["array"].([]any)[0] != nil {
		t.Fatal("JSON types changed")
	}
}

func TestKVV2CRUDAndCAS(t *testing.T) {
	f, c := readyKV(t)
	values := map[string]any{"changed": true}
	if err := c.WriteKV("secrets/", "record", values, 0, true); err == nil {
		t.Fatal("CAS zero overwrote existing document")
	}
	if err := c.WriteKV("secrets/", "record", values, 9, false); err == nil {
		t.Fatal("stale version accepted")
	}
	if err := c.WriteKV("secrets/", "record", values, 1, false); err != nil {
		t.Fatal(err)
	}
	if f.version != 2 || !reflect.DeepEqual(f.values, values) {
		t.Fatal("write did not preserve object")
	}
	if err := c.DeleteKV("secrets/", "record"); err != nil {
		t.Fatal(err)
	}
	doc, err := c.ReadKV("secrets/", "record")
	if err != nil || doc.Exists || doc.Version != 2 {
		t.Fatalf("soft delete %+v %v", doc, err)
	}
	for _, request := range f.requests {
		if strings.Contains(request, "destroy/") || strings.Contains(request, "token/create") {
			t.Fatal("unexpected destructive/token request", request)
		}
	}
}

func TestKVV1CRUD(t *testing.T) {
	f, c := readyKV(t)
	doc, err := c.ReadKV("legacy/", "record")
	if err != nil || !doc.Exists || doc.Version != 0 {
		t.Fatalf("v1 %+v %v", doc, err)
	}
	values := map[string]any{"typed": []any{true, nil, float64(8)}}
	if err := c.WriteKV("legacy/", "record", values, 0, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.values, values) {
		t.Fatalf("v1 payload %v", f.values)
	}
	if err := c.DeleteKV("legacy/", "record"); err != nil {
		t.Fatal(err)
	}
	doc, err = c.ReadKV("legacy/", "record")
	if err != nil || doc.Exists {
		t.Fatalf("v1 delete %+v %v", doc, err)
	}
}

func TestKVBoundaries(t *testing.T) {
	f, c := readyKV(t)
	for _, path := range []string{"../x", "a/../b", "/v1/auth/token/create", "%2e%2e/x", "%252f", "a\\b", "a//b", "a?b", "a#b", "a\x00b", "blocked/record"} {
		before := len(f.requests)
		if _, err := c.ReadKV("secrets/", path); err == nil {
			t.Error("accepted path", path)
		}
		if err := c.WriteKV("secrets/", path, map[string]any{}, 1, false); err == nil {
			t.Error("write accepted path", path)
		}
		if err := c.DeleteKV("secrets/", path); err == nil {
			t.Error("delete accepted path", path)
		}
		if len(f.requests) != before {
			t.Error("invalid route reached Vault", path)
		}
	}
	for _, mount := range []string{"transit/", "unknown/", "future/"} {
		if _, err := c.ReadKV(mount, "record"); err == nil {
			t.Error("accepted mount", mount)
		}
	}
	if _, err := c.ReadKV("secrets/", "denied"); err == nil {
		t.Fatal("denied read accepted")
	}
	if err := c.WriteKV("secrets/", "denied", map[string]any{}, 1, false); err == nil {
		t.Fatal("denied write accepted")
	}
}

func TestEnvironmentRPCProtectsAllProfileNames(t *testing.T) {
	_, c := readyKV(t)
	for _, key := range []string{"CUSTOM_ID", "VAUTH_USER_NAME", "lowercase"} {
		if err := c.Write(map[string]any{key: "new"}, 1); err == nil {
			t.Error("identity/noncanonical name accepted", key)
		}
	}
	c.mu.Lock()
	c.clear()
	c.mu.Unlock()
	if _, err := c.ReadKV("secrets/", "record"); err == nil {
		t.Fatal("KV session data survived clear")
	}
	if errors.Is(c.CheckSession(), ErrSessionExpired) {
		t.Fatal("signed-out error should not silently sign in")
	}
}
